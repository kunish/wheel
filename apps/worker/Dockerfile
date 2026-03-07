## Multi-stage Go build — produces ~15MB Alpine image
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder
ARG TARGETOS TARGETARCH
ARG VERSION=dev

RUN apk add --no-cache git make

WORKDIR /build
COPY apps/worker/go.mod apps/worker/go.sum ./
COPY apps/worker/third_party/CLIProxyAPIPlus/go.mod apps/worker/third_party/CLIProxyAPIPlus/go.sum ./third_party/CLIProxyAPIPlus/
RUN go mod download

COPY apps/worker/ .
ENV GOOS=$TARGETOS GOARCH=$TARGETARCH VERSION=$VERSION OUTPUT=/wheel
RUN make build

# ── Runtime ──
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata curl \
    && addgroup -g 1001 -S app \
    && adduser -S app -u 1001 -G app

WORKDIR /app
COPY --from=builder --chown=app:app /wheel .

USER app

ENV PORT=8787

EXPOSE 8787

ENTRYPOINT ["./wheel"]
