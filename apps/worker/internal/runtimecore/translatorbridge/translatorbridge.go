package translatorbridge

import (
	"context"

	sdktranslator "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/translator"
	_ "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/translator/builtin"
	runtimetranslator "github.com/kunish/wheel/apps/worker/internal/runtimecore/translator"
)

type Adapter struct{}

var defaultAdapter = &Adapter{}

func Default() *Adapter {
	return defaultAdapter
}

func (a *Adapter) TranslateRequest(from, to runtimetranslator.Format, model string, rawJSON []byte, stream bool) []byte {
	return translateRequest(from.String(), to.String(), model, rawJSON, stream)
}

func (a *Adapter) TranslateStream(ctx context.Context, from, to runtimetranslator.Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	return translateStream(ctx, from.String(), to.String(), model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func (a *Adapter) TranslateNonStream(ctx context.Context, from, to runtimetranslator.Format, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return translateNonStream(ctx, from.String(), to.String(), model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func (a *Adapter) TranslateTokenCount(ctx context.Context, from, to runtimetranslator.Format, count int64, rawJSON []byte) string {
	return translateTokenCount(ctx, from.String(), to.String(), count, rawJSON)
}

func translateRequest(from, to, model string, rawJSON []byte, stream bool) []byte {
	return sdktranslator.TranslateRequest(sdktranslator.FromString(from), sdktranslator.FromString(to), model, rawJSON, stream)
}

func translateStream(ctx context.Context, from, to, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) []string {
	return sdktranslator.TranslateStream(ctx, sdktranslator.FromString(from), sdktranslator.FromString(to), model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func translateNonStream(ctx context.Context, from, to, model string, originalRequestRawJSON, requestRawJSON, rawJSON []byte, param *any) string {
	return sdktranslator.TranslateNonStream(ctx, sdktranslator.FromString(from), sdktranslator.FromString(to), model, originalRequestRawJSON, requestRawJSON, rawJSON, param)
}

func translateTokenCount(ctx context.Context, from, to string, count int64, rawJSON []byte) string {
	return sdktranslator.TranslateTokenCount(ctx, sdktranslator.FromString(from), sdktranslator.FromString(to), count, rawJSON)
}
