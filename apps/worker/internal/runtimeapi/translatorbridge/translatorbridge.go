package translatorbridge

import (
	"context"

	runtimetranslator "github.com/kunish/wheel/apps/worker/internal/runtimecore/translator"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	_ "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator/builtin"
)

type Adapter struct{}

var defaultAdapter = &Adapter{}

func Default() *Adapter {
	return defaultAdapter
}

func (a *Adapter) TranslateRequest(from, to runtimetranslator.Format, model string, rawJSON []byte, stream bool) []byte {
	return TranslateRequest(from.String(), to.String(), model, rawJSON, stream)
}

func (a *Adapter) TranslateStream(ctx context.Context, from, to runtimetranslator.Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	return TranslateStream(ctx, from.String(), to.String(), model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func (a *Adapter) TranslateNonStream(ctx context.Context, from, to runtimetranslator.Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return TranslateNonStream(ctx, from.String(), to.String(), model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func (a *Adapter) TranslateTokenCount(ctx context.Context, from, to runtimetranslator.Format, count int64, rawJSON []byte) string {
	return TranslateTokenCount(ctx, from.String(), to.String(), count, rawJSON)
}

func TranslateRequest(from, to, model string, rawJSON []byte, stream bool) []byte {
	return sdktranslator.TranslateRequest(sdktranslator.FromString(from), sdktranslator.FromString(to), model, rawJSON, stream)
}

func TranslateStream(ctx context.Context, from, to, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	return sdktranslator.TranslateStream(ctx, sdktranslator.FromString(from), sdktranslator.FromString(to), model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func TranslateNonStream(ctx context.Context, from, to, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return sdktranslator.TranslateNonStream(ctx, sdktranslator.FromString(from), sdktranslator.FromString(to), model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func TranslateTokenCount(ctx context.Context, from, to string, count int64, rawJSON []byte) string {
	return sdktranslator.TranslateTokenCount(ctx, sdktranslator.FromString(from), sdktranslator.FromString(to), count, rawJSON)
}
