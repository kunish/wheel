package runtimeauth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func EnsureAuthIndex(fileName, apiKey, id string) string {
	seed := strings.TrimSpace(fileName)
	if seed != "" {
		seed = "file:" + seed
	} else if apiKey = strings.TrimSpace(apiKey); apiKey != "" {
		seed = "api_key:" + apiKey
	} else if id = strings.TrimSpace(id); id != "" {
		seed = "id:" + id
	}
	if seed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:8])
}
