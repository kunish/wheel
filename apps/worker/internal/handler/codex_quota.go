package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func (h *Handler) ListCodexQuota(c *gin.Context) {
	channel, err := h.validateCodexChannel(c)
	if err != nil {
		return
	}

	search := strings.TrimSpace(c.Query("search"))
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("pageSize"), 12)
	if pageSize > 50 {
		pageSize = 50
	}

	providerFilter := runtimeProviderFilter(channel.Type)

	var files []codexAuthFile
	if h.codexCapabilities().LocalEnabled {
		files, err = h.listManagedCodexAuthFiles(c.Request.Context(), channel.ID)
		if err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
	} else {
		if err := h.ensureCodexManagementConfigured(); err != nil {
			errorJSON(c, http.StatusBadRequest, err.Error())
			return
		}
		var resp struct {
			Files []map[string]any `json:"files"`
		}
		if err := h.codexManagementCall(c, http.MethodGet, "/auth-files", nil, nil, &resp); err != nil {
			errorJSON(c, http.StatusBadGateway, err.Error())
			return
		}
		files = parseAuthFiles(resp.Files)
	}

	paged, total := filterAndPaginateAuthFiles(files, providerFilter, search, "", page, pageSize)

	if channel.Type == types.OutboundCopilot {
		items := h.collectCopilotQuotaItems(c.Request.Context(), paged, codexQuotaFetchConcurrency, func(ctx context.Context, file codexAuthFile) ([]quotaSnapshot, string, string, error) {
			if h.codexCapabilities().LocalEnabled {
				return h.fetchLocalCopilotQuota(ctx, file.Raw)
			}
			return h.fetchCopilotQuota(ctx, file.AuthIndex)
		})
		h.storeQuotaCache(channel.ID, paged, items)
		successJSON(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
		return
	}

	items := h.collectCodexQuotaItems(c.Request.Context(), paged, codexQuotaFetchConcurrency, func(ctx context.Context, file codexAuthFile) (codexQuotaWindow, codexQuotaWindow, string, error) {
		accountID := stringFromMap(file.Raw, "account_id", "accountId")
		if accountID == "" {
			accountID = extractCodexAccountID(file.Raw)
		}
		if h.codexCapabilities().LocalEnabled {
			return h.fetchLocalCodexQuota(ctx, file.Raw)
		}
		return h.fetchCodexQuota(ctx, file.AuthIndex, accountID)
	})
	h.storeQuotaCache(channel.ID, paged, items)

	successJSON(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) collectCodexQuotaItems(ctx context.Context, files []codexAuthFile, concurrency int, fetch func(context.Context, codexAuthFile) (codexQuotaWindow, codexQuotaWindow, string, error)) []codexQuotaItem {
	items := make([]codexQuotaItem, len(files))
	if concurrency <= 1 {
		for i, file := range files {
			if ctx.Err() != nil {
				break
			}
			items[i] = buildCodexQuotaItem(ctx, file, fetch)
		}
		return items
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, file := range files {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		select {
		case sem <- struct{}{}:
			go func(i int, file codexAuthFile) {
				defer wg.Done()
				defer func() { <-sem }()
				items[i] = buildCodexQuotaItem(ctx, file, fetch)
			}(i, file)
		case <-ctx.Done():
			wg.Done()
		}
	}
	wg.Wait()
	return items
}

func (h *Handler) collectCopilotQuotaItems(ctx context.Context, files []codexAuthFile, concurrency int, fetch func(context.Context, codexAuthFile) ([]quotaSnapshot, string, string, error)) []codexQuotaItem {
	items := make([]codexQuotaItem, len(files))
	if concurrency <= 1 {
		for i, file := range files {
			if ctx.Err() != nil {
				break
			}
			items[i] = buildCopilotQuotaItem(ctx, file, fetch)
		}
		return items
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, file := range files {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		select {
		case sem <- struct{}{}:
			go func(i int, file codexAuthFile) {
				defer wg.Done()
				defer func() { <-sem }()
				items[i] = buildCopilotQuotaItem(ctx, file, fetch)
			}(i, file)
		case <-ctx.Done():
			wg.Done()
		}
	}
	wg.Wait()
	return items
}

func buildCopilotQuotaItem(ctx context.Context, file codexAuthFile, fetch func(context.Context, codexAuthFile) ([]quotaSnapshot, string, string, error)) codexQuotaItem {
	item := codexQuotaItem{
		Name:      file.Name,
		Email:     file.Email,
		AuthIndex: file.AuthIndex,
	}
	if file.AuthIndex == "" {
		item.Error = "missing auth_index"
		return item
	}
	snapshots, planType, resetAt, err := fetch(ctx, file)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	if len(snapshots) == 0 {
		item.Error = "copilot quota unavailable"
		return item
	}
	item.PlanType = planType
	item.ResetAt = resetAt
	item.Snapshots = snapshots
	return item
}

func buildCodexQuotaItem(ctx context.Context, file codexAuthFile, fetch func(context.Context, codexAuthFile) (codexQuotaWindow, codexQuotaWindow, string, error)) codexQuotaItem {
	item := codexQuotaItem{
		Name:      file.Name,
		Email:     file.Email,
		AuthIndex: file.AuthIndex,
	}
	if file.AuthIndex == "" {
		item.Error = "missing auth_index"
		return item
	}
	accountID := stringFromMap(file.Raw, "account_id", "accountId")
	if accountID == "" {
		accountID = extractCodexAccountID(file.Raw)
	}
	if accountID == "" {
		item.Error = "missing chatgpt account id"
		return item
	}
	weekly, codeReview, planType, err := fetch(ctx, file)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	item.PlanType = planType
	item.Weekly = weekly
	item.CodeReview = codeReview
	return item
}

func (h *Handler) fetchCodexQuota(ctx context.Context, authIndex string, accountID string) (codexQuotaWindow, codexQuotaWindow, string, error) {
	req := map[string]any{
		"authIndex": authIndex,
		"method":    "GET",
		"url":       codexQuotaEndpoint,
		"header": map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"Chatgpt-Account-Id": accountID,
			"Content-Type":       "application/json",
			"User-Agent":         "wheel/codex-quota",
		},
	}

	var out struct {
		StatusCode int    `json:"status_code"`
		Body       string `json:"body"`
	}
	if err := h.codexManagementCallContext(ctx, http.MethodPost, "/api-call", nil, req, &out); err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", err
	}
	if out.StatusCode < 200 || out.StatusCode >= 300 {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("quota request returned status %d", out.StatusCode)
	}
	return parseCodexQuotaBody(out.Body)
}

func (h *Handler) fetchLocalCodexQuota(ctx context.Context, raw map[string]any) (codexQuotaWindow, codexQuotaWindow, string, error) {
	accessToken := extractAccessToken(raw)
	if accessToken == "" {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("missing access token")
	}
	accountID := stringFromMap(raw, "account_id", "accountId")
	if accountID == "" {
		accountID = extractCodexAccountID(raw)
	}
	if accountID == "" {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("missing chatgpt account id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, codexQuotaEndpoint, nil)
	if err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("build quota request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Chatgpt-Account-Id", accountID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wheel/codex-quota")

	resp, err := h.doCodexQuotaRequest(req)
	if err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("request codex quota: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("read codex quota response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("quota request returned status %d", resp.StatusCode)
	}
	return parseCodexQuotaBody(string(body))
}

func (h *Handler) fetchCopilotQuota(ctx context.Context, authIndex string) ([]quotaSnapshot, string, string, error) {
	query := url.Values{}
	if strings.TrimSpace(authIndex) != "" {
		query.Set("auth_index", authIndex)
	}
	var out struct {
		AccessTypeSKU     string         `json:"access_type_sku"`
		CopilotPlan       string         `json:"copilot_plan"`
		QuotaResetDate    string         `json:"quota_reset_date"`
		QuotaSnapshots    map[string]any `json:"quota_snapshots"`
		MonthlyQuotas     map[string]any `json:"monthly_quotas"`
		LimitedUser       map[string]any `json:"limited_user_quotas"`
		LimitedReset      string         `json:"limited_user_reset_date"`
		QuotaResetDateUTC string         `json:"quota_reset_date_utc"`
	}
	if err := h.codexManagementCallContext(ctx, http.MethodGet, "/copilot-quota", query, nil, &out); err != nil {
		return nil, "", "", err
	}
	body, err := json.Marshal(out)
	if err != nil {
		return nil, "", "", fmt.Errorf("encode copilot quota response: %w", err)
	}
	return parseCopilotQuotaBody(string(body))
}

func (h *Handler) fetchLocalCopilotQuota(ctx context.Context, raw map[string]any) ([]quotaSnapshot, string, string, error) {
	accessToken := extractAccessToken(raw)
	if accessToken == "" {
		return nil, "", "", fmt.Errorf("missing access token")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/copilot_internal/user", nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("build copilot quota request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "wheel/copilot-quota")
	req.Header.Set("Accept", "application/json")

	resp, err := h.doCodexQuotaRequest(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("request copilot quota: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("read copilot quota response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("quota request returned status %d", resp.StatusCode)
	}
	return parseCopilotQuotaBody(string(body))
}

func (h *Handler) doCodexQuotaRequest(req *http.Request) (*http.Response, error) {
	if h != nil && h.codexQuotaDo != nil {
		return h.codexQuotaDo(req)
	}
	return (&http.Client{Timeout: 60 * time.Second}).Do(req)
}

func parseCodexQuotaBody(body string) (codexQuotaWindow, codexQuotaWindow, string, error) {
	bodyMap := map[string]any{}
	if err := json.Unmarshal([]byte(body), &bodyMap); err != nil {
		return codexQuotaWindow{}, codexQuotaWindow{}, "", fmt.Errorf("invalid quota response")
	}

	planType := stringFromMap(bodyMap, "plan_type", "planType")
	weeklyRate := mapFromAny(bodyMap["rate_limit"])
	if len(weeklyRate) == 0 {
		weeklyRate = mapFromAny(bodyMap["rateLimit"])
	}
	codeReviewRate := mapFromAny(bodyMap["code_review_rate_limit"])
	if len(codeReviewRate) == 0 {
		codeReviewRate = mapFromAny(bodyMap["codeReviewRateLimit"])
	}
	if len(codeReviewRate) == 0 {
		additional := sliceFromAny(bodyMap["additional_rate_limits"])
		if len(additional) == 0 {
			additional = sliceFromAny(bodyMap["additionalRateLimits"])
		}
		for _, item := range additional {
			rate := mapFromAny(valueByKeys(mapFromAny(item), "rate_limit", "rateLimit"))
			if len(rate) == 0 {
				continue
			}
			feature := strings.ToLower(stringFromMap(mapFromAny(item), "metered_feature", "meteredFeature", "limit_name", "limitName"))
			if strings.Contains(feature, "review") || strings.Contains(feature, "other") || len(codeReviewRate) == 0 {
				codeReviewRate = rate
				if strings.Contains(feature, "review") || strings.Contains(feature, "other") {
					break
				}
			}
		}
	}

	weekly := parseQuotaWindow(mapFromAny(valueByKeys(weeklyRate, "secondary_window", "secondaryWindow")))
	if weekly.LimitWindowSeconds == 0 {
		weekly = parseQuotaWindow(mapFromAny(valueByKeys(weeklyRate, "primary_window", "primaryWindow")))
	}
	weekly.Allowed = boolFromMap(weeklyRate, "allowed")
	weekly.LimitReached = boolFromMap(weeklyRate, "limit_reached", "limitReached")

	codeReview := parseQuotaWindow(mapFromAny(valueByKeys(codeReviewRate, "primary_window", "primaryWindow")))
	if codeReview.LimitWindowSeconds == 0 {
		codeReview = parseQuotaWindow(mapFromAny(valueByKeys(codeReviewRate, "secondary_window", "secondaryWindow")))
	}
	codeReview.Allowed = boolFromMap(codeReviewRate, "allowed")
	codeReview.LimitReached = boolFromMap(codeReviewRate, "limit_reached", "limitReached")

	return weekly, codeReview, planType, nil
}

func parseCopilotQuotaBody(body string) ([]quotaSnapshot, string, string, error) {
	bodyMap := map[string]any{}
	if err := json.Unmarshal([]byte(body), &bodyMap); err != nil {
		return nil, "", "", fmt.Errorf("invalid copilot quota response")
	}

	planType := stringFromMap(bodyMap, "copilot_plan", "access_type_sku", "accessTypeSKU")
	resetAt := stringFromMap(bodyMap, "quota_reset_date", "quotaResetDate", "quota_reset_date_utc", "quotaResetDateUtc", "limited_user_reset_date", "limitedUserResetDate")

	snapshotsMap := mapFromAny(bodyMap["quota_snapshots"])
	if len(snapshotsMap) > 0 {
		snapshots := collectCopilotSnapshotsFromQuotaSnapshots(snapshotsMap)
		if len(snapshots) == 0 {
			return nil, planType, resetAt, fmt.Errorf("copilot quota unavailable")
		}
		return snapshots, planType, resetAt, nil
	}

	monthlyQuotas := mapFromAny(bodyMap["monthly_quotas"])
	if len(monthlyQuotas) == 0 {
		monthlyQuotas = mapFromAny(bodyMap["monthlyQuotas"])
	}
	limitedQuotas := mapFromAny(bodyMap["limited_user_quotas"])
	if len(limitedQuotas) == 0 {
		limitedQuotas = mapFromAny(bodyMap["limitedUserQuotas"])
	}
	snapshots := collectCopilotSnapshotsFromMonthlyQuota(monthlyQuotas, limitedQuotas)
	if len(snapshots) == 0 {
		return nil, planType, resetAt, fmt.Errorf("copilot quota unavailable")
	}
	return snapshots, planType, resetAt, nil
}

func collectCopilotSnapshotsFromQuotaSnapshots(raw map[string]any) []quotaSnapshot {
	keys := []struct {
		id    string
		label string
	}{
		{id: "chat", label: "Chat"},
		{id: "completions", label: "Completions"},
		{id: "premium_interactions", label: "Premium Interactions"},
	}
	out := make([]quotaSnapshot, 0, len(keys))
	for _, key := range keys {
		detail := mapFromAny(raw[key.id])
		if len(detail) == 0 {
			continue
		}
		out = appendCopilotSnapshot(out, key.id, key.label, detail)
	}
	return out
}

func collectCopilotSnapshotsFromMonthlyQuota(monthlyQuotas map[string]any, limitedQuotas map[string]any) []quotaSnapshot {
	keys := []struct {
		id    string
		label string
	}{
		{id: "chat", label: "Chat"},
		{id: "completions", label: "Completions"},
	}
	out := make([]quotaSnapshot, 0, len(keys))
	for _, key := range keys {
		entitlement := floatFromMap(monthlyQuotas, key.id)
		if entitlement <= 0 {
			continue
		}
		remaining := entitlement
		if raw := valueByKeys(limitedQuotas, key.id); raw != nil {
			remaining = floatFromMap(map[string]any{"value": raw}, "value")
		}
		percentRemaining := 0.0
		if entitlement > 0 {
			percentRemaining = (remaining / entitlement) * 100
		}
		out = append(out, quotaSnapshot{
			ID:               key.id,
			Label:            key.label,
			PercentRemaining: clampPercent(percentRemaining),
			Remaining:        remaining,
			Entitlement:      entitlement,
		})
	}
	return out
}

func appendCopilotSnapshot(out []quotaSnapshot, id string, label string, detail map[string]any) []quotaSnapshot {
	if len(detail) == 0 {
		return out
	}
	percentRemaining := floatFromMap(detail, "percent_remaining", "percentRemaining")
	remaining := floatFromMap(detail, "quota_remaining", "quotaRemaining", "remaining")
	entitlement := floatFromMap(detail, "entitlement")
	unlimited := boolFromMap(detail, "unlimited")
	if percentRemaining == 0 && remaining == 0 && entitlement == 0 && !unlimited {
		return out
	}
	return append(out, quotaSnapshot{
		ID:               id,
		Label:            label,
		PercentRemaining: clampPercent(percentRemaining),
		Remaining:        remaining,
		Entitlement:      entitlement,
		Unlimited:        unlimited,
	})
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func extractCodexAccountID(auth map[string]any) string {
	idToken := mapFromAny(auth["id_token"])
	if len(idToken) == 0 {
		return ""
	}
	return stringFromMap(idToken, "chatgpt_account_id", "chatgptAccountId")
}

func extractAccessToken(auth map[string]any) string {
	if token := stringFromMap(auth, "access_token", "accessToken"); token != "" {
		return token
	}
	tokenMap := mapFromAny(auth["token"])
	return stringFromMap(tokenMap, "access_token", "accessToken")
}

func parseQuotaWindow(raw map[string]any) codexQuotaWindow {
	if len(raw) == 0 {
		return codexQuotaWindow{}
	}
	return codexQuotaWindow{
		UsedPercent:        floatFromMap(raw, "used_percent", "usedPercent"),
		LimitWindowSeconds: int64FromMap(raw, "limit_window_seconds", "limitWindowSeconds"),
		ResetAfterSeconds:  int64FromMap(raw, "reset_after_seconds", "resetAfterSeconds"),
		ResetAt:            stringFromMap(raw, "reset_at", "resetAt"),
	}
}

// ──── Quota Cache ────

// quotaCacheKey builds a cache key for a given channel and auth file name.
func quotaCacheKey(channelID int, name string) string {
	return strconv.Itoa(channelID) + ":" + name
}

// storeQuotaCache writes quota items for the given channel into the cache.
func (h *Handler) storeQuotaCache(channelID int, files []codexAuthFile, items []codexQuotaItem) {
	now := time.Now()
	for i, item := range items {
		if i >= len(files) {
			break
		}
		h.quotaCache.Store(quotaCacheKey(channelID, files[i].Name), quotaCacheEntry{
			Item:      item,
			FetchedAt: now,
		})
	}
}

// loadQuotaCache retrieves a cached quota item. Returns the item and true if
// the entry exists and has not expired; otherwise returns zero value and false.
func (h *Handler) loadQuotaCache(channelID int, name string) (codexQuotaItem, bool) {
	v, ok := h.quotaCache.Load(quotaCacheKey(channelID, name))
	if !ok {
		return codexQuotaItem{}, false
	}
	entry, ok := v.(quotaCacheEntry)
	if !ok || time.Since(entry.FetchedAt) > quotaCacheTTL {
		h.quotaCache.Delete(quotaCacheKey(channelID, name))
		return codexQuotaItem{}, false
	}
	return entry.Item, true
}
