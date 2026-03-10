package translator

import "context"

// translateRequestByFormatName converts a request payload between schemas by their string identifiers.
func translateRequestByFormatName(from, to Format, model string, rawJSON []byte, stream bool) []byte {
	return translateRequestDefault(from, to, model, rawJSON, stream)
}

// hasResponseTransformerByFormatName reports whether a response translator exists between two schemas.
func hasResponseTransformerByFormatName(from, to Format) bool {
	return hasResponseTransformerDefault(from, to)
}

// translateStreamByFormatName converts streaming responses between schemas by their string identifiers.
func translateStreamByFormatName(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	return translateStreamDefault(ctx, from, to, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// translateNonStreamByFormatName converts non-streaming responses between schemas by their string identifiers.
func translateNonStreamByFormatName(ctx context.Context, from, to Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return translateNonStreamDefault(ctx, from, to, model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

// translateTokenCountByFormatName converts token counts between schemas by their string identifiers.
func translateTokenCountByFormatName(ctx context.Context, from, to Format, count int64, rawJSON []byte) string {
	return translateTokenCountDefault(ctx, from, to, count, rawJSON)
}
