# cpa

本仓库包含两个项目目录：

- `cpa-backed/`：CLIProxyAPI 后端项目
- `cpa-front/`：管理中心前端项目

## 启动说明

### 1. 启动后端

```bash
cd cpa-backed
cp config.example.yaml config.yaml
go run ./cmd/server --config config.yaml
```

后端默认提供管理中心访问入口，当前本地常用访问地址：

```text
http://127.0.0.1:8317/management.html
```

### 2. 启动前端开发服务

```bash
cd cpa-front
npm install
npm run dev -- --host 0.0.0.0
```

前端开发服务启动后，按终端输出的 Vite 地址访问。开发模式下前端会通过 `/api/management` 请求后端管理 API，请保持后端服务同时运行。

### 3. 构建前端并同步到后端

```bash
cd cpa-front
npm run build
cp dist/index.html ../cpa-backed/static/management.html
```

同步后重新启动后端，或刷新 `http://127.0.0.1:8317/management.html`，即可通过后端同一端口访问最新管理中心页面。

### 4. 常用验证命令

```bash
cd cpa-backed
gofmt -w .
go build -o test-output ./cmd/server && rm test-output

cd ../cpa-front
npm run type-check
npm run build
```

## 本地结构说明

- 后端来源：当前本地 CPA 主仓库工作区
- 前端来源：当前本地管理中心前端工作区

## 说明

- 本次发布内容不包含本地运行时密钥、授权文件、日志和依赖缓存
- 前端 `node_modules`、`dist` 未纳入版本管理
- 后端本地 `config.yaml`、`auths/` 未纳入版本管理
