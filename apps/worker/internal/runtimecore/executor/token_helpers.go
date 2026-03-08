package executor

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tiktoken-go/tokenizer"
)

func tokenizerForModel(model string) (tokenizer.Codec, error) {
	sanitized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case sanitized == "":
		return tokenizer.Get(tokenizer.Cl100kBase)
	case strings.HasPrefix(sanitized, "gpt-5"):
		return tokenizer.ForModel(tokenizer.GPT5)
	case strings.HasPrefix(sanitized, "gpt-4.1"):
		return tokenizer.ForModel(tokenizer.GPT41)
	case strings.HasPrefix(sanitized, "gpt-4o"):
		return tokenizer.ForModel(tokenizer.GPT4o)
	case strings.HasPrefix(sanitized, "gpt-4"):
		return tokenizer.ForModel(tokenizer.GPT4)
	case strings.HasPrefix(sanitized, "gpt-3.5"), strings.HasPrefix(sanitized, "gpt-3"):
		return tokenizer.ForModel(tokenizer.GPT35Turbo)
	default:
		return tokenizer.Get(tokenizer.O200kBase)
	}
}

func countOpenAIChatTokens(enc tokenizer.Codec, payload []byte) (int64, error) {
	if enc == nil {
		return 0, fmt.Errorf("encoder is nil")
	}
	if len(payload) == 0 {
		return 0, nil
	}
	segments := make([]string, 0, 16)
	root := gjson.ParseBytes(payload)
	root.Get("messages").ForEach(func(_, message gjson.Result) bool {
		if role := strings.TrimSpace(message.Get("role").String()); role != "" {
			segments = append(segments, role)
		}
		content := message.Get("content")
		if content.Type == gjson.String {
			if text := strings.TrimSpace(content.String()); text != "" {
				segments = append(segments, text)
			}
		}
		return true
	})
	joined := strings.Join(segments, "\n")
	if strings.TrimSpace(joined) == "" {
		return 0, nil
	}
	count, err := enc.Count(joined)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func buildOpenAIUsageJSON(count int64) []byte {
	return []byte(fmt.Sprintf(`{"usage":{"prompt_tokens":%d,"completion_tokens":0,"total_tokens":%d}}`, count, count))
}
