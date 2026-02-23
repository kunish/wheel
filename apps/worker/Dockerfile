## Multi-stage Go build — produces ~15MB Alpine image
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS TARGETARCH

RUN apk add --no-cache git

WORKDIR /build
COPY apps/worker/go.mod apps/worker/go.sum ./
RUN go mod download

COPY apps/worker/ .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /wheel ./cmd/worker

# ── Runtime ──
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /wheel .

ENV PORT=8787

EXPOSE 8787

ENTRYPOINT ["./wheel"]
