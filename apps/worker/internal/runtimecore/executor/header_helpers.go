package executor

import (
	"net/http"
	"strings"
)

func applyCustomHeadersFromAttrs(r *http.Request, attrs map[string]string) {
	if r == nil || len(attrs) == 0 {
		return
	}
	for k, v := range attrs {
		if !strings.HasPrefix(k, "header:") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(k, "header:"))
		value := strings.TrimSpace(v)
		if name == "" || value == "" {
			continue
		}
		r.Header.Set(name, value)
	}
}
