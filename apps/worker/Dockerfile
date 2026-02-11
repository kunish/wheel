## Multi-stage Go build — produces ~15MB Alpine image
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS TARGETARCH

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /wheel ./cmd/worker

# ── Runtime ──
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /wheel .
COPY drizzle/ ./drizzle/

RUN mkdir -p /app/data

ENV PORT=8787
ENV DB_PATH=/app/data/wheel.db

EXPOSE 8787

ENTRYPOINT ["./wheel"]
