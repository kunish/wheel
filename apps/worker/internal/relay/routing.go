package relay

// ── Routing Rule Types ──────────────────────────────────────────

// RoutingCondition defines a single match condition.
type RoutingCondition struct {
	Field    string `json:"field"`    // "model", "header:<name>", "apikey_name", "request_type", "body:<path>"
	Operator string `json:"operator"` // "eq", "neq", "contains", "prefix", "suffix", "regex", "in"
	Value    string `json:"value"`
}

// RoutingAction defines what happens when all conditions match.
type RoutingAction struct {
	Type       string `json:"type"`                 // "route", "reject", "rewrite"
	GroupName  string `json:"groupName,omitempty"`  // override group selection
	ModelName  string `json:"modelName,omitempty"`  // override target model
	StatusCode int    `json:"statusCode,omitempty"` // for "reject"
	Message    string `json:"message,omitempty"`    // for "reject"
}

// RoutingRule is a single conditional routing rule.
type RoutingRule struct {
	ID         int                `json:"id"`
	Name       string             `json:"name"`
	Priority   int                `json:"priority"` // lower = higher priority
	Enabled    bool               `json:"enabled"`
	Conditions []RoutingCondition `json:"conditions"`
	Action     RoutingAction      `json:"action"`
}

// RuleEvalContext holds the request data for rule evaluation.
type RuleEvalContext struct {
	Model       string
	Headers     map[string]string
	ApiKeyID    int
	ApiKeyName  string
	RequestType string
	Body        map[string]any
}

// RuleResult is the outcome of evaluating all rules.
type RuleResult struct {
	Matched  bool
	RuleName string
	Action   RoutingAction
}
