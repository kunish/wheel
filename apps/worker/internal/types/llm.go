package types

// OpenAI model list response types.

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelListResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// Anthropic model list response types.

type AnthropicModel struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

type AnthropicModelListResponse struct {
	Data    []AnthropicModel `json:"data"`
	HasMore bool             `json:"has_more"`
}
