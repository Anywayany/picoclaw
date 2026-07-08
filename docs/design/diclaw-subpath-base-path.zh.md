# DiClaw 子路径适配代码变更说明

## 修改目的

本次修改用于支持 PicoClaw Launcher 部署在非根路径下，例如：

```text
https://aiservice.byd.com/diclaw/
https://ids.byd.com/diclaw/
```

修改前，前端和后端都默认工作在站点根路径：

```text
/mobile
/api/auth/status
/pico/ws
/assets/...
/launcher-login
```

如果直接用 Nginx 把 `/diclaw/` 代理到 Launcher，会出现这些问题：

- 静态资源仍然从 `/assets/...` 加载，而不是 `/diclaw/assets/...`。
- 前端 API 请求仍然访问 `/api/...`，会跑到域名根路径。
- 未登录跳转会跳到 `/launcher-login`，而不是 `/diclaw/launcher-login`。
- WebSocket 会连接 `/pico/ws`，而不是 `/diclaw/pico/ws`。
- 登录 Cookie 的 `Path` 是 `/`，会影响同域名下其它系统路径。

本次改动的目标：

- 本地默认行为不变，仍然支持 `http://127.0.0.1:18800/mobile`。
- 部署时通过环境变量启用 `/diclaw` 前缀。
- 前端、后端、登录跳转、Cookie、WebSocket、静态资源都统一感知 `/diclaw`。
- Nginx 转发时保留 `/diclaw` 路径，不要求 Nginx strip 前缀。

## 修改思路

本次采用“可配置 public base path”的方式。

前端构建时使用：

```bash
VITE_PUBLIC_BASE_PATH=/diclaw
```

后端运行时使用：

```bash
PICOCLAW_PUBLIC_BASE_PATH=/diclaw
```

当这两个变量为空时，行为保持原样：

```text
/mobile
/api/auth/status
/pico/ws
/assets/...
```

当它们设置为 `/diclaw` 时，对外路径变为：

```text
/diclaw/mobile
/diclaw/api/auth/status
/diclaw/pico/ws
/diclaw/assets/...
```

后端设计重点：请求进入原有 `http.ServeMux` 前，先剥离外部路径前缀。

例如 Nginx 转发进来的请求是：

```text
GET /diclaw/api/auth/status
```

后端中间件在进入原有路由前改写为：

```text
GET /api/auth/status
```

因此原有业务 handler 仍然注册 `/api/...`、`/mobile`、`/pico/ws`，不需要大规模重写已有路由。

同时，为了兼容本地开发和已有部署，后端仍然允许无前缀路径：

```text
GET /api/auth/status
GET /mobile
```

## 代码修改内容

### 1. 后端新增 public path 工具

新增文件：

```text
web/backend/publicpath/publicpath.go
web/backend/publicpath/publicpath_test.go
```

主要能力：

- `Normalize(raw string)`：把 `diclaw`、`/diclaw/` 规范化为 `/diclaw`，空值或 `/` 规范化为空字符串。
- `WithBase(base, appPath string)`：把应用内部路径拼成外部路径，例如 `/mobile` -> `/diclaw/mobile`。
- `StripBase(base, externalPath string)`：把外部路径剥离前缀，例如 `/diclaw/mobile` -> `/mobile`。
- `StripPrefixMiddleware(base, next)`：请求进入原有 mux 前剥离前缀。
- `ExternalRequestURI(r)`：保留剥离前的原始 URI，用于登录 redirect。
- `CookiePath(base)`：生成 Cookie Path，空前缀为 `/`，`/diclaw` 前缀为 `/diclaw`。

测试覆盖：

- 空前缀和 `/diclaw` 前缀的规范化。
- 路径拼接和路径剥离。
- 中间件是否把 `/diclaw/mobile?x=1` 改写成 `/mobile?x=1`，同时保留原始 URI。

### 2. 后端启动流程接入 `PICOCLAW_PUBLIC_BASE_PATH`

修改文件：

```text
web/backend/main.go
web/backend/main_test.go
```

主要改动：

```go
publicBasePath := publicpath.Normalize(os.Getenv(publicpath.EnvPublicBasePath))
```

然后把 `publicBasePath` 传给：

- launcher auth routes
- API handler
- dashboard auth middleware
- 浏览器自动打开 URL 生成逻辑

middleware stack 中加入前缀剥离：

```go
publicpath.StripPrefixMiddleware(publicBasePath, middleware.JSONContentType(dashAuth))
```

测试覆盖：

- 默认空前缀时，启动浏览器路径仍为 `/launcher-setup`、`/launcher-auto-login`、`/`。
- `/diclaw` 前缀时，启动浏览器路径为 `/diclaw/launcher-setup`、`/diclaw/launcher-auto-login`、`/diclaw/`。

### 3. API handler 保存 public base path

修改文件：

```text
web/backend/api/router.go
```

新增字段和 setter：

```go
publicBasePath string

func (h *Handler) SetPublicBasePath(basePath string)
```

用途：API handler 在生成浏览器可见 URL 时知道当前外部路径前缀。

### 4. Pico WebSocket URL 带前缀

修改文件：

```text
web/backend/api/gateway_host.go
web/backend/api/gateway_host_test.go
```

修改前：

```text
wss://ids.byd.com:443/pico/ws
```

修改后，在 `PICOCLAW_PUBLIC_BASE_PATH=/diclaw` 时返回：

```text
wss://ids.byd.com:443/diclaw/pico/ws
```

同样适配：

```text
/pico/events
/pico/send
```

测试新增：

```text
TestBuildWsURLIncludesPublicBasePath
```

### 5. Dashboard 登录 Cookie 支持前缀 Path

修改文件：

```text
web/backend/api/auth.go
web/backend/middleware/launcher_dashboard_auth.go
web/backend/middleware/launcher_dashboard_auth_test.go
```

修改前，登录 Cookie 固定：

```text
Path=/
```

修改后：

- 默认空前缀仍然是 `Path=/`。
- `/diclaw` 前缀部署时是 `Path=/diclaw`。

这样可以避免 Cookie 泄到同域名其它系统路径。

新增方法：

```go
SetLauncherDashboardSessionCookieWithPath(...)
ClearLauncherDashboardSessionCookieWithPath(...)
```

测试新增：

```text
TestLauncherDashboardAuth_PrefixedSessionCookiePath
```

### 6. Dashboard 未登录跳转支持前缀

修改文件：

```text
web/backend/middleware/launcher_dashboard_auth.go
web/backend/middleware/launcher_dashboard_auth_test.go
```

修改前：

```text
/mobile
  -> /launcher-login?redirect=%2Fmobile
```

修改后，在 `/diclaw` 前缀部署时：

```text
/diclaw/mobile
  -> /diclaw/launcher-login?redirect=%2Fdiclaw%2Fmobile
```

测试新增：

```text
TestLauncherDashboardAuth_PrefixedMobileRedirectPreservesExternalTarget
```

### 7. 前端新增 public base path 工具

新增文件：

```text
web/frontend/src/lib/public-base-path.ts
```

主要能力：

- `PUBLIC_BASE_PATH`：读取 `import.meta.env.VITE_PUBLIC_BASE_PATH`。
- `withBasePath(path)`：把 `/api/auth/status` 转成 `/diclaw/api/auth/status`。
- `stripBasePath(pathname)`：把 `/diclaw/mobile` 转成 `/mobile`，用于页面判断。
- `withBasePathInput(input)`：给 `fetch` 的同源绝对路径自动补前缀。

### 8. Vite 构建和开发代理支持前缀

修改文件：

```text
web/frontend/vite.config.ts
```

新增：

```ts
base: publicBasePath === "" ? "/" : `${publicBasePath}/`
```

构建后的 HTML 会引用：

```text
/diclaw/assets/index-xxx.js
/diclaw/assets/index-xxx.css
/diclaw/favicon.ico
/diclaw/site.webmanifest
```

开发代理也会根据前缀工作：

```text
/diclaw/api         -> http://localhost:18800/api
/diclaw/pico/ws    -> ws://localhost:18800/pico/ws
/diclaw/pico/media -> http://localhost:18800/pico/media
```

### 9. TanStack Router 支持 basepath

修改文件：

```text
web/frontend/src/main.tsx
```

新增：

```ts
basepath: PUBLIC_BASE_PATH || "/"
```

前端内部路由仍然写：

```text
/mobile
/launcher-login
/config
```

浏览器地址会映射到：

```text
/diclaw/mobile
/diclaw/launcher-login
/diclaw/config
```

### 10. 前端 API 请求自动带前缀

修改文件：

```text
web/frontend/src/api/http.ts
web/frontend/src/api/launcher-auth.ts
web/frontend/src/components/config/config-sections.tsx
```

主要改动：

- `launcherFetch()` 使用 `withBasePathInput()`。
- 登录、退出、setup、auth status 显式使用 `withBasePath()`。
- 配置页面里的命令模式测试接口也改为带前缀。

示例：

```text
/api/auth/status
  -> /diclaw/api/auth/status
```

### 11. 前端登录跳转、移动端 redirect 和 auth path 判断支持前缀

修改文件：

```text
web/frontend/src/features/mobile/redirect.ts
web/frontend/src/lib/launcher-login-path.ts
web/frontend/src/routes/__root.tsx
web/frontend/src/components/app-header.tsx
```

主要改动：

- 判断当前页面是否是移动端页面时，先剥离 `/diclaw`。
- 未登录跳转时使用 `/diclaw/launcher-login`。
- 移动端 redirect 保留 `/diclaw/mobile`。
- 退出登录后跳转到 `/diclaw/launcher-login`。

### 12. 前端 WebSocket 连接带前缀

修改文件：

```text
web/frontend/src/features/chat/controller.ts
```

修改前：

```text
ws://host/pico/ws
```

修改后，在 `/diclaw` 前缀部署时：

```text
ws://host/diclaw/pico/ws
```

正式 HTTPS 环境下为：

```text
wss://aiservice.byd.com/diclaw/pico/ws
```

## 复现验证

### 默认根路径验证

```bash
make -C web build

PICOCLAW_HOME=/workspaces/picoclaw/.runtime/picoclaw-home \
PICOCLAW_BINARY=/workspaces/picoclaw/build/picoclaw \
/workspaces/picoclaw/web/build/picoclaw-launcher \
  -no-browser \
  -host 127.0.0.1 \
  -port 18800 \
  -debug
```

验证：

```bash
curl -i http://127.0.0.1:18800/mobile
curl -i http://127.0.0.1:18800/api/auth/status
```

预期：

```text
/mobile -> 302 /launcher-login?redirect=%2Fmobile
/api/auth/status -> 200 JSON
```

### `/diclaw` 前缀验证

```bash
VITE_PUBLIC_BASE_PATH=/diclaw make -C web build

PICOCLAW_HOME=/workspaces/picoclaw/.runtime/picoclaw-home \
PICOCLAW_BINARY=/workspaces/picoclaw/build/picoclaw \
PICOCLAW_PUBLIC_BASE_PATH=/diclaw \
/workspaces/picoclaw/web/build/picoclaw-launcher \
  -no-browser \
  -host 127.0.0.1 \
  -port 18800 \
  -debug
```

验证：

```bash
curl -i http://127.0.0.1:18800/diclaw/mobile
curl -i http://127.0.0.1:18800/diclaw/api/auth/status
curl -i http://127.0.0.1:18800/diclaw/launcher-login
```

预期：

```text
/diclaw/mobile -> 302 /diclaw/launcher-login?redirect=%2Fdiclaw%2Fmobile
/diclaw/api/auth/status -> 200 JSON
/diclaw/launcher-login -> 200 HTML
```

验证静态资源：

```bash
curl -i http://127.0.0.1:18800/diclaw/launcher-login | grep /diclaw/assets
```

验证 WebSocket 未登录路径：

```bash
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" \
  -H "Sec-WebSocket-Version: 13" \
  http://127.0.0.1:18800/diclaw/pico/ws
```

预期：

```text
401 unauthorized
```

## 提交前验证命令

```bash
go test ./web/backend/...
pnpm --dir web/frontend build
VITE_PUBLIC_BASE_PATH=/diclaw pnpm --dir web/frontend build
VITE_PUBLIC_BASE_PATH=/diclaw make -C web build
```
