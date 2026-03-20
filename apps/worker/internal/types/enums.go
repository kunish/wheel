package types

// OutboundType represents the upstream provider type.
type OutboundType int

const (
	OutboundOpenAIChat      OutboundType = 0
	OutboundOpenAI          OutboundType = 1
	OutboundAnthropic       OutboundType = 2
	OutboundGemini          OutboundType = 3
	OutboundOpenAIResponses OutboundType = 4
	OutboundOpenAIEmbedding OutboundType = 5

	// ── Extended providers ──
	OutboundAzureOpenAI OutboundType = 10 // Azure OpenAI Service
	OutboundBedrock     OutboundType = 11 // AWS Bedrock (Anthropic/Meta models)
	OutboundVertex      OutboundType = 12 // Google Vertex AI
	OutboundCohere      OutboundType = 13 // Cohere (native API)

	// OpenAI-compatible providers (use OpenAI protocol with custom base URL)
	OutboundGroq        OutboundType = 20
	OutboundMistral     OutboundType = 21
	OutboundDeepSeek    OutboundType = 22
	OutboundXAI         OutboundType = 23 // xAI (Grok)
	OutboundCerebras    OutboundType = 24
	OutboundOpenRouter  OutboundType = 25
	OutboundPerplexity  OutboundType = 26
	OutboundTogether    OutboundType = 27
	OutboundOllama      OutboundType = 28
	OutboundVLLM        OutboundType = 29
	OutboundHuggingFace OutboundType = 30
	OutboundNovita      OutboundType = 31
	OutboundSiliconFlow OutboundType = 32
	OutboundCodex       OutboundType = 33 // Codex via embedded runtime
	OutboundCopilot     OutboundType = 34 // GitHub Copilot via embedded runtime
	OutboundCodexCLI    OutboundType = 35 // OpenAI Codex CLI (chatgpt.com Responses API)
	OutboundAntigravity OutboundType = 36 // Google Antigravity (Gemini internal API)
	OutboundCursor      OutboundType = 37 // Cursor IDE (api2.cursor.sh Agent API via ConnectRPC)
)

// outboundTypeName returns a human-readable name for the provider type.
func outboundTypeName(t OutboundType) string {
	switch t {
	case OutboundOpenAIChat, OutboundOpenAI:
		return "openai"
	case OutboundAnthropic:
		return "anthropic"
	case OutboundGemini:
		return "gemini"
	case OutboundOpenAIResponses:
		return "openai-responses"
	case OutboundOpenAIEmbedding:
		return "openai-embedding"
	case OutboundAzureOpenAI:
		return "azure-openai"
	case OutboundBedrock:
		return "bedrock"
	case OutboundVertex:
		return "vertex"
	case OutboundCohere:
		return "cohere"
	case OutboundGroq:
		return "groq"
	case OutboundMistral:
		return "mistral"
	case OutboundDeepSeek:
		return "deepseek"
	case OutboundXAI:
		return "xai"
	case OutboundCerebras:
		return "cerebras"
	case OutboundOpenRouter:
		return "openrouter"
	case OutboundPerplexity:
		return "perplexity"
	case OutboundTogether:
		return "together"
	case OutboundOllama:
		return "ollama"
	case OutboundVLLM:
		return "vllm"
	case OutboundHuggingFace:
		return "huggingface"
	case OutboundNovita:
		return "novita"
	case OutboundSiliconFlow:
		return "siliconflow"
	case OutboundCodex:
		return "codex"
	case OutboundCopilot:
		return "copilot"
	case OutboundCodexCLI:
		return "codex-cli"
	case OutboundAntigravity:
		return "antigravity"
	case OutboundCursor:
		return "cursor"
	default:
		return "unknown"
	}
}

// DefaultBaseURL returns the default API base URL for a provider type.
func DefaultBaseURL(t OutboundType) string {
	switch t {
	case OutboundOpenAIChat, OutboundOpenAI, OutboundOpenAIResponses, OutboundOpenAIEmbedding:
		return "https://api.openai.com"
	case OutboundAnthropic:
		return "https://api.anthropic.com"
	case OutboundGemini:
		return "https://generativelanguage.googleapis.com"
	case OutboundGroq:
		return "https://api.groq.com/openai"
	case OutboundMistral:
		return "https://api.mistral.ai"
	case OutboundDeepSeek:
		return "https://api.deepseek.com"
	case OutboundXAI:
		return "https://api.x.ai"
	case OutboundCerebras:
		return "https://api.cerebras.ai"
	case OutboundOpenRouter:
		return "https://openrouter.ai/api"
	case OutboundPerplexity:
		return "https://api.perplexity.ai"
	case OutboundTogether:
		return "https://api.together.xyz"
	case OutboundOllama:
		return "http://localhost:11434"
	case OutboundVLLM:
		return "http://localhost:8000"
	case OutboundHuggingFace:
		return "https://api-inference.huggingface.co"
	case OutboundCohere:
		return "https://api.cohere.com"
	case OutboundNovita:
		return "https://api.novita.ai"
	case OutboundSiliconFlow:
		return "https://api.siliconflow.cn"
	case OutboundCodex:
		return "http://codex-internal"
	case OutboundCopilot:
		return "http://codex-internal"
	case OutboundCodexCLI:
		return "https://chatgpt.com"
	case OutboundAntigravity:
		return "https://cloudcode-pa.googleapis.com"
	case OutboundCursor:
		return "https://api2.cursor.sh"
	default:
		return ""
	}
}

// GroupMode controls how channels are selected within a group.
type GroupMode int

const (
	GroupModeRoundRobin GroupMode = 1
	GroupModeRandom     GroupMode = 2
	GroupModeFailover   GroupMode = 3
	GroupModeWeighted   GroupMode = 4
	GroupModeAdaptive   GroupMode = 5
)

// AutoGroupType controls automatic group creation behavior.
type AutoGroupType int

const (
	AutoGroupNone  AutoGroupType = 0
	AutoGroupFuzzy AutoGroupType = 1
	AutoGroupExact AutoGroupType = 2
)

// AttemptStatus is the result status of a relay attempt.
type AttemptStatus string

const (
	AttemptStatusSuccess      AttemptStatus = "success"
	AttemptStatusFailed       AttemptStatus = "failed"
	attemptStatusCircuitBreak AttemptStatus = "circuit_break"
	attemptStatusSkipped      AttemptStatus = "skipped"
)
