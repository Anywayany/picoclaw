# 模型切换后自动生效修复指南

本文用于指导能力较弱的模型在同一类代码库中完成“用户切换模型后不需要手动重启服务”的修复。按顺序执行，不要跳步。

## 目标

当前问题是：用户在移动端或 Web UI 中切换默认模型后，配置文件已经保存，但运行中的 gateway 仍然使用启动时加载的旧配置。用户必须手动重启服务，新的默认模型才会生效。

修复目标是：

1. 用户切换默认模型后，后端自动通知运行中的 gateway reload。
2. reload 成功后，接口响应明确返回 `applied: true` 和 `restart_required: false`。
3. reload 失败时保留原来的重启提示，不要假装已经生效。
4. 前端优先使用接口返回的应用结果展示 toast。
5. 补充测试，保证默认模型切换会触发 gateway reload。

## 判断思路

先确认系统里有两个不同职责：

1. `web/backend/api` 是 launcher 后端，负责处理 UI 的模型新增、编辑、设为默认等 API。
2. `pkg/gateway` 是实际运行模型服务的 gateway，它在启动时加载配置。

模型切换接口只保存配置，不等于运行中的 gateway 已经重新读取配置。因此修复点不是只改前端提示，而是要让 launcher 在保存配置后调用 gateway 的 `/reload`。

关键设计判断：

1. 如果 gateway 正在运行，就读 `.picoclaw.pid`，拿到 host、port、token。
2. 使用 token 调用 `POST /reload`。
3. `/reload` 必须等真实 reload 完成后再返回，否则 UI 会误报“已生效”。
4. 如果 gateway 不在运行，可以返回 `applied: false` 且 `restart_required: false`，因为没有运行中进程需要 reload。
5. 如果 gateway 在运行但 reload 失败，返回 `restart_required: true`，让前端继续提示用户重启。

## 修改计划

按下面顺序修改：

1. 修改 `pkg/gateway/gateway.go`，让 `/reload` 从“只排队”改成“排队并等待结果”。
2. 修改 `web/backend/api/gateway.go`，增加 `applyGatewayConfigChange`，负责调用运行中 gateway 的 `/reload`。
3. 修改 `web/backend/api/models.go`：
   - `handleSetDefaultModel` 保存配置后调用 `applyGatewayConfigChange`。
   - `handleUpdateModel` 只有在编辑影响当前默认模型时调用 `applyGatewayConfigChange`。
4. 修改前端模型 API 类型，增加 `applied`、`apply_method`、`apply_error`、`restart_required`。
5. 修改前端 toast 逻辑，优先使用后端返回的应用状态。
6. 增加后端测试，验证“设为默认模型”会调用 gateway reload。
7. 运行 Go 测试、前端构建、launcher 构建。
8. 重启本地服务并验证 `/mobile` 和登录页能访问。

## 后端关键代码

### 1. 让 gateway reload 同步返回结果

文件：`pkg/gateway/gateway.go`

把手动 reload channel 从只传信号改成传结果 channel：

```go
type services struct {
	DeviceService    *devices.Service
	HealthServer     *health.Server
	VoiceAgentCancel context.CancelFunc
	manualReloadChan chan chan error
	reloading        atomic.Bool
	authToken        string
}
```

创建 channel 时使用 `chan chan error`，并让触发函数等待 reload 结果：

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

manualReloadChan := make(chan chan error, 1)
runningServices.manualReloadChan = manualReloadChan
reloadTrigger := func() error {
	if !runningServices.reloading.CompareAndSwap(false, true) {
		return fmt.Errorf("reload already in progress")
	}
	resultCh := make(chan error, 1)
	select {
	case manualReloadChan <- resultCh:
	case <-ctx.Done():
		runningServices.reloading.Store(false)
		return ctx.Err()
	default:
		runningServices.reloading.Store(false)
		return fmt.Errorf("reload already queued")
	}
	select {
	case err := <-resultCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
runningServices.HealthServer.SetReloadFunc(reloadTrigger)
agentLoop.SetReloadFunc(reloadTrigger)
```

在主循环里接收 `resultCh`，reload 完成后把错误写回去：

```go
case resultCh := <-manualReloadChan:
	logger.Info("Manual reload triggered via /reload endpoint")
	newCfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Errorf("Error loading config for manual reload: %v", err)
		runningServices.reloading.Store(false)
		resultCh <- fmt.Errorf("error loading config for manual reload: %w", err)
		continue
	}
	if err = newCfg.ValidateModelList(); err != nil {
		logger.Errorf("Config validation failed: %v", err)
		runningServices.reloading.Store(false)
		resultCh <- fmt.Errorf("config validation failed: %w", err)
		continue
	}
	err = executeReload(ctx, agentLoop, newCfg, &provider, runningServices, msgBus, allowEmptyStartup, debug)
	if err != nil {
		logger.Errorf("Manual reload failed: %v", err)
	} else {
		logger.Info("Manual reload completed successfully")
	}
	resultCh <- err
```

注意：不要在 HTTP handler 中直接执行 `executeReload`。gateway 的 reload 已经有自己的主循环和状态管理，应该通过现有 reload 机制进入。

### 2. 增加 launcher 侧配置应用 helper

文件：`web/backend/api/gateway.go`

增加一个可在测试中替换的 HTTP 调用函数：

```go
var gatewayReloadDo = func(req *http.Request) (*http.Response, error) {
	client := http.Client{Timeout: 45 * time.Second}
	return client.Do(req)
}
```

增加统一响应结构：

```go
type gatewayConfigApplyResult struct {
	Applied         bool   `json:"applied"`
	ApplyMethod     string `json:"apply_method,omitempty"`
	ApplyError      string `json:"apply_error,omitempty"`
	RestartRequired bool   `json:"restart_required"`
}

func (r gatewayConfigApplyResult) appendTo(data map[string]any) {
	data["applied"] = r.Applied
	data["restart_required"] = r.RestartRequired
	if r.ApplyMethod != "" {
		data["apply_method"] = r.ApplyMethod
	}
	if r.ApplyError != "" {
		data["apply_error"] = r.ApplyError
	}
}
```

增加实际应用配置的方法：

```go
func (h *Handler) applyGatewayConfigChange(cfg *config.Config) gatewayConfigApplyResult {
	result := gatewayConfigApplyResult{ApplyMethod: "none"}
	if cfg == nil {
		result.RestartRequired = true
		result.ApplyError = "config unavailable"
		return result
	}

	pidData := h.sanitizeGatewayPidData(ppid.ReadPidFileWithCheck(globalConfigDir()), cfg)
	if pidData == nil {
		return result
	}
	if strings.TrimSpace(pidData.Token) == "" {
		result.RestartRequired = true
		result.ApplyError = "gateway reload token missing"
		return result
	}

	port := pidData.Port
	if port == 0 {
		port = cfg.Gateway.Port
	}
	if port == 0 {
		port = 18790
	}
	host := gatewayProbeHost(pidData.Host)
	if host == "" {
		host = gatewayProbeHost(h.effectiveGatewayBindHost(cfg))
	}
	reloadURL := "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/reload"

	req, err := http.NewRequest(http.MethodPost, reloadURL, nil)
	if err != nil {
		result.RestartRequired = true
		result.ApplyError = err.Error()
		return result
	}
	req.Header.Set("Authorization", "Bearer "+pidData.Token)

	resp, err := gatewayReloadDo(req)
	if err != nil {
		result.RestartRequired = true
		result.ApplyMethod = "reload"
		result.ApplyError = err.Error()
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		result.RestartRequired = true
		result.ApplyMethod = "reload"
		result.ApplyError = strings.TrimSpace(string(body))
		if result.ApplyError == "" {
			result.ApplyError = resp.Status
		}
		return result
	}

	gateway.mu.Lock()
	gateway.pidData = pidData
	gateway.bootDefaultModel = strings.TrimSpace(cfg.Agents.Defaults.GetModelName())
	gateway.bootConfigSignature = computeConfigSignature(cfg)
	setGatewayRuntimeStatusLocked("running")
	gateway.mu.Unlock()

	result.Applied = true
	result.ApplyMethod = "reload"
	return result
}
```

需要的 import 通常包括：

```go
import (
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)
```

不要泄露 token。token 只用于请求头，不要写日志，不要返回给前端。

### 3. 保存默认模型后自动应用

文件：`web/backend/api/models.go`

在 `handleSetDefaultModel` 保存配置成功后调用 helper：

```go
applyResult := h.applyGatewayConfigChange(cfg)
response := map[string]any{
	"status":        "ok",
	"default_model": req.ModelName,
}
applyResult.appendTo(response)

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(response)
```

### 4. 编辑当前默认模型时自动应用

文件：`web/backend/api/models.go`

保存旧默认模型名，并判断本次编辑是否影响当前默认模型：

```go
oldDefaultModelName := strings.TrimSpace(cfg.Agents.Defaults.GetModelName())
if oldDefaultModelName == cfg.ModelList[idx].ModelName &&
	!defaultModelAllowedForModelConfig(&mc.ModelConfig) {
	cfg.Agents.Defaults.ModelName = ""
}

affectsDefaultModel := oldDefaultModelName != "" &&
	(oldDefaultModelName == cfg.ModelList[idx].ModelName || oldDefaultModelName == mc.ModelName)
```

保存配置后，只有影响默认模型时才调用 reload：

```go
response := map[string]any{"status": "ok"}
if affectsDefaultModel {
	applyResult := h.applyGatewayConfigChange(cfg)
	applyResult.appendTo(response)
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(response)
```

不要在编辑任意非默认模型时 reload gateway。这样会避免无意义 reload。

## 前端关键代码

### 1. 扩展模型 API 响应类型

文件：`web/frontend/src/api/models.ts`

```ts
interface ModelActionResponse {
  status: string
  index?: number
  default_model?: string
  applied?: boolean
  apply_method?: string
  apply_error?: string
  restart_required?: boolean
}
```

### 2. toast 支持“已自动应用”描述

文件：`web/frontend/src/lib/restart-required.ts`

```ts
export function showSaveSuccessOrRestartToast(
  t: TFunction,
  savedMessage: string,
  name: string,
  restartRequired: boolean,
  applied = false,
) {
  if (restartRequired) {
    showRestartRequiredToast(t, name)
    return
  }
  toast.success(savedMessage, {
    description: applied ? t("common.appliedDesc") : undefined,
  })
}
```

### 3. 切换默认模型时使用接口返回结果

文件：`web/frontend/src/components/models/models-page.tsx`

```ts
const result = await setDefaultModel(model.model_name)
await fetchModels()
const gateway = await refreshGatewayState({ force: true })
showSaveSuccessOrRestartToast(
  t,
  t("models.defaultChangeSuccess"),
  model.model_name,
  result.restart_required ?? gateway?.restartRequired === true,
  result.applied === true,
)
```

### 4. 新增模型并设为默认时使用接口返回结果

文件：`web/frontend/src/components/models/add-model-sheet.tsx`

```ts
const defaultResult = setAsDefault
  ? await setDefaultModel(modelName)
  : undefined
const gateway = await refreshGatewayState({ force: true })
showSaveSuccessOrRestartToast(
  t,
  t("models.add.saveSuccess"),
  modelName,
  defaultResult?.restart_required ?? gateway?.restartRequired === true,
  defaultResult?.applied === true,
)
```

### 5. 编辑模型时合并 update 和 default 结果

文件：`web/frontend/src/components/models/edit-model-sheet.tsx`

```ts
const updateResult = await updateModel(model.index, {
  // 原有字段保持不变
})

const defaultResult =
  setAsDefault && !model.is_default
    ? await setDefaultModel(model.model_name)
    : undefined
const applyResult = defaultResult ?? updateResult

const gateway = await refreshGatewayState({ force: true })
showSaveSuccessOrRestartToast(
  t,
  t("models.edit.saveSuccess"),
  model.model_name,
  applyResult.restart_required ?? gateway?.restartRequired === true,
  applyResult.applied === true,
)
```

### 6. 增加 i18n 文案

文件：

```text
web/frontend/src/i18n/locales/en.json
web/frontend/src/i18n/locales/zh.json
web/frontend/src/i18n/locales/cs.json
web/frontend/src/i18n/locales/bn-in.json
web/frontend/src/i18n/locales/pt-br.json
```

新增 key：

```json
{
  "common": {
    "appliedDesc": "The change has been applied to the running gateway."
  }
}
```

中文：

```json
{
  "common": {
    "appliedDesc": "变更已自动应用到正在运行的服务。"
  }
}
```

如果不会翻译其他语言，可以先使用英文兜底，保证 key 存在。

## 测试代码

文件：`web/backend/api/models_test.go`

新增测试思路：

1. 创建临时配置。
2. 准备两个模型：`first-model` 和 `second-model`。
3. 当前默认模型设为 `first-model`。
4. 写入假的 `.picoclaw.pid`，包含当前进程 pid、host、port、reload token。
5. 替换 `gatewayProcessMatcher`，让测试认为 gateway 正在运行。
6. 替换 `gatewayReloadDo`，拦截 HTTP 请求并检查：
   - method 是 `POST`
   - path 是 `/reload`
   - Authorization 是 `Bearer reload-token`
7. 调用 `/api/models/default` 切到 `second-model`。
8. 断言响应里：
   - `applied == true`
   - `restart_required == false`
   - `apply_method == "reload"`
9. 断言 launcher 记录的 `gateway.bootDefaultModel` 更新为 `second-model`。

关键测试代码：

```go
func TestHandleSetDefaultModel_AppliesRunningGatewayConfig(t *testing.T) {
	configPath, cleanup := setupOAuthTestEnv(t)
	defer cleanup()

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	cfg.ModelList = []*config.ModelConfig{
		{ModelName: "first-model", Provider: "openai", Model: "gpt-4o"},
		{ModelName: "second-model", Provider: "openai", Model: "gpt-4o-mini"},
	}
	cfg.Agents.Defaults.ModelName = "first-model"
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	home := os.Getenv("PICOCLAW_HOME")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(home, ".picoclaw.pid"),
		[]byte(fmt.Sprintf(`{"pid":%d,"token":"reload-token","version":"test","port":18790,"host":"127.0.0.1"}`, os.Getpid())),
		0o600,
	); err != nil {
		t.Fatalf("WriteFile(pid) error = %v", err)
	}

	origGatewayReloadDo := gatewayReloadDo
	origGatewayProcessMatcher := gatewayProcessMatcher
	t.Cleanup(func() {
		gatewayReloadDo = origGatewayReloadDo
		gatewayProcessMatcher = origGatewayProcessMatcher
	})

	reloadCalled := false
	gatewayProcessMatcher = func(pid int) (bool, bool) {
		if pid != os.Getpid() {
			t.Fatalf("pid = %d, want current pid", pid)
		}
		return true, true
	}
	gatewayReloadDo = func(req *http.Request) (*http.Response, error) {
		reloadCalled = true
		if req.Method != http.MethodPost {
			t.Fatalf("reload method = %s, want POST", req.Method)
		}
		if req.URL.Path != "/reload" {
			t.Fatalf("reload path = %s, want /reload", req.URL.Path)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer reload-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"status":"reload completed"}`)),
			Header:     make(http.Header),
		}, nil
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/models/default", bytes.NewBufferString(`{
		"model_name": "second-model"
	}`))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !reloadCalled {
		t.Fatal("expected gateway reload to be called")
	}
}
```

实际测试里还应该恢复 `gateway` 全局状态，包括 `bootDefaultModel`、`bootConfigSignature`、`pidData` 和 `runtimeStatus`，避免污染其他测试。

## 验证命令

格式化 Go：

```bash
gofmt -w pkg/gateway/gateway.go web/backend/api/gateway.go web/backend/api/models.go web/backend/api/models_test.go
```

格式化前端：

```bash
pnpm exec prettier --write \
  web/frontend/src/api/models.ts \
  web/frontend/src/components/models/models-page.tsx \
  web/frontend/src/components/models/add-model-sheet.tsx \
  web/frontend/src/components/models/edit-model-sheet.tsx \
  web/frontend/src/lib/restart-required.ts \
  web/frontend/src/i18n/locales/en.json \
  web/frontend/src/i18n/locales/zh.json \
  web/frontend/src/i18n/locales/cs.json \
  web/frontend/src/i18n/locales/bn-in.json \
  web/frontend/src/i18n/locales/pt-br.json
```

运行测试：

```bash
go test ./web/backend/api ./pkg/health
go test -tags goolm,stdjson ./pkg/gateway
```

前端构建：

```bash
cd web/frontend
pnpm build
```

launcher 构建：

```bash
make -C web build
```

如果不带 tags 的 `go test ./pkg/gateway` 报 `olm/olm.h` 缺失，不要把它当成这次修复失败。这个仓库在当前环境使用 `goolm,stdjson` tags 可以绕过系统 libolm 依赖。

## 本地服务验证

构建完成后重启 launcher。示例：

```bash
PICOCLAW_HOME=/workspaces/picoclaw/.runtime/picoclaw-home \
PICOCLAW_BINARY=/workspaces/picoclaw/build/picoclaw \
/workspaces/picoclaw/web/build/picoclaw-launcher \
  -no-browser \
  -host 127.0.0.1 \
  -port 18900 \
  -debug
```

检查移动端入口：

```bash
curl -I http://127.0.0.1:18900/mobile
curl -I 'http://127.0.0.1:18900/launcher-login?redirect=%2Fmobile'
```

期望结果：

1. `/mobile` 未登录时返回 302 到 `/launcher-login?redirect=%2Fmobile`。
2. 登录页返回 200。
3. UI 中切换默认模型后，不再要求用户手动重启。
4. 如果 gateway reload 失败，UI 仍然提示需要重启。

## 容易犯错的点

1. 只改前端提示是不够的。必须让运行中的 gateway reload。
2. `/reload` 不能只返回“已排队”，否则 UI 会误判。
3. 不要把 reload token 暴露给前端或日志。
4. 不要编辑非默认模型时也 reload，除非它影响当前默认模型。
5. 保存配置失败时不能调用 reload。
6. reload 失败时不能返回 `applied: true`。
7. 测试必须恢复全局变量和全局 gateway 状态。
8. 多语言文案 key 必须在所有已启用 locale 中存在，否则构建或运行时可能缺 key。

## 最终行为

完成后，用户切换默认模型的链路应为：

```text
前端 setDefaultModel
  -> launcher handleSetDefaultModel
  -> 保存 config.json
  -> launcher 读取 pid 文件并带 token 调用 gateway POST /reload
  -> gateway 同步执行 reload 并返回结果
  -> launcher 返回 applied/restart_required
  -> 前端显示“已保存并自动应用”或“需要重启”
```

这就是本次修复的核心思想：保存配置只是持久化，reload 才是让运行中服务真正使用新模型。
