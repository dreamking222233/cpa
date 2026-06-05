# 本地 CPA 状态总结

更新时间：2026-06-05（北京时间）

## 1. 仓库与版本

- 主仓库：`/Volumes/work_space/cpa`
- 上游仓库：`https://github.com/router-for-me/CLIProxyAPI`
- 当前主仓库版本：`v7.1.45`
- 当前主仓库提交：`5753d1a0896fd5bb9ace47adb17b0174ceb79e4d`
- 前端仓库目录：`.frontend/Cli-Proxy-API-Management-Center`
- 当前前端提交：`87702bb5a0625072f03cbc49f4da9d918a7910c0`

## 2. 当前已完成的本地定制

### 2.1 代理池

- 已支持多代理配置，而不是单一 `socks5` 代理。
- 已支持协议：
  - `socks5://`
  - `socks5h://`
  - `http://`
  - `https://`
- 已增加代理池策略配置：
  - 随机使用
  - 固定请求次数后轮换
- 已增加代理测试能力，可在管理端测试代理是否可用。
- 已增加代理池请求日志页面，可查看请求使用了哪个代理。

### 2.2 请求日志页面

- 已增加请求日志页面。
- 当前可以查看：
  - 请求时间（北京时间）
  - 模型名称
  - 输入 token
  - 输出 token
  - cached token
  - reasoning token
  - 总 token
  - 估算成本（参考 OpenAI 官方价格）
  - 使用的授权文件
- 当前后端配置中 `request-log: true`，请求日志功能已启用。

### 2.3 授权文件批量操作

- 认证文件页面已支持批量启用。
- 认证文件页面已支持批量禁用。
- 可通过勾选账号后批量处理，不再需要单个操作。

### 2.4 图片生成接口限制

- 已保留原有 `/v1/responses` 文本能力。
- 已限制 `/v1/responses` 不允许图片生成请求。
- 若通过 `/v1/responses` 发起生图请求，将返回错误提示。
- 图片生成改为走指定的新路径策略，不影响普通文本请求。
- `/v1/images...` 类接口仍可用于图片生成。

### 2.5 前端管理界面

- 已接入官方前端项目。
- 左侧菜单栏已显示“请求日志”入口。
- 代理池相关页面和能力已接入前端。

## 3. 当前运行状态

### 3.1 服务地址

- 后端：`http://127.0.0.1:8317`
- 前端：`http://127.0.0.1:4174`
- 请求日志页面：`http://127.0.0.1:4174/#/request-logs`

### 3.2 当前后端关键配置

- API Key：`sk-X9PDjTXL7vjpNY0Lv`
- `request-log: true`
- `disable-image-generation: "chat"`
- 代理池策略：
  - `mode: random`
  - `requests-per-proxy: 100`
- 当前已配置 1 条代理池记录。

## 4. 当前授权文件状态

- 当前后端已加载 4 个 Codex OAuth 授权文件。
- 其中 3 个处于可用状态。
- 其中 `5.json` 已出现失效：
  - 状态：`error`
  - 原因：`Your authentication token has been invalidated. Please try signing in again.`
- 当前系统在请求失败后可自动切换到其他可用授权文件。

## 5. Codex 本地接入状态

- `~/.codex/config.toml` 已改为指向本地 CPA：
  - `base_url = "http://127.0.0.1:8317/v1"`
- 已关闭 `requires_openai_auth`
- 已显式配置本地 CPA 的 `Authorization` 头
- 已在 `~/.zshrc` 中添加 `codex()` 包装函数，用于：
  - 为 `127.0.0.1` / `localhost` 强制设置 `NO_PROXY`
  - 清空 `HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY`
- 目的：避免 Codex 本地请求走代理后无法访问本地 CPA

## 6. 已验证结果

- `GET /v1/models` 可正常返回 `200`
- `POST /v1/responses` 普通文本请求可正常返回 `200`
- 请求日志接口可正常返回数据
- 在交互式 `zsh` 环境中，`codex exec` 已验证可通过本地 CPA 正常发起请求

## 7. 当前需要注意的事项

- 主仓库与前端仓库目前仍是两个独立 Git 仓库。
- 主仓库中的 `.frontend/` 目录当前不是主仓库的一部分。
- 如果需要把前端改动也纳入 GitHub 管理，建议为前端单独建仓库，或后续再决定是否改为子模块方案。
- 失效授权文件 `5.json` 建议重新登录刷新，或直接禁用，避免继续命中失败重试。
