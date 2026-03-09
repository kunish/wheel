package handler

import (
	"strconv"
	"strings"
)

func parsePositiveInt(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func valueByKeys(m map[string]any, keys ...string) any {
	if len(m) == 0 {
		return nil
	}
	for _, key := range keys {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return nil
}

func mapFromAny(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return map[string]any{}
	}
	return m
}

func sliceFromAny(v any) []map[string]any {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m := mapFromAny(item); len(m) > 0 {
			out = append(out, m)
		}
	}
	return out
}

func stringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if s := strings.TrimSpace(typed); s != "" {
				return s
			}
		}
	}
	return ""
}

func boolFromMap(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			v := strings.TrimSpace(strings.ToLower(typed))
			if v == "true" || v == "1" || v == "yes" {
				return true
			}
		}
	}
	return false
}

func int64FromMap(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return int64(typed)
		case float32:
			return int64(typed)
		case int:
			return int64(typed)
		case int32:
			return int64(typed)
		case int64:
			return typed
		case string:
			if v, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil {
				return v
			}
		}
	}
	return 0
}

func floatFromMap(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed
		case float32:
			return float64(typed)
		case int:
			return float64(typed)
		case int32:
			return float64(typed)
		case int64:
			return float64(typed)
		case string:
			if v, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return v
			}
		}
	}
	return 0
}
