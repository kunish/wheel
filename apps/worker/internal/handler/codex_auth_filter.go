package handler

import (
	"fmt"
	"sort"
	"strings"
)

func parseAuthFiles(files []map[string]any) []codexAuthFile {
	out := make([]codexAuthFile, 0, len(files))
	for _, raw := range files {
		if len(raw) == 0 {
			continue
		}
		entry := codexAuthFile{
			Name:      stringFromMap(raw, "name"),
			Provider:  canonicalRuntimeProvider(stringFromMap(raw, "provider", "type")),
			Type:      strings.ToLower(stringFromMap(raw, "type")),
			Email:     stringFromMap(raw, "email"),
			Disabled:  boolFromMap(raw, "disabled"),
			AuthIndex: stringFromMap(raw, "auth_index", "authIndex"),
			Raw:       raw,
		}
		if entry.Provider == "" {
			entry.Provider = entry.Type
		}
		if entry.Name == "" {
			entry.Name = stringFromMap(raw, "id")
		}
		if entry.Name == "" {
			continue
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func filterAndPaginateAuthFiles(files []codexAuthFile, provider string, search string, disabled string, page int, pageSize int) ([]codexAuthFile, int) {
	filtered := filterCodexAuthFiles(files, provider, search, disabled)

	total := len(filtered)
	if total == 0 {
		return []codexAuthFile{}, 0
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []codexAuthFile{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

func filterCodexAuthFiles(files []codexAuthFile, provider string, search string, disabled string) []codexAuthFile {
	provider = canonicalRuntimeProvider(provider)
	search = strings.ToLower(strings.TrimSpace(search))
	disabled = strings.ToLower(strings.TrimSpace(disabled))

	filtered := make([]codexAuthFile, 0, len(files))
	for _, file := range files {
		p := canonicalRuntimeProvider(file.Provider)
		if p == "" {
			p = canonicalRuntimeProvider(file.Type)
		}
		if provider != "" && p != provider {
			continue
		}
		if search != "" {
			hit := strings.Contains(strings.ToLower(file.Name), search) || strings.Contains(strings.ToLower(file.Email), search)
			if !hit {
				continue
			}
		}
		if disabled == "true" && !file.Disabled {
			continue
		}
		if disabled == "false" && file.Disabled {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

// filterByQuotaStatus keeps only auth files whose quota matches the given status.
// files and quotaItems must be parallel slices (same length, same order).
func filterByQuotaStatus(files []codexAuthFile, quotaItems []codexQuotaItem, status string) ([]codexAuthFile, []codexQuotaItem) {
	matchingFiles := make([]codexAuthFile, 0, len(files))
	matchingQuota := make([]codexQuotaItem, 0, len(files))
	for i, item := range quotaItems {
		match := false
		switch status {
		case "error":
			match = item.Error != ""
		case "exhausted":
			if item.Weekly.LimitReached || item.CodeReview.LimitReached {
				match = true
			}
			for _, s := range item.Snapshots {
				if !s.Unlimited && s.PercentRemaining <= 0 {
					match = true
					break
				}
			}
		}
		if match {
			matchingFiles = append(matchingFiles, files[i])
			matchingQuota = append(matchingQuota, item)
		}
	}
	return matchingFiles, matchingQuota
}

func selectCodexAuthFilesForBatch(files []codexAuthFile, scope codexAuthBatchScope) ([]codexAuthFile, error) {
	if scope.AllMatching {
		filtered := filterCodexAuthFiles(files, scope.Provider, scope.Search, "")
		excluded := make(map[string]struct{}, len(scope.ExcludeNames))
		for _, name := range scope.ExcludeNames {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			excluded[name] = struct{}{}
		}
		selected := make([]codexAuthFile, 0, len(filtered))
		for _, file := range filtered {
			if _, ok := excluded[file.Name]; ok {
				continue
			}
			selected = append(selected, file)
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("no auth files matched selection")
		}
		return selected, nil
	}

	selectedNames := make(map[string]struct{}, len(scope.Names))
	for _, name := range scope.Names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		selectedNames[name] = struct{}{}
	}
	if len(selectedNames) == 0 {
		return nil, fmt.Errorf("names is required")
	}
	selected := make([]codexAuthFile, 0, len(selectedNames))
	for _, file := range files {
		if _, ok := selectedNames[file.Name]; ok {
			selected = append(selected, file)
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no auth files matched selection")
	}
	return selected, nil
}
