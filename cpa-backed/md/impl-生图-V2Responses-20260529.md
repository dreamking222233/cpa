# 生图-V2Responses 实施记录

## 任务概述

为 GPT Image 2 的 Responses 生图调用新增 `/v2/responses` 入口，避免业务侧继续依赖传统 `/v1/responses` 路径。为兼容已有客户端，本次保留 `/v1/responses`。

## 文件变更清单

- `internal/api/server.go`
  - 新增 `registerOpenAIResponsesRoutes` helper。
  - `/v1`、`/backend-api/codex` 复用统一 Responses 路由注册。
  - 新增 `/v2/responses`、`/v2/responses/compact`。
  - 根路径 endpoint 列表增加 `GET /v2/responses`、`POST /v2/responses`、`POST /v2/responses/compact`。
- `internal/api/middleware/request_logging.go`
  - WebSocket 请求日志识别兼容 `/v2/responses`。
- `internal/logging/gin_logger.go`
  - AI API 请求 ID 前缀增加 `/v2/responses`。
- `internal/api/server_test.go`
  - 新增 `/v2/responses` 路由存在性测试。
- `internal/api/middleware/request_logging_test.go`
  - 新增 `/v2/responses` WebSocket 日志跳过逻辑测试。
- `internal/logging/gin_logger_test.go`
  - 新增 `/v2/responses` AI API path 识别测试。
- `sdk/api/handlers/openai/openai_responses_handlers.go`
  - 更新 Responses handler 注释，标明同时服务 `/v1/responses` 和 `/v2/responses`。
  - `/v1/responses`、`/v1/responses/compact` 遇到 `gpt-image-2` 或 `image_generation` 请求时返回错误，提示改用 `/v2/responses`。
- `sdk/api/handlers/openai/openai_responses_websocket.go`
  - 更新 Responses WebSocket handler 注释，标明同时服务 `/v1/responses` 和 `/v2/responses`。
  - `/v1/responses` WebSocket 请求中出现生图 payload 时返回 websocket error。
- `internal/runtime/executor/codex_executor.go`
  - Codex HTTP Responses 执行路径仅在 `/v2/responses` 自动注入 `image_generation`。
- `internal/runtime/executor/codex_websockets_executor.go`
  - Codex WebSocket Responses 执行路径仅在 `/v2/responses` 自动注入 `image_generation`。
- `internal/runtime/executor/helps/payload_helpers.go`
  - 对 `/v1/responses`、`/v1/responses/compact` 做最终兜底过滤，移除 payload 规则重新加入的 `image_generation`。

## 核心代码说明

新增的 `/v2/responses` 没有单独实现一套处理链路，而是复用现有 `OpenAIResponsesAPIHandler`。因此通过 `/v2/responses` 传入的 `image_generation` tool 请求，会继续进入现有 Responses executor，并保持原有 `gpt-image-2` 工具模型处理逻辑。

为满足“只限制 `/v1/responses` 生图，`/v1/images/*` 仍可生图”，本次追加了两层限制：

- API handler 层拒绝 `/v1/responses`、`/v1/responses/compact` 中的 `gpt-image-2` 或 `image_generation` 请求。
- executor/payload 层只允许 `/v2/responses` 自动注入或保留 `image_generation`，防止 v1 通过自动注入或 payload override 绕过。

`/v1/images/generations` 与 `/v1/images/edits` 不在本次限制范围内，继续保持图片接口能力。

## 测试验证

- 已安装 Go 环境并执行 `gofmt`。
- 已执行 `git diff --check`，无空白错误。
- 已执行 `go test ./sdk/api/handlers/openai ./internal/runtime/executor ./internal/runtime/executor/helps ./internal/api ./internal/api/middleware ./internal/logging`，测试通过。
- 已执行 `go test ./...`，全仓库测试通过。
- 已执行 `go build -o /tmp/cpa-server ./cmd/server`，构建通过。
- 已使用临时配置启动服务并验证：
  - `GET /healthz` 返回 200。
  - 根路径 endpoint 列表包含 `/v2/responses`。
  - `POST /v1/responses` 普通请求未被生图门禁拦截。
  - `POST /v1/responses` 使用 `gpt-image-2` 返回 `image_generation_requires_v2_responses`。
  - `POST /v1/images/generations` 使用 `gpt-image-2` 未被 `/v1/responses` 生图门禁拦截。
  - `POST /v2/responses` 使用 `image_generation` 进入后续模型/凭据选择流程，未被 v1 门禁拦截。
- 根据自查建议，已增强 `/v2/responses` 路由测试：除鉴权命中外，也检查路由绑定到 Responses、ResponsesWebsocket、Compact 对应 handler。
- 已新增并执行 v1 Responses 生图拒绝、Codex 自动注入路径、payload override 兜底过滤相关测试。

## 待优化项

- 在安装 Go 工具链的环境中运行完整测试与 `gofmt`。
