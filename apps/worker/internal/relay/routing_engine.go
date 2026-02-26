package relay

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// RoutingEngine evaluates routing rules against incoming requests.
type RoutingEngine struct {
	mu            sync.RWMutex
	rules         []RoutingRule
	compiledRegex map[string]*regexp.Regexp // cond value -> compiled regex
}

// NewRoutingEngine creates a new RoutingEngine.
func NewRoutingEngine() *RoutingEngine {
	return &RoutingEngine{}
}

// SetRules replaces the rule set (sorted by priority ascending).
// Regex patterns are pre-compiled here to avoid per-request overhead.
func (e *RoutingEngine) SetRules(rules []RoutingRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sorted := make([]RoutingRule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	e.rules = sorted

	// Pre-compile all regex patterns
	compiled := make(map[string]*regexp.Regexp)
	for _, rule := range sorted {
		for _, c := range rule.Conditions {
			if c.Operator == "regex" {
				if _, exists := compiled[c.Value]; !exists {
					if re, err := regexp.Compile(c.Value); err == nil {
						compiled[c.Value] = re
					}
				}
			}
		}
	}
	e.compiledRegex = compiled
}

// LoadFromModels converts DB models to internal rules and replaces the rule set.
func (e *RoutingEngine) LoadFromModels(models []types.RoutingRuleModel) {
	rules := make([]RoutingRule, 0, len(models))
	for _, m := range models {
		conds := make([]RoutingCondition, 0, len(m.Conditions))
		for _, c := range m.Conditions {
			conds = append(conds, RoutingCondition{
				Field: c.Field, Operator: c.Operator, Value: c.Value,
			})
		}
		rules = append(rules, RoutingRule{
			ID:         m.ID,
			Name:       m.Name,
			Priority:   m.Priority,
			Enabled:    m.Enabled,
			Conditions: conds,
			Action: RoutingAction{
				Type:       m.Action.Type,
				GroupName:  m.Action.GroupName,
				ModelName:  m.Action.ModelName,
				StatusCode: m.Action.StatusCode,
				Message:    m.Action.Message,
			},
		})
	}
	e.SetRules(rules)
}

// Evaluate checks all enabled rules and returns the first match.
func (e *RoutingEngine) Evaluate(ctx *RuleEvalContext) *RuleResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}
		if matchAll(rule.Conditions, ctx, e.compiledRegex) {
			return &RuleResult{
				Matched:  true,
				RuleName: rule.Name,
				Action:   rule.Action,
			}
		}
	}
	return &RuleResult{Matched: false}
}

// ── Condition Matching ──────────────────────────────────────────

func matchAll(conds []RoutingCondition, ctx *RuleEvalContext, compiled map[string]*regexp.Regexp) bool {
	// A rule with no conditions should NOT match all requests
	if len(conds) == 0 {
		return false
	}
	for _, c := range conds {
		if !matchOne(c, ctx, compiled) {
			return false
		}
	}
	return true
}

func matchOne(cond RoutingCondition, ctx *RuleEvalContext, compiled map[string]*regexp.Regexp) bool {
	actual := resolveField(cond.Field, ctx)
	return evalOp(cond.Operator, actual, cond.Value, compiled)
}

func resolveField(field string, ctx *RuleEvalContext) string {
	switch {
	case field == "model":
		return ctx.Model
	case field == "request_type":
		return ctx.RequestType
	case field == "apikey_name":
		return ctx.ApiKeyName
	case strings.HasPrefix(field, "header:"):
		name := strings.TrimPrefix(field, "header:")
		return ctx.Headers[strings.ToLower(name)]
	case strings.HasPrefix(field, "body:"):
		path := strings.TrimPrefix(field, "body:")
		return resolveBodyPath(ctx.Body, path)
	default:
		return ""
	}
}

func resolveBodyPath(body map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var cur any = body
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	if cur != nil {
		b, _ := json.Marshal(cur)
		return string(b)
	}
	return ""
}

func evalOp(op, actual, expected string, compiled map[string]*regexp.Regexp) bool {
	switch op {
	case "eq":
		return actual == expected
	case "neq":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	case "prefix":
		return strings.HasPrefix(actual, expected)
	case "suffix":
		return strings.HasSuffix(actual, expected)
	case "regex":
		if re, ok := compiled[expected]; ok {
			return re.MatchString(actual)
		}
		// Fallback: pattern wasn't pre-compiled (invalid regex), return false
		return false
	case "in":
		for _, v := range strings.Split(expected, ",") {
			if strings.TrimSpace(v) == actual {
				return true
			}
		}
		return false
	default:
		return false
	}
}
