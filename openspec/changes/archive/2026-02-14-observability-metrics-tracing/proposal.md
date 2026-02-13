## Why

Wheel 作为 LLM API Gateway，已有 WebSocket 实时仪表盘和请求日志，但缺乏与外部监控栈（Prometheus/Grafana、Jaeger/Tempo）的集成能力。运维团队无法将 Wheel 纳入现有的告警、SLO 和分布式追踪体系。本次变更为 Worker 添加可选的 Prometheus 指标导出和 OpenTelemetry 分布式追踪。

## What Changes

- 新增 `internal/observe` 包，使用 OTel Prometheus bridge 统一指标定义，自动暴露为 Prometheus 格式
- 新增 `/metrics` HTTP 端点（无 auth，标准 Prometheus 抓取模式）
- 新增 OTLP gRPC trace exporter，支持将 span 发送到 Jaeger/Tempo 等后端
- 在 relay handler 中埋点：请求计数、错误计数、重试计数、token 用量、费用、延迟、TTFB、活跃流数
- 在熔断器中埋点：state 变化通过 UpDownCounter 暴露
- 分布式追踪：每个 relay 请求创建 root span，每次 attempt 创建 child span，熔断器跳过记录为 span event
- 通过 nil-receiver 模式实现零开销禁用（默认关闭）
- 新增 4 个环境变量配置：`METRICS_ENABLED`、`OTEL_ENABLED`、`OTEL_EXPORTER_ENDPOINT`、`OTEL_SERVICE_NAME`

## Capabilities

### New Capabilities

- `prometheus-metrics`: Prometheus 指标导出，包含 9 个 metric instruments 覆盖请求、错误、重试、token、费用、延迟、TTFB、熔断器状态、活跃流
- `otel-tracing`: OpenTelemetry 分布式追踪，relay/attempt span 层级结构，熔断器 span event

### Modified Capabilities

<!-- 无现有 spec 需要修改 -->

## Impact

- 新增依赖：`go.opentelemetry.io/otel`（及 sdk/metric/trace/exporters）、`github.com/prometheus/client_golang`
- 修改文件：`config.go`、`relay.go`、`circuit.go`、`main.go`
- 新建文件：`observe/observe.go`、`observe/metrics.go`、`observe/tracing.go`
- 新增 HTTP 端点：`GET /metrics`（无认证）
- 当 `OTEL_ENABLED=true` 时会建立到 OTLP endpoint 的 gRPC 连接
