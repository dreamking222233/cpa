# 生图-V2Responses 实施方案

## 用户原始需求

把调用 `gpt-image-2` 模型的接口改为 `v2/responses`，不采用传统的 `v1`。

## 技术方案设计

当前项目已支持 `/v1/responses`，并通过 Responses API 的 `image_generation` tool 触发 `gpt-image-2` 生图能力。为避免破坏现有 OpenAI 兼容客户端，本次不移除 `/v1/responses`，新增 `/v2/responses` 作为新的调用入口，复用现有 `OpenAIResponsesHandlers`。

具体策略：

- 在 API Server 中新增 `/v2` 分组。
- `/v2/responses` 的 `GET` 和 `POST` 复用现有 Responses websocket 与普通请求处理器。
- `/v2/responses/compact` 同步提供，保持 Responses 能力完整。
- `/v1/responses` 保留普通 Responses 功能，但拒绝 `gpt-image-2` 或 `image_generation` 生图请求。
- `/v1/images/generations` 与 `/v1/images/edits` 保持原有图片接口能力，可以继续用于生图。
- Codex executor 仅在 `/v2/responses` 请求路径下自动注入 `image_generation` 工具。
- 请求日志与 Gin 请求 ID 识别补充 `/v2/responses`，确保新增入口仍能进入请求日志链路。
- 底层 executor 不改动，上游仍由现有 Responses 执行器处理 `image_generation` tool。

## 涉及文件清单

- `internal/api/server.go`
- `internal/api/middleware/request_logging.go`
- `internal/logging/gin_logger.go`
- `internal/api/server_test.go`
- `internal/api/middleware/request_logging_test.go`
- `internal/logging/gin_logger_test.go`
- `sdk/api/handlers/openai/openai_responses_handlers.go`
- `sdk/api/handlers/openai/openai_responses_websocket.go`
- `sdk/api/handlers/openai/openai_images_handlers.go`
- `internal/runtime/executor/codex_executor.go`
- `internal/runtime/executor/codex_websockets_executor.go`
- `internal/runtime/executor/helps/payload_helpers.go`

## 实施步骤概要

1. 新增统一注册 Responses 路由的 helper，减少 `/v1` 与 `/v2` 重复代码。
2. 在 `/v2` 分组注册 `/responses`、`/responses/compact`。
3. 请求日志 websocket 判断兼容 `/v2/responses`。
4. Gin AI API path 前缀加入 `/v2/responses`。
5. 增加路由和日志判断测试。
6. 增加 v1 生图拒绝测试与 executor 注入路径测试。
7. 运行可用测试；若本地缺少 Go 工具链，记录验证受限。
