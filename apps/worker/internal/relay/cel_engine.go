package relay

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/cel-go/cel"
)

// CELEngine compiles and evaluates CEL expressions for routing rules.
type CELEngine struct {
	mu       sync.RWMutex
	env      *cel.Env
	programs map[string]cel.Program // expression string -> compiled program
}

// NewCELEngine creates a CEL engine with the standard variables available in routing.
func NewCELEngine() (*CELEngine, error) {
	env, err := cel.NewEnv(
		cel.Variable("model", cel.StringType),
		cel.Variable("request_type", cel.StringType),
		cel.Variable("apikey_name", cel.StringType),
		cel.Variable("apikey_id", cel.IntType),
		cel.Variable("is_stream", cel.BoolType),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("body", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL env: %w", err)
	}
	return &CELEngine{
		env:      env,
		programs: make(map[string]cel.Program),
	}, nil
}

// Compile pre-compiles a CEL expression and caches the program.
func (e *CELEngine) Compile(expression string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.programs[expression]; exists {
		return nil
	}

	ast, issues := e.env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("CEL compile error: %w", issues.Err())
	}

	prg, err := e.env.Program(ast)
	if err != nil {
		return fmt.Errorf("CEL program error: %w", err)
	}

	e.programs[expression] = prg
	return nil
}

// Evaluate runs a compiled CEL expression against the routing context.
// Returns true if the expression evaluates to true, false otherwise.
func (e *CELEngine) Evaluate(expression string, ctx *RuleEvalContext) bool {
	e.mu.RLock()
	prg, ok := e.programs[expression]
	e.mu.RUnlock()

	if !ok {
		// Try to compile on-the-fly
		if err := e.Compile(expression); err != nil {
			log.Printf("[cel] failed to compile expression %q: %v", expression, err)
			return false
		}
		e.mu.RLock()
		prg = e.programs[expression]
		e.mu.RUnlock()
	}

	// Build activation map from context
	activation := map[string]any{
		"model":        ctx.Model,
		"request_type": ctx.RequestType,
		"apikey_name":  ctx.ApiKeyName,
		"apikey_id":    int64(ctx.ApiKeyID),
		"is_stream":    ctx.IsStream,
		"headers":      ctx.Headers,
		"body":         ctx.Body,
	}

	out, _, err := prg.Eval(activation)
	if err != nil {
		log.Printf("[cel] evaluation error for %q: %v", expression, err)
		return false
	}

	result, ok := out.Value().(bool)
	return ok && result
}

// ClearCache clears the compiled program cache.
func (e *CELEngine) ClearCache() {
	e.mu.Lock()
	e.programs = make(map[string]cel.Program)
	e.mu.Unlock()
}
