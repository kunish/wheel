package relay

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

// GuardrailPlugin is a relay plugin that enforces guardrail rules from the DB.
// It supports keyword, regex, length, and PII-based rules with block/warn/redact actions.
type GuardrailPlugin struct {
	db    *bun.DB
	mu    sync.RWMutex
	rules []types.GuardrailRule

	compiledPatterns map[int]*regexp.Regexp
	piiPatterns      map[string]*regexp.Regexp
}

// NewGuardrailPlugin creates a guardrail plugin backed by DB rules.
func NewGuardrailPlugin(db *bun.DB) *GuardrailPlugin {
	p := &GuardrailPlugin{
		db:               db,
		compiledPatterns: make(map[int]*regexp.Regexp),
		piiPatterns:      defaultPIIPatterns(),
	}
	p.Reload(context.Background())
	return p
}

func (p *GuardrailPlugin) Name() string { return "guardrail" }

// Reload loads guardrail rules from the DB.
func (p *GuardrailPlugin) Reload(ctx context.Context) {
	var rules []types.GuardrailRule
	err := p.db.NewSelect().Model(&rules).Where("enabled = ?", true).Scan(ctx)
	if err != nil {
		log.Printf("[guardrail] failed to load rules: %v", err)
		return
	}

	compiled := make(map[int]*regexp.Regexp, len(rules))
	for _, r := range rules {
		if r.Type == "regex" && r.Pattern != "" {
			if re, err := regexp.Compile(r.Pattern); err == nil {
				compiled[r.ID] = re
			} else {
				log.Printf("[guardrail] bad regex for rule %d: %v", r.ID, err)
			}
		}
	}

	p.mu.Lock()
	p.rules = rules
	p.compiledPatterns = compiled
	p.mu.Unlock()

	log.Printf("[guardrail] loaded %d rules", len(rules))
}

func (p *GuardrailPlugin) PreHook(ctx *RelayContext) *ShortCircuit {
	p.mu.RLock()
	rules := p.rules
	compiled := p.compiledPatterns
	p.mu.RUnlock()

	if len(rules) == 0 {
		return nil
	}

	content := extractUserContent(ctx.Body)

	for _, rule := range rules {
		if rule.Target != "input" && rule.Target != "both" {
			continue
		}

		violation := p.checkRule(rule, content, compiled)
		if violation == "" {
			continue
		}

		switch rule.Action {
		case "block":
			return &ShortCircuit{
				StatusCode: 400,
				Body: OpenAIErrorBody(
					"guardrail_violation",
					fmt.Sprintf("Blocked by guardrail rule '%s': %s", rule.Name, violation),
				),
			}
		case "warn":
			log.Printf("[guardrail] WARN rule '%s': %s", rule.Name, violation)
		case "redact":
			p.redactContent(ctx.Body, rule, compiled)
		}
	}
	return nil
}

func (p *GuardrailPlugin) PostHook(ctx *RelayContext, resp *RelayPluginResponse) {
	if resp == nil || !resp.Success {
		return
	}

	p.mu.RLock()
	rules := p.rules
	compiled := p.compiledPatterns
	p.mu.RUnlock()

	if len(rules) == 0 {
		return
	}

	var outputContent string
	if resp.Body != nil {
		outputContent = extractAssistantContent(resp.Body)
	}
	if resp.IsStream && resp.StreamContent != "" {
		outputContent = resp.StreamContent
	}
	if outputContent == "" {
		return
	}

	for _, rule := range rules {
		if rule.Target != "output" && rule.Target != "both" {
			continue
		}
		violation := p.checkRule(rule, outputContent, compiled)
		if violation == "" {
			continue
		}
		switch rule.Action {
		case "block":
			resp.Success = false
			resp.Error = fmt.Errorf("blocked by guardrail rule '%s': %s", rule.Name, violation)
			resp.Body = OpenAIErrorBody("guardrail_violation",
				fmt.Sprintf("Response blocked by guardrail rule '%s'", rule.Name))
			resp.StatusCode = 400
			return
		case "warn":
			log.Printf("[guardrail] WARN output rule '%s': %s", rule.Name, violation)
		case "redact":
			if resp.Body != nil {
				redactAssistantContent(resp.Body, rule, compiled, p.piiPatterns)
			}
		}
	}
}

func (p *GuardrailPlugin) checkRule(rule types.GuardrailRule, content string, compiled map[int]*regexp.Regexp) string {
	switch rule.Type {
	case "keyword":
		if rule.Pattern != "" && strings.Contains(strings.ToLower(content), strings.ToLower(rule.Pattern)) {
			return fmt.Sprintf("contains blocked keyword '%s'", rule.Pattern)
		}
	case "regex":
		if re, ok := compiled[rule.ID]; ok && re.MatchString(content) {
			return fmt.Sprintf("matches blocked pattern '%s'", rule.Pattern)
		}
	case "length":
		if rule.MaxLength > 0 && len(content) > rule.MaxLength {
			return fmt.Sprintf("exceeds max length %d (got %d)", rule.MaxLength, len(content))
		}
	case "pii":
		for name, re := range p.piiPatterns {
			if re.MatchString(content) {
				return fmt.Sprintf("contains PII (%s)", name)
			}
		}
	}
	return ""
}

func (p *GuardrailPlugin) redactContent(body map[string]any, rule types.GuardrailRule, compiled map[int]*regexp.Regexp) {
	messages, ok := body["messages"].([]any)
	if !ok {
		return
	}
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "user" && role != "system" {
			continue
		}
		if content, ok := msg["content"].(string); ok {
			msg["content"] = p.applyRedaction(content, rule, compiled)
		}
	}
}

func (p *GuardrailPlugin) applyRedaction(content string, rule types.GuardrailRule, compiled map[int]*regexp.Regexp) string {
	switch rule.Type {
	case "keyword":
		return strings.ReplaceAll(content, rule.Pattern, "[REDACTED]")
	case "regex":
		if re, ok := compiled[rule.ID]; ok {
			return re.ReplaceAllString(content, "[REDACTED]")
		}
	case "pii":
		for _, re := range p.piiPatterns {
			content = re.ReplaceAllString(content, "[REDACTED]")
		}
	}
	return content
}

func extractAssistantContent(body map[string]any) string {
	choices, ok := body["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return ""
	}
	msg, ok := choice["message"].(map[string]any)
	if !ok {
		return ""
	}
	content, _ := msg["content"].(string)
	return content
}

func redactAssistantContent(body map[string]any, rule types.GuardrailRule, compiled map[int]*regexp.Regexp, piiPatterns map[string]*regexp.Regexp) {
	choices, ok := body["choices"].([]any)
	if !ok || len(choices) == 0 {
		return
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return
	}
	msg, ok := choice["message"].(map[string]any)
	if !ok {
		return
	}
	content, ok := msg["content"].(string)
	if !ok {
		return
	}

	switch rule.Type {
	case "keyword":
		msg["content"] = strings.ReplaceAll(content, rule.Pattern, "[REDACTED]")
	case "regex":
		if re, ok := compiled[rule.ID]; ok {
			msg["content"] = re.ReplaceAllString(content, "[REDACTED]")
		}
	case "pii":
		for _, re := range piiPatterns {
			content = re.ReplaceAllString(content, "[REDACTED]")
		}
		msg["content"] = content
	}
}

func defaultPIIPatterns() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"email":       regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		"phone":       regexp.MustCompile(`(\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`),
		"ssn":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		"credit_card": regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
		"ip_address":  regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
	}
}
