package translator

import "context"

// requestEnvelope represents a request in the translation pipeline.
type requestEnvelope struct {
	Format Format
	Model  string
	Stream bool
	Body   []byte
}

// responseEnvelope represents a response in the translation pipeline.
type responseEnvelope struct {
	Format Format
	Model  string
	Stream bool
	Body   []byte
	Chunks []string
}

// requestMiddleware decorates request translation.
type requestMiddleware func(ctx context.Context, req requestEnvelope, next requestHandler) (requestEnvelope, error)

// responseMiddleware decorates response translation.
type responseMiddleware func(ctx context.Context, resp responseEnvelope, next responseHandler) (responseEnvelope, error)

// requestHandler performs request translation between formats.
type requestHandler func(ctx context.Context, req requestEnvelope) (requestEnvelope, error)

// responseHandler performs response translation between formats.
type responseHandler func(ctx context.Context, resp responseEnvelope) (responseEnvelope, error)

// pipeline orchestrates request/response transformation with middleware support.
type pipeline struct {
	registry           *registry
	requestMiddleware  []requestMiddleware
	responseMiddleware []responseMiddleware
}

// newPipeline constructs a pipeline bound to the provided registry.
func newPipeline(reg *registry) *pipeline {
	if reg == nil {
		reg = defaultReg()
	}
	return &pipeline{registry: reg}
}

// useRequest adds request middleware executed in registration order.
func (p *pipeline) useRequest(mw requestMiddleware) {
	if mw != nil {
		p.requestMiddleware = append(p.requestMiddleware, mw)
	}
}

// useResponse adds response middleware executed in registration order.
func (p *pipeline) useResponse(mw responseMiddleware) {
	if mw != nil {
		p.responseMiddleware = append(p.responseMiddleware, mw)
	}
}

// translateRequest applies middleware and registry transformations.
func (p *pipeline) translateRequest(ctx context.Context, from, to Format, req requestEnvelope) (requestEnvelope, error) {
	terminal := func(ctx context.Context, input requestEnvelope) (requestEnvelope, error) {
		translated := p.registry.translateRequest(from, to, input.Model, input.Body, input.Stream)
		input.Body = translated
		input.Format = to
		return input, nil
	}

	handler := terminal
	for i := len(p.requestMiddleware) - 1; i >= 0; i-- {
		mw := p.requestMiddleware[i]
		next := handler
		handler = func(ctx context.Context, r requestEnvelope) (requestEnvelope, error) {
			return mw(ctx, r, next)
		}
	}

	return handler(ctx, req)
}

// translateResponse applies middleware and registry transformations.
func (p *pipeline) translateResponse(ctx context.Context, from, to Format, resp responseEnvelope, originalReq, translatedReq []byte, param *any) (responseEnvelope, error) {
	terminal := func(ctx context.Context, input responseEnvelope) (responseEnvelope, error) {
		if input.Stream {
			input.Chunks = p.registry.translateStream(ctx, from, to, input.Model, originalReq, translatedReq, input.Body, param)
		} else {
			input.Body = []byte(p.registry.translateNonStream(ctx, from, to, input.Model, originalReq, translatedReq, input.Body, param))
		}
		input.Format = to
		return input, nil
	}

	handler := terminal
	for i := len(p.responseMiddleware) - 1; i >= 0; i-- {
		mw := p.responseMiddleware[i]
		next := handler
		handler = func(ctx context.Context, r responseEnvelope) (responseEnvelope, error) {
			return mw(ctx, r, next)
		}
	}

	return handler(ctx, resp)
}
