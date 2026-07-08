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

核心代码：

```go
package publicpath

import (
    "context"
    "net/http"
    "net/url"
    "path"
    "strings"
)

const EnvPublicBasePath = "PICOCLAW_PUBLIC_BASE_PATH"

type originalRequestURIKey struct{}

// Normalize converts a configured public base path to "" or "/name".
func Normalize(raw string) string {
    raw = strings.TrimSpace(raw)
    if raw == "" || raw == "/" {
        return ""
    }
    if !strings.HasPrefix(raw, "/") {
        raw = "/" + raw
    }
    cleaned := path.Clean(raw)
    if cleaned == "/" || cleaned == "." {
        return ""
    }
    return strings.TrimRight(cleaned, "/")
}
```

这段代码定义了统一的环境变量名 `PICOCLAW_PUBLIC_BASE_PATH`，并把用户输入规范化：

```text
""          -> ""
"/"         -> ""
"diclaw"    -> "/diclaw"
"/diclaw/"  -> "/diclaw"
```

路径拼接代码：

```go
// WithBase prefixes absolute application paths with base.
func WithBase(base, appPath string) string {
    base = Normalize(base)
    if appPath == "" {
        appPath = "/"
    }
    if !strings.HasPrefix(appPath, "/") {
        appPath = "/" + appPath
    }
    if base == "" {
        return appPath
    }
    if appPath == "/" {
        return base + "/"
    }
    if appPath == base || strings.HasPrefix(appPath, base+"/") {
        return appPath
    }
    return base + appPath
}
```

作用是把应用内部路径转换成浏览器可见路径：

```text
WithBase("", "/mobile")        -> "/mobile"
WithBase("/diclaw", "/mobile") -> "/diclaw/mobile"
WithBase("/diclaw", "/")       -> "/diclaw/"
```

前缀剥离代码：

```go
// StripBase removes base from an externally visible path.
func StripBase(base, externalPath string) (string, bool) {
    base = Normalize(base)
    if base == "" {
        if externalPath == "" {
            return "/", true
        }
        return externalPath, true
    }
    if externalPath == base {
        return "/", true
    }
    if strings.HasPrefix(externalPath, base+"/") {
        stripped := strings.TrimPrefix(externalPath, base)
        if stripped == "" {
            return "/", true
        }
        return stripped, true
    }
    return externalPath, false
}
```

作用是把外部请求路径还原成原有业务路由：

```text
StripBase("/diclaw", "/diclaw/mobile")          -> "/mobile", true
StripBase("/diclaw", "/diclaw/api/auth/status") -> "/api/auth/status", true
StripBase("/diclaw", "/api/auth/status")        -> "/api/auth/status", false
```

HTTP 中间件代码：

```go
// StripPrefixMiddleware rewrites requests under base to root-relative paths
// before they reach the existing mux. Requests outside base are left unchanged
// so local root-path development remains available.
func StripPrefixMiddleware(base string, next http.Handler) http.Handler {
    base = Normalize(base)
    if base == "" {
        return next
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        originalURI := "/"
        if r.URL != nil {
            originalURI = r.URL.RequestURI()
        }
        ctx := context.WithValue(r.Context(), originalRequestURIKey{}, originalURI)
        r = r.WithContext(ctx)

        if r.URL == nil {
            next.ServeHTTP(w, r)
            return
        }
        stripped, ok := StripBase(base, r.URL.Path)
        if !ok {
            next.ServeHTTP(w, r)
            return
        }

        clone := r.Clone(ctx)
        u := *r.URL
        u.Path = stripped
        u.RawPath = ""
        clone.URL = &u
        next.ServeHTTP(w, clone)
    })
}
```

这段是后端适配的关键。Nginx 保留 `/diclaw` 转发时，Launcher 实际收到：

```text
GET /diclaw/api/auth/status
```

中间件进入原有 `http.ServeMux` 之前改写成：

```text
GET /api/auth/status
```

因此原来的路由注册逻辑不用整体改成 `/diclaw/api/...`。

保留原始 URI 的代码：

```go
// ExternalRequestURI returns the request URI before StripPrefixMiddleware
// rewrote it, falling back to the current URI.
func ExternalRequestURI(r *http.Request) string {
    if r == nil {
        return "/"
    }
    if v, ok := r.Context().Value(originalRequestURIKey{}).(string); ok && v != "" {
        return v
    }
    if r.URL == nil {
        return "/"
    }
    return r.URL.RequestURI()
}

// EnsureExternalRequestURI returns a public request URI that includes base.
func EnsureExternalRequestURI(base string, r *http.Request) string {
    uri := ExternalRequestURI(r)
    if uri == "" {
        uri = "/"
    }
    if base == "" {
        return uri
    }
    pathPart := uri
    query := ""
    if i := strings.IndexByte(uri, '?'); i >= 0 {
        pathPart = uri[:i]
        query = uri[i:]
    }
    if _, ok := StripBase(base, pathPart); ok && Normalize(base) != "" && strings.HasPrefix(pathPart, Normalize(base)) {
        return uri
    }
    return WithBase(base, pathPart) + query
}
```

作用是登录跳转时不要把用户从 `/diclaw/mobile` 错误带回 `/mobile`。

Cookie Path 和 URL 路径处理代码：

```go
// CookiePath returns a path suitable for launcher auth cookies.
func CookiePath(base string) string {
    base = Normalize(base)
    if base == "" {
        return "/"
    }
    return base
}

// AddBaseToURLPath prefixes same-origin absolute URL strings.
func AddBaseToURLPath(base, rawURL string) string {
    base = Normalize(base)
    if base == "" || rawURL == "" {
        return rawURL
    }
    u, err := url.Parse(rawURL)
    if err != nil || u.IsAbs() || !strings.HasPrefix(rawURL, "/") || strings.HasPrefix(rawURL, "//") {
        return rawURL
    }
    u.Path = WithBase(base, u.Path)
    return u.String()
}
```

测试文件覆盖了：

- `Normalize`：空值、`/`、`diclaw`、`/diclaw/`。
- `WithBase`：根路径、普通路径、已经带前缀的路径。
- `StripBase`：`/diclaw/mobile?x=1` 进入 mux 前变成 `/mobile?x=1`。
- `ExternalRequestURI`：中间件改写后仍能拿到原始 `/diclaw/mobile?x=1`。

### 2. 后端启动流程接入 `PICOCLAW_PUBLIC_BASE_PATH`

修改文件：

```text
web/backend/main.go
web/backend/main_test.go
```

新增 import：

```go
import (
    ...
    "github.com/sipeed/picoclaw/web/backend/publicpath"
    ...
)
```

启动时读取环境变量：

```go
publicBasePath := publicpath.Normalize(os.Getenv(publicpath.EnvPublicBasePath))
```

把 `publicBasePath` 注入登录接口：

```go
api.RegisterLauncherAuthRoutes(mux, api.LauncherAuthRouteOpts{
    SessionCookie:  dashboardSessionCookie,
    PublicBasePath: publicBasePath,
    PasswordStore:  passwordStore,
    StoreError:     authStoreErr,
})
```

把 `publicBasePath` 注入 API handler，用于生成 WebSocket、events、send 的浏览器可见 URL：

```go
apiHandler := api.NewHandler(launcherCfg.ConfigPath)
...
apiHandler.SetPublicBasePath(publicBasePath)
apiHandler.RegisterRoutes(mux)
```

把 `publicBasePath` 注入 dashboard auth middleware：

```go
dashAuth := middleware.LauncherDashboardAuth(middleware.LauncherDashboardAuthConfig{
    ExpectedCookie: dashboardSessionCookie,
    LocalAutoLogin: localAutoLogin,
    PublicBasePath: publicBasePath,
}, accessControlledMux)
```

在 middleware stack 中接入前缀剥离：

```go
basePathHandler := publicpath.StripPrefixMiddleware(
    publicBasePath,
    middleware.JSONContentType(dashAuth),
)

handler := middleware.Recoverer(
    middleware.Logger(
        middleware.ReferrerPolicyNoReferrer(
            basePathHandler,
        ),
    ),
)
```

浏览器自动打开地址也要带前缀：

```go
func launcherBrowserLaunchSuffix(
    needsSetup bool,
    localAutoLogin *middleware.LauncherDashboardLocalAutoLogin,
    publicBasePath string,
) string {
    if needsSetup {
        return publicpath.WithBase(publicBasePath, middleware.LauncherDashboardSetupPath)
    }
    if localAutoLogin != nil {
        return publicpath.AddBaseToURLPath(publicBasePath, localAutoLogin.URLPath())
    }
    return publicpath.WithBase(publicBasePath, "/")
}
```

调用处：

```go
browserLaunchURL = serverAddr + launcherBrowserLaunchSuffix(
    needsInitialSetup,
    localAutoLogin,
    publicBasePath,
)
```

测试补充：

```go
if got := launcherBrowserLaunchSuffix(true, autoLogin, "/diclaw"); got != "/diclaw/launcher-setup" {
    t.Fatalf("prefixed setup suffix = %q", got)
}
if got := launcherBrowserLaunchSuffix(false, nil, "/diclaw"); got != "/diclaw/" {
    t.Fatalf("prefixed root suffix = %q", got)
}
```

### 3. API handler 保存 public base path

修改文件：

```text
web/backend/api/router.go
```

`Handler` 结构体新增字段：

```go
type Handler struct {
    configPath                 string
    cfgMu                      sync.RWMutex
    cfg                        *config.Config
    serverBindHostInput        string
    serverBindHostExplicit     bool
    serverCIDRs                []string
    serverAllowLocalhostBypass bool
    serverTrustedProxyCIDRs    []string
    publicBasePath             string
    debug                      bool
    oauthMu                    sync.Mutex
    oauthFlows                 map[string]*oauthFlow
}
```

新增 setter：

```go
func (h *Handler) SetPublicBasePath(basePath string) {
    h.publicBasePath = basePath
}
```

这个字段目前主要被 `gateway_host.go` 使用，用来生成外部 WebSocket 和 Pico API 地址。

### 4. Pico WebSocket、events、send URL 带前缀

修改文件：

```text
web/backend/api/gateway_host.go
web/backend/api/gateway_host_test.go
```

新增 import：

```go
import (
    ...
    "github.com/sipeed/picoclaw/web/backend/publicpath"
)
```

修改前：

```go
func (h *Handler) buildWsURL(r *http.Request) string {
    return requestWSScheme(r) + "://" + h.picoWebUIAddr(r) + "/pico/ws"
}

func (h *Handler) buildPicoEventsURL(r *http.Request) string {
    return requestHTTPScheme(r) + "://" + h.picoWebUIAddr(r) + "/pico/events"
}

func (h *Handler) buildPicoSendURL(r *http.Request) string {
    return requestHTTPScheme(r) + "://" + h.picoWebUIAddr(r) + "/pico/send"
}
```

修改后：

```go
func (h *Handler) buildWsURL(r *http.Request) string {
    return requestWSScheme(r) + "://" + h.picoWebUIAddr(r) +
        publicpath.WithBase(h.publicBasePath, "/pico/ws")
}

func (h *Handler) buildPicoEventsURL(r *http.Request) string {
    return requestHTTPScheme(r) + "://" + h.picoWebUIAddr(r) +
        publicpath.WithBase(h.publicBasePath, "/pico/events")
}

func (h *Handler) buildPicoSendURL(r *http.Request) string {
    return requestHTTPScheme(r) + "://" + h.picoWebUIAddr(r) +
        publicpath.WithBase(h.publicBasePath, "/pico/send")
}
```

测试新增：

```go
func TestBuildWsURLIncludesPublicBasePath(t *testing.T) {
    configPath := filepath.Join(t.TempDir(), "config.json")
    h := NewHandler(configPath)
    h.SetPublicBasePath("/diclaw")

    req := httptest.NewRequest("GET", "http://launcher.local/diclaw/api/pico/info", nil)
    req.Host = "ids.byd.com"
    req.Header.Set("X-Forwarded-Proto", "https")

    if got := h.buildWsURL(req); got != "wss://ids.byd.com:443/diclaw/pico/ws" {
        t.Fatalf("buildWsURL() = %q, want %q", got, "wss://ids.byd.com:443/diclaw/pico/ws")
    }
}
```

### 5. Dashboard 登录接口 Cookie 支持前缀 Path

修改文件：

```text
web/backend/api/auth.go
web/backend/middleware/launcher_dashboard_auth.go
web/backend/middleware/launcher_dashboard_auth_test.go
```

`LauncherAuthRouteOpts` 新增 `PublicBasePath`：

```go
type LauncherAuthRouteOpts struct {
    SessionCookie  string
    SecureCookie   func(*http.Request) bool
    PublicBasePath string
    PasswordStore  PasswordStore
    StoreError     error
}
```

注册 auth handlers 时计算 Cookie Path：

```go
h := &launcherAuthHandlers{
    sessionCookie: opts.SessionCookie,
    secureCookie:  secure,
    cookiePath:    publicpath.CookiePath(opts.PublicBasePath),
    store:         opts.PasswordStore,
    storeErr:      opts.StoreError,
    loginLimit:    newLoginRateLimiter(),
}
```

`launcherAuthHandlers` 新增字段：

```go
type launcherAuthHandlers struct {
    sessionCookie string
    secureCookie  func(*http.Request) bool
    cookiePath    string
    store         PasswordStore
    storeErr      error
    loginLimit    *loginRateLimiter
}
```

登录成功写 Cookie 时使用带 Path 的方法：

```go
middleware.SetLauncherDashboardSessionCookieWithPath(
    w,
    r,
    h.sessionCookie,
    h.secureCookie,
    h.cookiePath,
)
```

退出登录清 Cookie 时同样使用带 Path 的方法：

```go
middleware.ClearLauncherDashboardSessionCookieWithPath(
    w,
    r,
    h.secureCookie,
    h.cookiePath,
)
```

middleware 中新增带 Path 的 Cookie 方法：

```go
func SetLauncherDashboardSessionCookieWithPath(
    w http.ResponseWriter,
    r *http.Request,
    sessionValue string,
    secure func(*http.Request) bool,
    cookiePath string,
) {
    if secure == nil {
        secure = DefaultLauncherDashboardSecureCookie
    }
    if strings.TrimSpace(cookiePath) == "" {
        cookiePath = "/"
    }
    http.SetCookie(w, &http.Cookie{
        Name:     LauncherDashboardCookieName,
        Value:    sessionValue,
        Path:     cookiePath,
        MaxAge:   launcherDashboardSessionMaxAgeSec,
        HttpOnly: true,
        SameSite: http.SameSiteLaxMode,
        Secure:   secure(r),
    })
}
```

保留原有方法，避免影响其它调用：

```go
func SetLauncherDashboardSessionCookie(
    w http.ResponseWriter,
    r *http.Request,
    sessionValue string,
    secure func(*http.Request) bool,
) {
    SetLauncherDashboardSessionCookieWithPath(w, r, sessionValue, secure, "/")
}
```

清理 Cookie 的带 Path 方法：

```go
func ClearLauncherDashboardSessionCookieWithPath(
    w http.ResponseWriter,
    r *http.Request,
    secure func(*http.Request) bool,
    cookiePath string,
) {
    if secure == nil {
        secure = DefaultLauncherDashboardSecureCookie
    }
    if strings.TrimSpace(cookiePath) == "" {
        cookiePath = "/"
    }
    http.SetCookie(w, &http.Cookie{
        Name:     LauncherDashboardCookieName,
        Value:    "",
        Path:     cookiePath,
        MaxAge:   -1,
        HttpOnly: true,
        SameSite: http.SameSiteLaxMode,
        Secure:   secure(r),
    })
}
```

测试新增：

```go
func TestLauncherDashboardAuth_PrefixedSessionCookiePath(t *testing.T) {
    rec := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
    SetLauncherDashboardSessionCookieWithPath(rec, req, "cookie-value", nil, "/diclaw")
    cookies := rec.Result().Cookies()
    if len(cookies) != 1 {
        t.Fatalf("cookies = %#v", cookies)
    }
    if got := cookies[0].Path; got != "/diclaw" {
        t.Fatalf("cookie path = %q, want /diclaw", got)
    }
}
```

### 6. Dashboard 未登录跳转支持前缀

修改文件：

```text
web/backend/middleware/launcher_dashboard_auth.go
web/backend/middleware/launcher_dashboard_auth_test.go
```

`LauncherDashboardAuthConfig` 新增 `PublicBasePath`：

```go
type LauncherDashboardAuthConfig struct {
    ExpectedCookie string
    LocalAutoLogin *LauncherDashboardLocalAutoLogin
    SecureCookie   func(*http.Request) bool
    // PublicBasePath is the externally visible path prefix, e.g. /diclaw.
    PublicBasePath string
}
```

拒绝未登录访问时，把 base path 传入 redirect 生成逻辑：

```go
func LauncherDashboardAuth(cfg LauncherDashboardAuthConfig, next http.Handler) http.Handler {
    ...
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        p := canonicalLauncherDashboardPath(r.URL.Path)
        ...
        rejectLauncherDashboardAuth(w, r, p, cfg.PublicBasePath)
    })
}
```

移动端未登录跳转代码：

```go
func rejectLauncherDashboardAuth(
    w http.ResponseWriter,
    r *http.Request,
    canonicalPath string,
    basePath string,
) {
    if canonicalPath == "/pico/ws" {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    if isLauncherJSONPath(canonicalPath) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        _, _ = w.Write([]byte(`{"error":"unauthorized"}`))
        return
    }
    http.Redirect(w, r, launcherDashboardLoginRedirectPath(r, canonicalPath, basePath), http.StatusFound)
}

func launcherDashboardLoginRedirectPath(r *http.Request, canonicalPath, basePath string) string {
    basePath = publicpath.Normalize(basePath)
    loginPath := publicpath.WithBase(basePath, "/launcher-login")
    if !isMobileDashboardPath(canonicalPath) {
        return loginPath
    }
    return loginPath + "?redirect=" + url.QueryEscape(publicpath.EnsureExternalRequestURI(basePath, r))
}
```

路径变化：

```text
空前缀：
/mobile -> /launcher-login?redirect=%2Fmobile

/diclaw 前缀：
/diclaw/mobile -> /diclaw/launcher-login?redirect=%2Fdiclaw%2Fmobile
```

本地自动登录也需要带前缀：

```go
func handleLauncherLocalAutoLogin(w http.ResponseWriter, r *http.Request, cfg LauncherDashboardAuthConfig) {
    basePath := publicpath.Normalize(cfg.PublicBasePath)
    if validLauncherDashboardAuth(r, cfg) {
        http.Redirect(w, r, publicpath.WithBase(basePath, "/"), http.StatusSeeOther)
        return
    }
    ...
    if cfg.LocalAutoLogin != nil && cfg.LocalAutoLogin.consume(r.URL.Query().Get("nonce")) {
        SetLauncherDashboardSessionCookieWithPath(
            w,
            r,
            cfg.ExpectedCookie,
            cfg.SecureCookie,
            publicpath.CookiePath(basePath),
        )
        http.Redirect(w, r, publicpath.WithBase(basePath, "/"), http.StatusSeeOther)
        return
    }
    rejectLauncherDashboardAuth(w, r, LauncherDashboardLocalAutoLoginPath, cfg.PublicBasePath)
}
```

测试新增：

```go
func TestLauncherDashboardAuth_PrefixedMobileRedirectPreservesExternalTarget(t *testing.T) {
    cfg := LauncherDashboardAuthConfig{
        ExpectedCookie: "deadbeef",
        PublicBasePath: "/diclaw",
    }
    next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
        t.Fatal("next handler should not run without session cookie")
    })
    h := publicpath.StripPrefixMiddleware("/diclaw", LauncherDashboardAuth(cfg, next))

    rec := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/diclaw/mobile?foo=bar", nil)
    h.ServeHTTP(rec, req)

    want := "/diclaw/launcher-login?redirect=%2Fdiclaw%2Fmobile%3Ffoo%3Dbar"
    if got := rec.Header().Get("Location"); got != want {
        t.Fatalf("Location = %q, want %q", got, want)
    }
}
```

### 7. 前端新增 public base path 工具

新增文件：

```text
web/frontend/src/lib/public-base-path.ts
```

完整核心代码：

```ts
function normalizeBasePath(raw: string | undefined): string {
  const trimmed = (raw ?? "").trim()
  if (trimmed === "" || trimmed === "/") {
    return ""
  }
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`
  return withSlash.replace(/\/+$/, "")
}

export const PUBLIC_BASE_PATH = normalizeBasePath(
  import.meta.env.VITE_PUBLIC_BASE_PATH,
)
```

这段读取构建期变量：

```bash
VITE_PUBLIC_BASE_PATH=/diclaw
```

路径拼接函数：

```ts
export function withBasePath(path: string): string {
  if (PUBLIC_BASE_PATH === "") {
    return path
  }
  if (!path.startsWith("/") || path.startsWith("//")) {
    return path
  }
  if (path === PUBLIC_BASE_PATH || path.startsWith(`${PUBLIC_BASE_PATH}/`)) {
    return path
  }
  if (path === "/") {
    return `${PUBLIC_BASE_PATH}/`
  }
  return `${PUBLIC_BASE_PATH}${path}`
}
```

示例：

```text
withBasePath("/api/auth/status") -> "/diclaw/api/auth/status"
withBasePath("/mobile")          -> "/diclaw/mobile"
withBasePath("/")                -> "/diclaw/"
```

路径剥离函数：

```ts
export function stripBasePath(pathname: string): string {
  if (PUBLIC_BASE_PATH === "") {
    return pathname || "/"
  }
  if (pathname === PUBLIC_BASE_PATH) {
    return "/"
  }
  if (pathname.startsWith(`${PUBLIC_BASE_PATH}/`)) {
    return pathname.slice(PUBLIC_BASE_PATH.length) || "/"
  }
  return pathname || "/"
}
```

用途是判断页面类型时继续按内部路径判断：

```text
stripBasePath("/diclaw/mobile")         -> "/mobile"
stripBasePath("/diclaw/launcher-login") -> "/launcher-login"
```

fetch 输入自动补前缀：

```ts
export function withBasePathInput(input: RequestInfo | URL): RequestInfo | URL {
  if (typeof input === "string") {
    return withBasePath(input)
  }
  if (input instanceof URL) {
    if (
      typeof globalThis.location !== "undefined" &&
      input.origin === globalThis.location.origin
    ) {
      const copy = new URL(input.href)
      copy.pathname = withBasePath(copy.pathname)
      return copy
    }
    return input
  }
  return input
}
```

### 8. Vite 构建和开发代理支持前缀

修改文件：

```text
web/frontend/vite.config.ts
```

新增 base path 规范化：

```ts
function normalizeBasePath(raw: string | undefined): string {
  const trimmed = (raw ?? "").trim()
  if (trimmed === "" || trimmed === "/") {
    return ""
  }
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`
  return withSlash.replace(/\/+$/, "")
}

const publicBasePath = normalizeBasePath(process.env.VITE_PUBLIC_BASE_PATH)
const proxyPath = (path: string) =>
  publicBasePath === "" ? path : `${publicBasePath}${path}`
const stripProxyBase = (path: string) =>
  publicBasePath === "" ? path : path.slice(publicBasePath.length) || "/"
```

构建 base 配置：

```ts
export default defineConfig({
  base: publicBasePath === "" ? "/" : `${publicBasePath}/`,
  ...
})
```

效果：

```text
默认构建：
<script src="/assets/index-xxx.js">

VITE_PUBLIC_BASE_PATH=/diclaw 构建：
<script src="/diclaw/assets/index-xxx.js">
```

开发代理配置：

```ts
server: {
  proxy: {
    [proxyPath("/api")]: {
      target: "http://localhost:18800",
      changeOrigin: true,
      rewrite: stripProxyBase,
    },
    [proxyPath("/pico/media")]: {
      target: "http://localhost:18800",
      changeOrigin: true,
      rewrite: stripProxyBase,
    },
    [proxyPath("/pico/ws")]: {
      target: "ws://localhost:18800",
      ws: true,
      rewrite: stripProxyBase,
    },
  },
}
```

带 `/diclaw` 时，Vite dev server 收到：

```text
/diclaw/api/auth/status
/diclaw/pico/ws
```

会代理给后端：

```text
/api/auth/status
/pico/ws
```

### 9. TanStack Router 支持 basepath

修改文件：

```text
web/frontend/src/main.tsx
```

新增 import：

```ts
import { PUBLIC_BASE_PATH } from "./lib/public-base-path"
```

router 初始化新增 `basepath`：

```ts
const router = createRouter({
  routeTree,
  basepath: PUBLIC_BASE_PATH || "/",
  context: {
    queryClient,
  },
})
```

这样源码里的路由仍然可以保持：

```text
/mobile
/launcher-login
/launcher-setup
/config
```

浏览器访问时由 router 映射到：

```text
/diclaw/mobile
/diclaw/launcher-login
/diclaw/launcher-setup
/diclaw/config
```

### 10. 前端 API 请求自动带前缀

修改文件：

```text
web/frontend/src/api/http.ts
web/frontend/src/api/launcher-auth.ts
web/frontend/src/components/config/config-sections.tsx
```

`launcherFetch()` 修改前：

```ts
const res = await fetch(input, {
  credentials: "same-origin",
  ...init,
})
```

修改后：

```ts
const res = await fetch(withBasePathInput(input), {
  credentials: "same-origin",
  ...init,
})
```

未登录重定向也要带前缀：

```ts
globalThis.location.assign(
  isMobilePathname(pathname)
    ? buildLauncherAuthPath(
        "/launcher-login",
        getCurrentMobileRedirectTarget(),
      )
    : withBasePath("/launcher-login"),
)
```

登录 API 修改前：

```ts
const res = await fetch("/api/auth/login", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  credentials: "same-origin",
  body: JSON.stringify({ password }),
})
```

修改后：

```ts
const res = await fetch(withBasePath("/api/auth/login"), {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  credentials: "same-origin",
  body: JSON.stringify({ password }),
})
```

同样修改的接口：

```ts
fetch(withBasePath("/api/auth/status"), ...)
fetch(withBasePath("/api/auth/logout"), ...)
fetch(withBasePath("/api/auth/setup"), ...)
```

配置页直接 fetch 的接口也补前缀：

```ts
const res = await fetch(
  withBasePath("/api/config/test-command-patterns"),
  {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      allow_patterns: allowPatterns,
      deny_patterns: denyPatterns,
      command: testCommand,
    }),
  },
)
```

### 11. 前端登录跳转、移动端 redirect 和 auth path 判断支持前缀

修改文件：

```text
web/frontend/src/features/mobile/redirect.ts
web/frontend/src/lib/launcher-login-path.ts
web/frontend/src/routes/__root.tsx
web/frontend/src/components/app-header.tsx
```

移动端 redirect 工具新增 import：

```ts
import { stripBasePath, withBasePath } from "@/lib/public-base-path"
```

安全 redirect target 返回带前缀的同源路径：

```ts
export function getSafeRedirectTarget(
  value: string | null | undefined,
  fallback = withBasePath(DEFAULT_REDIRECT_FALLBACK),
): string {
  ...
  try {
    const parsed = new URL(
      target,
      globalThis.location?.origin ?? "http://localhost",
    )
    if (parsed.origin !== (globalThis.location?.origin ?? parsed.origin)) {
      return fallback
    }
    return `${withBasePath(parsed.pathname)}${parsed.search}${parsed.hash}`
  } catch {
    return fallback
  }
}
```

登录页路径生成：

```ts
export function buildLauncherAuthPath(
  pathname: "/launcher-login" | "/launcher-setup",
  redirectTarget: string,
): string {
  const safeRedirect = getSafeRedirectTarget(redirectTarget)
  const authPath = withBasePath(pathname)
  if (safeRedirect === withBasePath(DEFAULT_REDIRECT_FALLBACK)) {
    return authPath
  }
  return `${authPath}?redirect=${encodeURIComponent(safeRedirect)}`
}
```

移动端路径判断先剥离前缀：

```ts
export function isMobilePathname(pathname: string): boolean {
  const stripped = stripBasePath(pathname)
  return stripped === "/mobile" || stripped.startsWith("/mobile/")
}
```

当前移动端 redirect target 保留 `/diclaw`：

```ts
export function getCurrentMobileRedirectTarget(): string {
  if (typeof globalThis.location === "undefined") {
    return withBasePath("/mobile")
  }
  const { pathname, search, hash } = globalThis.location
  if (!isMobilePathname(pathname || "/")) {
    return withBasePath("/mobile")
  }
  return `${withBasePath(pathname || "/mobile")}${search}${hash}`
}
```

登录页路径判断先剥离前缀：

```ts
import { stripBasePath } from "@/lib/public-base-path"

/** Normalize URL pathname for comparisons (trailing slashes, empty). */
export function normalizePathname(p: string): string {
  const t = stripBasePath(p).replace(/\/+$/, "")
  return t === "" ? "/" : t
}
```

Root route 中先把浏览器路径和 router path 转成内部路径：

```ts
const windowPath =
  typeof globalThis.location !== "undefined"
    ? globalThis.location.pathname || "/"
    : routerState.pathname
const appWindowPath = stripBasePath(windowPath)
const appRouterPath = stripBasePath(routerState.pathname)

const isAuthPage =
  isLauncherAuthPathname(appWindowPath) ||
  isLauncherAuthPathname(appRouterPath) ||
  routerState.matches.some(
    (m) => m.routeId === "/launcher-login" || m.routeId === "/launcher-setup",
  )
const isMobilePage =
  isMobilePathname(appWindowPath) ||
  isMobilePathname(appRouterPath) ||
  routerState.matches.some((m) => m.routeId === "/mobile")
```

Root route 未登录跳转带前缀：

```ts
if (!s.initialized) {
  globalThis.location.assign(
    isMobilePage
      ? buildLauncherAuthPath("/launcher-setup", authRedirectTarget)
      : withBasePath("/launcher-setup"),
  )
} else if (!s.authenticated) {
  globalThis.location.assign(
    isMobilePage
      ? buildLauncherAuthPath("/launcher-login", authRedirectTarget)
      : withBasePath("/launcher-login"),
  )
}
```

退出登录跳转带前缀：

```ts
const handleLogout = async () => {
  await postLauncherDashboardLogout()
  globalThis.location.assign(withBasePath("/launcher-login"))
}
```

### 12. 前端 WebSocket 连接带前缀

修改文件：

```text
web/frontend/src/features/chat/controller.ts
```

新增 import：

```ts
import { withBasePath } from "@/lib/public-base-path"
```

修改前：

```ts
const wsScheme = window.location.protocol === "https:" ? "wss:" : "ws:"
const wsUrl = `${wsScheme}//${window.location.host}/pico/ws`
const url = `${wsUrl}?session_id=${encodeURIComponent(sessionId)}`
const socket = new WebSocket(url)
```

修改后：

```ts
const wsScheme = window.location.protocol === "https:" ? "wss:" : "ws:"
const wsUrl = `${wsScheme}//${window.location.host}${withBasePath("/pico/ws")}`
const url = `${wsUrl}?session_id=${encodeURIComponent(sessionId)}`
const socket = new WebSocket(url)
```

路径变化：

```text
默认本地：
ws://127.0.0.1:18800/pico/ws

/diclaw 前缀：
ws://127.0.0.1:18800/diclaw/pico/ws

HTTPS 内网域名：
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
