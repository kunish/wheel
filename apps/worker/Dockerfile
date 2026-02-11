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
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /wheel .
COPY apps/worker/drizzle/ ./drizzle/

RUN mkdir -p /app/data

ENV PORT=8787
ENV DATA_PATH=/app/data

EXPOSE 8787

ENTRYPOINT ["./wheel"]
