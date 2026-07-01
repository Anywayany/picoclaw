# 当前代码相对 upstream/main / v0.3.1 的修改说明

生成时间：2026-07-01  
比较基线：`upstream/main` / `v0.3.1` / `nightly`，提交 `52320f48`  
当前版本：`wendi-mobile-h5`，提交 `6db0e9fc`  
比较命令：`git diff upstream/main..HEAD`

说明：本文只整理当前分支相对上游 `picoclaw` 的 `upstream/main / v0.3.1` 的差异。
## 总览

当前分支相对 `upstream/main / v0.3.1` 多 4 个提交：

- `a9a93a3c` wendi-mobile-h5 初版页面开发
- `16510e01` 修复使用非多模态大模型API发送图片直接报错的问题
- `449bf545` 删除一些多余文件
- `6db0e9fc` 修改移动端UI为DiAgent，删除不必要文件

差异规模：

- 变更文件：51 个
- 新增行数：约 5,413 行
- 删除行数：约 47 行

主要修改目的分为：

- 新增移动端 H5 聊天入口。
- 让移动端登录/初始化后能回到 `/mobile`。
- 让移动端图片附件能安全进入 Agent，避免非多模态模型直接收到图片报错。
- 提供移动端反向代理部署示例。
- 补充移动端文案、路由和测试。
- 清理部分中间截图报告，但仍保留了一批 `.runtime` 运行时产物。

## 1. 新增移动端 H5 / DiAgent 聊天入口

### 修改思路

上游 `v0.3.1` 只有桌面控制台式 Web UI。当前分支新增独立 `/mobile` 路由，把移动端聊天做成第一屏，不进入桌面 `AppLayout`。移动端页面复用已有 `usePicoChat`、`useChatModels`、`AssistantMessage`、`UserMessage` 和 session history 能力，但在布局上拆成移动端专用 shell、顶部栏、消息列表、底部输入栏和会话历史抽屉。

设计重点：

- `/mobile` 页面独立承载聊天体验。
- 顶部只保留会话历史、品牌 `DiAgent`、新建会话。
- 中间是可滚动消息列表。
- 底部是带安全区适配的输入栏。
- 网关未运行或 WebSocket 未连接时，禁用输入并给出状态文案。

### 具体修改代码

新增路由文件：

```tsx
// web/frontend/src/routes/mobile.tsx
import { createFileRoute } from "@tanstack/react-router"

import { MobileChatPage } from "@/features/mobile/mobile-chat-page"

export const Route = createFileRoute("/mobile")({
  component: MobileChatPage,
})
```

新增移动端 shell，替代桌面 `AppLayout`：

```tsx
// web/frontend/src/features/mobile/mobile-shell.tsx
export function MobileShell({
  children,
  authError,
  onDismissAuthError,
  devtools,
}: MobileShellProps) {
  return (
    <TooltipProvider>
      <main className="bg-background text-foreground flex h-dvh min-h-dvh w-full flex-col overflow-hidden">
        {authError && (
          <div className="bg-destructive text-destructive-foreground fixed inset-x-0 top-0 z-[100] flex items-center justify-between px-4 py-2 text-sm shadow-md">
            <span>Auth service error: {authError}</span>
            <button
              className="ml-4 opacity-70 hover:opacity-100"
              onClick={onDismissAuthError}
              aria-label="Dismiss"
            >
              x
            </button>
          </div>
        )}
        {children}
        {devtools}
        <Toaster position="bottom-center" />
      </main>
    </TooltipProvider>
  )
}
```

根路由按路径切换桌面/移动端布局：

```tsx
// web/frontend/src/routes/__root.tsx
const isMobilePage =
  isMobilePathname(windowPath) ||
  isMobilePathname(routerState.pathname) ||
  routerState.matches.some((m) => m.routeId === "/mobile")

if (isMobilePage) {
  return (
    <MobileShell
      authError={authError}
      onDismissAuthError={() => setAuthError(null)}
      devtools={import.meta.env.DEV ? <TanStackRouterDevtools /> : null}
    >
      <Outlet />
    </MobileShell>
  )
}
```

新增移动端主页面：

```tsx
// web/frontend/src/features/mobile/mobile-chat-page.tsx
export function MobileChatPage() {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [input, setInput] = useState("")
  const [attachments, setAttachments] = useState<ChatAttachment[]>([])
  const { status: gatewayStatus } = useAtomValue(gatewayAtom)
  const { messages, connectionState, isTyping, sendMessage, newChat, activeSessionId, switchSession } =
    usePicoChat()

  useMobileGatewayPolling()

  const disabledKey = getMobileDisabledKey({ gatewayStatus, connectionState })
  const disabled = disabledKey !== null
  const canSend = !disabled && (input.trim().length > 0 || attachments.length > 0)

  return (
    <div className="bg-background flex h-full min-h-0 flex-col">
      <header className="border-border/60 bg-background/95 supports-[backdrop-filter]:bg-background/80 grid h-12 shrink-0 grid-cols-[36px_1fr_36px] items-center border-b px-3 backdrop-blur">
        <MobileSessionHistory
          activeSessionId={activeSessionId}
          onSwitchSession={switchSession}
          onNewChat={newChat}
        />
        <div className="min-w-0 text-center">
          <div className="truncate text-sm font-semibold">DiAgent</div>
          <div className="truncate text-xs">{statusText}</div>
        </div>
        <Button onClick={handleNewChat} size="icon">
          <IconPlus className="size-5" />
        </Button>
      </header>

      <MobileMessageList messages={messages} isTyping={isTyping} />
      <MobileChatComposer ... />
    </div>
  )
}
```

新增移动端消息列表：

```tsx
// web/frontend/src/features/mobile/mobile-message-list.tsx
export function MobileMessageList({ messages, isTyping }: MobileMessageListProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [isAtBottom, setIsAtBottom] = useState(true)

  useEffect(() => {
    if (!scrollRef.current || !isAtBottom) return
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight
  }, [messages, isTyping, isAtBottom])

  return (
    <div
      ref={scrollRef}
      onScroll={handleScroll}
      className="min-h-0 flex-1 overflow-y-auto px-3 py-4 [scrollbar-gutter:stable]"
    >
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-5 pb-4">
        {messages.map((message) => (
          <div
            key={message.id}
            className={cn("flex w-full", message.role === "user" ? "justify-end" : "justify-start")}
          >
            {message.role === "assistant" ? <AssistantMessage ... /> : <UserMessage ... />}
          </div>
        ))}
        {isTyping ? <TypingIndicator /> : null}
      </div>
    </div>
  )
}
```

新增移动端输入栏：

```tsx
// web/frontend/src/features/mobile/mobile-chat-composer.tsx
export function MobileChatComposer({ input, attachments, fileInputRef, ...props }: MobileChatComposerProps) {
  const composingRef = useRef(false)

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    const nativeEvent = event.nativeEvent as Event & { isComposing?: boolean; keyCode?: number }
    if (composingRef.current || nativeEvent.isComposing || nativeEvent.keyCode === 229) return
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault()
      onSend()
    }
  }

  return (
    <div className="border-border/60 bg-background shrink-0 border-t px-3 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))]">
      <input ref={fileInputRef} type="file" accept={CHAT_IMAGE_ACCEPT} multiple className="hidden" onChange={onFileChange} />
      <div className="mx-auto flex max-w-3xl flex-col gap-2">
        {hasAvailableModels && <ModelSelector ... />}
        {attachments.length > 0 ? <div className="flex gap-2 overflow-x-auto pb-1">...</div> : null}
        <div className="border-border/70 bg-card flex items-end gap-2 rounded-lg border p-2 shadow-sm">
          <Button onClick={onAddImages} disabled={disabled} size="icon">
            <IconPhotoPlus className="size-5" />
          </Button>
          <TextareaAutosize
            value={input}
            onKeyDown={handleKeyDown}
            minRows={1}
            maxRows={5}
          />
          <Button onClick={onSend} disabled={!canSend} size="icon">
            <IconArrowUp className="size-5" />
          </Button>
        </div>
      </div>
    </div>
  )
}
```

新增移动端会话历史抽屉：

```tsx
// web/frontend/src/components/chat/mobile-session-history.tsx
export function MobileSessionHistory({ activeSessionId, onSwitchSession, onNewChat }: MobileSessionHistoryProps) {
  const {
    sessions,
    hasMore,
    loadError,
    loadErrorMessage,
    observerRef,
    loadSessions,
    handleDeleteSession,
  } = useSessionHistory({
    activeSessionId,
    onDeletedActiveSession: onNewChat,
  })

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetTrigger asChild>
        <Button variant="secondary" size="icon" aria-label={t("chat.history")}>
          <IconHistory className="size-5" />
        </Button>
      </SheetTrigger>
      <SheetContent side="bottom" className="flex max-h-[70vh] flex-col pb-[calc(1rem+env(safe-area-inset-bottom))]">
        ...
      </SheetContent>
    </Sheet>
  )
}
```

其他配套文件：

- `web/frontend/src/features/mobile/use-mobile-gateway-polling.ts`
- `web/frontend/src/routeTree.gen.ts`
- `web/frontend/src/components/chat/model-selector.tsx`

模型选择器为适配窄屏，把固定最大宽度改成百分比：

```tsx
// web/frontend/src/components/chat/model-selector.tsx
className="text-muted-foreground hover:text-foreground focus-visible:border-input h-8 max-w-[70%] min-w-0 bg-transparent shadow-none focus-visible:ring-0"
```

## 2. 移动端认证、初始化和 401 回跳

### 修改思路

移动端访问 `/mobile` 时，如果 launcher 未初始化或未登录，上游逻辑会跳转到 `/launcher-setup` 或 `/launcher-login`，登录成功后默认回首页 `/`。这会让手机用户登录后离开移动端页面。

当前分支新增安全 redirect 工具，只允许站内相对路径，并把 `/mobile` 的原始路径、query、hash 带到登录/初始化页。登录或初始化完成后返回原移动端目标。

### 具体修改代码

新增 redirect 工具：

```ts
// web/frontend/src/features/mobile/redirect.ts
export function getSafeRedirectTarget(
  value: string | null | undefined,
  fallback = DEFAULT_REDIRECT_FALLBACK,
): string {
  const target = value?.trim()
  if (!target) return fallback
  if (!target.startsWith("/") || target.startsWith("//")) return fallback
  if (hasControlCharacter(target)) return fallback

  try {
    const parsed = new URL(target, globalThis.location?.origin ?? "http://localhost")
    if (parsed.origin !== (globalThis.location?.origin ?? parsed.origin)) {
      return fallback
    }
    return `${parsed.pathname}${parsed.search}${parsed.hash}`
  } catch {
    return fallback
  }
}

export function buildLauncherAuthPath(
  pathname: "/launcher-login" | "/launcher-setup",
  redirectTarget: string,
): string {
  const safeRedirect = getSafeRedirectTarget(redirectTarget)
  if (safeRedirect === DEFAULT_REDIRECT_FALLBACK) return pathname
  return `${pathname}?redirect=${encodeURIComponent(safeRedirect)}`
}

export function isMobilePathname(pathname: string): boolean {
  return pathname === "/mobile" || pathname.startsWith("/mobile/")
}
```

根路由初始化/登录状态检查接入移动端 redirect：

```tsx
// web/frontend/src/routes/__root.tsx
const authRedirectTarget = isMobilePage ? getCurrentMobileRedirectTarget() : "/"

if (!s.initialized) {
  globalThis.location.assign(
    isMobilePage
      ? buildLauncherAuthPath("/launcher-setup", authRedirectTarget)
      : "/launcher-setup",
  )
} else if (!s.authenticated) {
  globalThis.location.assign(
    isMobilePage
      ? buildLauncherAuthPath("/launcher-login", authRedirectTarget)
      : "/launcher-login",
  )
}
```

前端 API 收到 401 时，移动端保持回跳目标：

```ts
// web/frontend/src/api/http.ts
if (res.status === 401) {
  if (typeof globalThis.location !== "undefined" && !isLauncherAuthPath()) {
    const pathname = globalThis.location.pathname || "/"
    globalThis.location.assign(
      isMobilePathname(pathname)
        ? buildLauncherAuthPath("/launcher-login", getCurrentMobileRedirectTarget())
        : "/launcher-login",
    )
  }
}
```

登录页读取 redirect，登录成功后返回目标：

```tsx
// web/frontend/src/routes/launcher-login.tsx
const redirectTarget = React.useMemo(() => getRedirectTargetFromLocation(), [])
const isMobileRedirect = React.useMemo(
  () => isMobilePathname(redirectTarget),
  [redirectTarget],
)
const setupPath = React.useMemo(
  () => buildLauncherAuthPath("/launcher-setup", redirectTarget),
  [redirectTarget],
)

if (result.ok) {
  globalThis.location.assign(redirectTarget)
  return
}
if (result.status === 409) {
  globalThis.location.assign(setupPath)
  return
}
```

初始化页初始化成功后返回带 redirect 的登录页：

```tsx
// web/frontend/src/routes/launcher-setup.tsx
const redirectTarget = React.useMemo(() => getRedirectTargetFromLocation(), [])
const loginPath = React.useMemo(
  () => buildLauncherAuthPath("/launcher-login", redirectTarget),
  [redirectTarget],
)

if (result.ok) {
  globalThis.location.assign(loginPath)
  return
}
```

后端未授权拦截对 `/mobile` 保留 request URI：

```go
// web/backend/middleware/launcher_dashboard_auth.go
func rejectLauncherDashboardAuth(w http.ResponseWriter, r *http.Request, canonicalPath string) {
	if strings.HasPrefix(canonicalPath, "/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		return
	}
	http.Redirect(w, r, launcherDashboardLoginRedirectPath(r, canonicalPath), http.StatusFound)
}

func launcherDashboardLoginRedirectPath(r *http.Request, canonicalPath string) string {
	if !isMobileDashboardPath(canonicalPath) {
		return "/launcher-login"
	}
	return "/launcher-login?redirect=" + url.QueryEscape(r.URL.RequestURI())
}

func isMobileDashboardPath(canonicalPath string) bool {
	return canonicalPath == "/mobile" || strings.HasPrefix(canonicalPath, "/mobile/")
}
```

## 3. 图片附件保存与非多模态模型兼容

### 修改思路

上游 `v0.3.1` 已有媒体引用解析能力，但移动端上传图片时会产生 `data:image/...` 内联数据。对于 DeepSeek 等非多模态模型，直接把图片 data URL 送进 provider 会报错。

当前分支的处理思路：

- 当前轮用户消息中的 `data:image/...` 不直接传给模型。
- 如果用户表达了“保存图片附件”的意图，后端先把图片保存到文件，并把模型输入改写成“图片已保存到这些路径”的文本提示。
- 如果只是普通图片输入，则把 data URL 转成临时图片文件，再注入 `[image:/path]` 路径标签，让模型通过工具读取，而不是直接接收 image input。
- 历史轮次里的 `data:` media 不再重放，避免第二轮继续把大块 base64 发给非视觉模型。
- 文件落点优先使用 workspace 下 `tmp/picoclaw-attachments`，显式 `/tmp/...` 目标只允许绝对路径、位于系统 temp 目录下，且必须是图片扩展名。

### 具体修改代码

`resolveMediaRefs` 增加 `workspaceDir` 参数，用于选择临时保存目录：

```go
// pkg/agent/agent_media.go
func resolveMediaRefs(
	messages []providers.Message,
	store media.MediaStore,
	maxSize int,
	currentTurnStart int,
	workspaceDir string,
) []providers.Message {
	currentTurnStart = normalizeCurrentTurnStart(messages, currentTurnStart)
	...
}
```

当前轮 data image 保存或转路径标签：

```go
// pkg/agent/agent_media.go
if strings.HasPrefix(ref, "data:image/") && attachmentSaveIntent {
	localPath, _, err := saveDataImageRef(ref, attachmentSaveTarget, workspaceDir, maxSize)
	if err != nil {
		logger.WarnCF("agent", "Failed to save inline image attachment", map[string]any{
			"path":  attachmentSaveTarget,
			"error": err.Error(),
		})
		resolved = append(resolved, ref)
		continue
	}
	savedAttachmentPaths = append(savedAttachmentPaths, localPath)
	continue
}

if strings.HasPrefix(ref, "data:image/") && m.Role == "user" && idx >= currentTurnStart {
	localPath, mime, err := saveDataImageRef(ref, "", workspaceDir, maxSize)
	if err != nil {
		logger.WarnCF("agent", "Failed to save inline image for current turn", map[string]any{
			"error": err.Error(),
		})
		resolved = append(resolved, ref)
		continue
	}
	pathTags = append(pathTags, buildPathTag(mime, localPath))
	continue
}
```

历史 data URL 不再重放：

```go
// pkg/agent/agent_media.go
if !strings.HasPrefix(ref, "media://") {
	if idx < currentTurnStart && strings.HasPrefix(ref, "data:") {
		continue
	}
	resolved = append(resolved, ref)
	continue
}
```

保存意图识别：

```go
// pkg/agent/agent_media.go
func isAttachmentSaveIntent(content string) bool {
	lower := strings.ToLower(content)
	if !containsAny(lower, []string{
		"save", "write", "copy", "store", "persist", "export",
		"保存", "存到", "另存", "写入", "复制", "导出",
	}) {
		return false
	}
	if !containsAny(lower, []string{
		"image", "photo", "picture", "attachment", "attached", "upload", "file",
		"图片", "图像", "照片", "附件", "附图", "截图", "上传", "文件",
	}) {
		return false
	}
	return true
}
```

只允许安全的显式保存目标：

```go
// pkg/agent/agent_media.go
func isAllowedInlineAttachmentSaveTarget(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	cleanPath := filepath.Clean(path)
	tmpDir := filepath.Clean(os.TempDir())
	rel, err := filepath.Rel(tmpDir, cleanPath)
	return err == nil && rel != "." && filepath.IsLocal(rel)
}

func hasImageFileExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp":
		return true
	default:
		return false
	}
}
```

解码和保存 data URL：

```go
// pkg/agent/agent_media.go
func saveDataImageRef(ref, targetPath, defaultDir string, maxSize int) (string, string, error) {
	comma := strings.IndexByte(ref, ',')
	if comma < 0 {
		return "", "", os.ErrInvalid
	}
	metadata := ref[:comma]
	payload := ref[comma+1:]
	if !strings.HasPrefix(metadata, "data:image/") || !strings.Contains(metadata, ";base64") {
		return "", "", os.ErrInvalid
	}
	if maxSize > 0 && base64.StdEncoding.DecodedLen(len(payload)) > maxSize {
		return "", "", os.ErrInvalid
	}

	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", "", err
	}
	if maxSize > 0 && len(data) > maxSize {
		return "", "", os.ErrInvalid
	}

	mime := strings.TrimPrefix(metadata, "data:")
	if semicolon := strings.IndexByte(mime, ';'); semicolon >= 0 {
		mime = mime[:semicolon]
	}
	if kind, err := filetype.Match(data); err == nil && kind != filetype.Unknown {
		mime = kind.MIME.Value
	}
	if !strings.HasPrefix(mime, "image/") {
		return "", "", os.ErrInvalid
	}

	cleanTarget := filepath.Clean(targetPath)
	if cleanTarget == "." || cleanTarget == "" {
		if defaultDir != "" {
			cleanTarget, err = createAttachmentPathInDir(defaultDir, mime)
		} else {
			cleanTarget, err = createDefaultInlineAttachmentPath(mime)
		}
		if err != nil {
			return "", "", err
		}
	} else if !isAllowedInlineAttachmentSaveTarget(cleanTarget) || !hasImageFileExtension(cleanTarget) {
		cleanTarget, err = createDefaultInlineAttachmentPath(mime)
		if err != nil {
			return "", "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o700); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(cleanTarget, data, 0o600); err != nil {
		return "", "", err
	}
	return cleanTarget, mime, nil
}
```

保存成功后给模型的提示：

```go
// pkg/agent/agent_media.go
func savedAttachmentNotice(paths []string) string {
	var b strings.Builder
	b.WriteString("The user asked to save image attachment(s). The backend has already saved them to:")
	for _, path := range paths {
		b.WriteString("\n- ")
		b.WriteString(path)
	}
	b.WriteString("\nReply that the image attachment has been saved. Do not call load_image, write_file, edit_file, exec, or other tools. Do not inspect or analyze the image contents.")
	return b.String()
}
```

调用点统一传入 workspace：

```go
// pkg/agent/pipeline_setup.go
messages = resolveMediaRefs(messages, p.MediaStore, maxMediaSize, currentTurnStart, ts.agent.Workspace)

// pkg/agent/pipeline_llm.go
exec.messages = resolveMediaRefs(exec.messages, p.MediaStore, maxMediaSize, exec.currentTurnStart, ts.agent.Workspace)

// pkg/agent/agent_init.go
return resolveMediaRefs(msgs, al.mediaStore, cfg.Agents.Defaults.GetMaxMediaSize(), 0, "")
```

## 4. 移动端反向代理部署示例

### 修改思路

移动端 H5 通常通过公网域名或内网反代访问。为了避免直接把整个桌面控制台暴露出去，当前分支新增 nginx 示例，只放行移动端需要的路径：`/mobile`、静态资源、登录/初始化、网关状态、pico websocket、媒体和会话读取 API。

### 具体修改代码

新增文件：

```nginx
# docs/wendi-mobile-nginx.example.conf
upstream picoclaw_launcher {
    server 127.0.0.1:18800;
}

server {
    listen 443 ssl http2;
    server_name picoclaw-mobile.company.com;

    location = / {
        return 404;
    }

    location = /mobile {
        limit_except GET { deny all; }
        proxy_pass http://picoclaw_launcher;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location = /pico/ws {
        limit_except GET { deny all; }
        proxy_pass http://picoclaw_launcher;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 3600s;
    }

    location / {
        return 404;
    }
}
```

完整文件还包括：

- `/mobile/`
- `/assets/`
- favicon / manifest
- `/launcher-login`
- `/launcher-setup`
- `/api/auth/status`
- `/api/auth/login`
- `/api/auth/setup`
- `/api/gateway/status`
- `/api/pico/info`
- `/api/sessions/{id}`
- `/pico/media/`

## 5. 移动端文案和路由生成文件

### 修改思路

移动端新增了状态、占位符、空状态和禁用提示，需要补充英文和中文 i18n。TanStack Router 的生成文件也需要加入 `/mobile` 路由声明。

### 具体修改代码

新增英文文案：

```json
// web/frontend/src/i18n/locales/en.json
"mobile": {
  "placeholder": "Message DiAgent...",
  "emptyTitle": "DiAgent",
  "emptyDescription": "Start a conversation from DiAgent.",
  "gatewayHint": "DiAgent is not ready. Start the gateway from the desktop dashboard.",
  "gateway": {
    "running": "Gateway running",
    "starting": "Gateway starting...",
    "restarting": "Gateway restarting...",
    "stopping": "Gateway stopping...",
    "stopped": "Gateway not running",
    "error": "Gateway error",
    "unknown": "Checking gateway..."
  },
  "connection": {
    "connected": "Connected",
    "connecting": "Connecting...",
    "disconnected": "Disconnected",
    "error": "Connection error"
  }
}
```

新增中文文案：

```json
// web/frontend/src/i18n/locales/zh.json
"mobile": {
  "placeholder": "向 DiAgent 发送消息...",
  "emptyTitle": "DiAgent",
  "emptyDescription": "从 DiAgent 开始一段对话。",
  "gatewayHint": "DiAgent 尚未就绪，请先在桌面控制台启动网关。",
  "gateway": {
    "running": "网关运行中",
    "starting": "网关启动中...",
    "restarting": "网关重启中...",
    "stopping": "网关停止中...",
    "stopped": "网关未运行",
    "error": "网关异常",
    "unknown": "正在检测网关..."
  }
}
```

路由生成文件加入 `/mobile`：

```ts
// web/frontend/src/routeTree.gen.ts
import { Route as MobileRouteImport } from './routes/mobile'

const MobileRoute = MobileRouteImport.update({
  id: '/mobile',
  path: '/mobile',
  getParentRoute: () => rootRouteImport,
} as any)

export interface FileRoutesByFullPath {
  '/mobile': typeof MobileRoute
}
```

## 6. 测试覆盖

### 修改思路

本分支涉及登录回跳、移动端图片附件保存、历史 data URL 清理等容易回归的逻辑，因此新增后端单元测试，验证：

- `/mobile` 未授权时 redirect 保留目标。
- 非移动端未授权时不把 query 带入登录页，避免敏感 token 泄漏。
- 移动端上传的 data image 可以保存到目标路径。
- 保存图片时不会把 media 或 `[image:]` 标签交给纯文本 provider。
- 没有显式目标时保存到 workspace 临时目录。
- 第二轮对话不会重放第一轮的 data URL。

### 具体修改代码

后端鉴权测试：

```go
// web/backend/middleware/launcher_dashboard_auth_test.go
func TestLauncherDashboardAuth_MobileRedirectPreservesTarget(t *testing.T) {
	cfg := LauncherDashboardAuthConfig{ExpectedCookie: "deadbeef"}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next handler should not run without session cookie")
	})
	h := LauncherDashboardAuth(cfg, next)

	for _, tc := range []struct {
		name     string
		path     string
		location string
	}{
		{name: "mobile root", path: "/mobile", location: "/launcher-login?redirect=%2Fmobile"},
		{name: "mobile query", path: "/mobile?foo=bar", location: "/launcher-login?redirect=%2Fmobile%3Ffoo%3Dbar"},
		{name: "mobile child", path: "/mobile/foo", location: "/launcher-login?redirect=%2Fmobile%2Ffoo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
			}
			if got := rec.Header().Get("Location"); got != tc.location {
				t.Fatalf("Location = %q, want %q", got, tc.location)
			}
		})
	}
}
```

非移动路径不保留 query：

```go
// web/backend/middleware/launcher_dashboard_auth_test.go
func TestLauncherDashboardAuth_NonMobileRedirectDoesNotPreserveQuery(t *testing.T) {
	cfg := LauncherDashboardAuthConfig{ExpectedCookie: "deadbeef"}
	h := LauncherDashboardAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next handler should not run without session cookie")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/models?token=secret", nil)
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Location"); got != "/launcher-login" {
		t.Fatalf("Location = %q, want %q", got, "/launcher-login")
	}
}
```

图片保存 provider 测试桩：

```go
// pkg/agent/agent_test.go
type inlineAttachmentSaveProvider struct {
	targetPath string
	pathPrefix string
	calls      int
	mediaSeen  []bool
	pathSeen   []bool
	seenPath   string
}

func (p *inlineAttachmentSaveProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	p.calls++
	hasMedia := false
	hasPath := false
	for _, msg := range messages {
		if strings.Contains(msg.Content, "[image:") || strings.Contains(msg.Content, "[file:") {
			return nil, fmt.Errorf("text provider unexpectedly received media path tag in save task: %q", msg.Content)
		}
		for _, ref := range msg.Media {
			if strings.TrimSpace(ref) != "" {
				hasMedia = true
				break
			}
		}
	}
	p.mediaSeen = append(p.mediaSeen, hasMedia)
	p.pathSeen = append(p.pathSeen, hasPath)

	if hasMedia {
		return nil, fmt.Errorf("text provider unexpectedly received image media")
	}
	if !hasPath {
		return nil, fmt.Errorf("text provider did not receive saved image path tag")
	}
	return &providers.LLMResponse{Content: "saved"}, nil
}
```

关键测试函数：

```go
// pkg/agent/agent_test.go
func TestAgentLoop_InlineImageAttachmentSaveTargetBypassesVisionInput(t *testing.T) { ... }
func TestAgentLoop_InlineImageAttachmentSaveAttachedImageToTmpPath(t *testing.T) { ... }
func TestAgentLoop_InlineImageAttachmentSaveIntentWithoutTargetUsesTempPath(t *testing.T) { ... }
func TestAgentLoop_HistoricalInlineDataURLsAreNotReplayedToProvider(t *testing.T) { ... }
```

## 7. 运行时产物和过程文档

### 修改思路

分支中还包含运行 launcher/agent 后生成的本地状态、日志、技能目录，以及一个移动端方案文档。这些文件相对上游也属于 diff，但它们不是核心业务代码。它们应被单独看待：如果目标是代码合并，`.runtime` 大概率应移出提交或加入忽略策略；如果目标是复现当前运行状态，则这些文件记录了当时的本地 workspace 和日志。

### 具体修改文件

过程文档：

- `2026-06-26-picoclaw-wendi-mobile-h5-final-plan-zh`

保留的运行时产物：

- `.runtime/picoclaw-home/launcher-auth.db`
- `.runtime/picoclaw-home/logs/gateway.log`
- `.runtime/picoclaw-home/logs/gateway_panic.log`
- `.runtime/picoclaw-home/logs/launcher.log`
- `.runtime/picoclaw-home/logs/launcher_panic.log`
- `.runtime/picoclaw-home/workspace/AGENT.md`
- `.runtime/picoclaw-home/workspace/HEARTBEAT.md`
- `.runtime/picoclaw-home/workspace/SOUL.md`
- `.runtime/picoclaw-home/workspace/USER.md`
- `.runtime/picoclaw-home/workspace/cron/jobs.json`
- `.runtime/picoclaw-home/workspace/heartbeat.log`
- `.runtime/picoclaw-home/workspace/memory/MEMORY.md`
- `.runtime/picoclaw-home/workspace/skills/*`
- `.runtime/picoclaw-home/workspace/state/state.json`

已经在后续提交里删除的中间验证产物：

- `android-ui-screenshots/*`
- `mobile-ui-screenshots/*`
- `mobile-send-screenshots/*`
- `mobile-backend-save-retest-report/*`
- `mobile-multiturn-attachment-screenshots/*`
- `.runtime/picoclaw-home/workspace/tmp/picoclaw-attachments/*`

## 8. 文件级差异清单

业务代码和配置：

- `docs/wendi-mobile-nginx.example.conf`
- `pkg/agent/agent_init.go`
- `pkg/agent/agent_media.go`
- `pkg/agent/agent_test.go`
- `pkg/agent/pipeline_llm.go`
- `pkg/agent/pipeline_setup.go`
- `pkg/agent/turn_coord.go`
- `web/backend/middleware/launcher_dashboard_auth.go`
- `web/backend/middleware/launcher_dashboard_auth_test.go`
- `web/frontend/src/api/http.ts`
- `web/frontend/src/components/chat/mobile-session-history.tsx`
- `web/frontend/src/components/chat/model-selector.tsx`
- `web/frontend/src/features/mobile/mobile-chat-composer.tsx`
- `web/frontend/src/features/mobile/mobile-chat-page.tsx`
- `web/frontend/src/features/mobile/mobile-message-list.tsx`
- `web/frontend/src/features/mobile/mobile-shell.tsx`
- `web/frontend/src/features/mobile/redirect.ts`
- `web/frontend/src/features/mobile/use-mobile-gateway-polling.ts`
- `web/frontend/src/i18n/locales/en.json`
- `web/frontend/src/i18n/locales/zh.json`
- `web/frontend/src/routeTree.gen.ts`
- `web/frontend/src/routes/__root.tsx`
- `web/frontend/src/routes/launcher-login.tsx`
- `web/frontend/src/routes/launcher-setup.tsx`
- `web/frontend/src/routes/mobile.tsx`

非核心业务代码/运行时产物：

- `.runtime/picoclaw-home/**`
- `2026-06-26-picoclaw-wendi-mobile-h5-final-plan-zh`

## 9. 总结

相对上游 `upstream/main / v0.3.1`，当前分支的核心价值是：

- 增加一个适合手机访问的 `/mobile` DiAgent 聊天入口。
- 让移动端认证流程能安全返回原页面。
- 让移动端图片附件通过文件路径进入 Agent，避免非多模态模型直接处理 image data URL 报错。
- 提供更收敛的 nginx 反代示例，便于只暴露移动端所需接口。
- 用单元测试覆盖 redirect 和图片附件处理的关键边界。

