# 当前版本相对 picoclaw-v0.2.9 的变更说明

生成时间：2026-07-01  
比较基线：`v0.2.9`  
当前版本：`nightly-4-g6db0e9fc`，当前分支 `wendi-mobile-h5`  
比较命令：`git diff v0.2.9..HEAD`

## 总览

当前版本相比最开始未修改的 `picoclaw-v0.2.9`，累计变更规模如下：

- 变更文件：289 个
- 新增行数：约 21,503 行
- 删除行数：约 1,684 行
- 主要来源：上游 `v0.3.0`、`v0.3.1/nightly` 的累计更新，加上当前分支额外 4 个 `wendi-mobile-h5` 移动端定制提交
- 当前工作区状态：生成本文档前工作区干净

需要特别区分两类变动：

- 上游累计变更：从 `v0.2.9` 到 `v0.3.1/nightly` 的功能、修复、依赖、文档和测试更新。
- 当前分支定制：`wendi-mobile-h5` 分支额外加入的移动端 H5/DiAgent 页面、移动端访问链路、图片发送处理，以及相关 nginx 配置和 UI 调整。

## 1. 移动端 H5/DiAgent 访问体验

目的：为手机浏览器提供一个轻量、可直接访问的聊天入口，并让未登录、未初始化、网关异常等场景在移动端有正确的跳转和展示。

主要变更：

- 新增 `/mobile` 前端路由：
  - `web/frontend/src/routes/mobile.tsx`
  - `web/frontend/src/features/mobile/mobile-chat-page.tsx`
  - `web/frontend/src/features/mobile/mobile-chat-composer.tsx`
  - `web/frontend/src/features/mobile/mobile-message-list.tsx`
  - `web/frontend/src/features/mobile/mobile-shell.tsx`
  - `web/frontend/src/features/mobile/use-mobile-gateway-polling.ts`
- 移动端页面顶部品牌改为 `DiAgent`，并展示网关/WebSocket 状态。
- 新增移动端会话历史抽屉：
  - `web/frontend/src/components/chat/mobile-session-history.tsx`
  - 支持打开历史、切换会话、删除会话、无限加载和空/错误状态。
- 移动端输入区重新设计：
  - 使用紧凑底部输入栏。
  - 支持模型选择。
  - 支持图片选择、图片预览、移除附件。
  - 支持安全区 `env(safe-area-inset-bottom)`，适配手机底部手势区域。
- 移动端消息列表单独实现：
  - 用户消息靠右，助手消息靠左。
  - 使用移动端滚动容器。
  - 保留助手详情显示逻辑。
  - 空会话展示移动端专用文案。
- 新增移动端 redirect 工具：
  - `web/frontend/src/features/mobile/redirect.ts`
  - 只允许站内相对路径，过滤 `//`、控制字符和跨 origin 目标。
  - 登录/初始化完成后可以回到原始 `/mobile` 页面。
- 根路由根据路径切换桌面 `AppLayout` 和移动端 `MobileShell`：
  - `web/frontend/src/routes/__root.tsx`
- 登录和初始化页面为移动端 redirect 做了布局适配：
  - `web/frontend/src/routes/launcher-login.tsx`
  - `web/frontend/src/routes/launcher-setup.tsx`
  - 移动端回跳时减少桌面卡片式布局感，登录成功后返回 `/mobile`。
- API 401 处理区分桌面和移动端：
  - `web/frontend/src/api/http.ts`
  - 移动端收到 401 时跳到 `/launcher-login?redirect=/mobile...`，避免登录后丢失入口。
- 新增移动端 nginx 示例：
  - `docs/wendi-mobile-nginx.example.conf`
  - 仅暴露 `/mobile`、静态资源、登录/初始化、必要 API、`/pico/ws` 和 `/pico/media/`。
  - 其他路径默认返回 404，降低移动端反代暴露面。

## 2. 图片输入、多模态路由与非视觉模型兼容

目的：让 Web/移动端能发送图片，同时避免非多模态模型直接收到图片后报错；在需要时把图片保存成文件路径供工具读取。

主要变更：

- 新增图片输入通用工具：
  - `web/frontend/src/features/chat/image-input.ts`
  - 支持 jpeg/png/gif/webp/bmp。
  - 限制单图最大 7 MB。
  - 支持通过 MIME 或扩展名识别图片。
  - 生成 `ChatAttachment`，以 data URL 形式传给聊天协议。
- 桌面聊天输入区和移动端输入区都接入图片附件：
  - `web/frontend/src/components/chat/chat-composer.tsx`
  - `web/frontend/src/components/chat/chat-page.tsx`
  - `web/frontend/src/features/mobile/mobile-chat-composer.tsx`
- 后端/Agent 媒体处理增强：
  - `pkg/agent/agent_media.go`
  - `pkg/agent/llm_media.go`
  - `pkg/agent/pipeline_llm.go`
- 当前轮次中的 `data:image/...` 会被保存为临时文件并注入 `[image:/path]` 标签，而不是直接把 image URL 发给非视觉模型。
- 历史消息里的内联 data URL 会被丢弃，避免后续轮次重放大块 base64 图片导致非视觉模型失败。
- 新增 `agents.defaults.image_model` 路由逻辑：
  - 当前轮次含图片时优先切到配置的 image model。
  - 如果正在使用轻量模型，则图片轮次绕过轻量模型，回到主模型候选集。
  - 视觉不支持时不再静默去掉图片重试，而是返回明确错误，提示配置多模态模型。
- 工具产生的图片只在当前轮次生成合成用户消息，历史工具结果不会重复触发图片输入。
- 支持用户显式要求“保存图片附件”时，后端直接保存附件，并要求模型只回复保存结果，不再调用工具分析图片。

## 3. Launcher Web 后端、鉴权和移动端跳转

目的：让 Web launcher 在公网/反向代理/移动端场景下更安全、更可控，并保证认证流程能回到正确页面。

主要变更：

- Launcher 访问控制支持更细粒度配置：
  - `web/backend/middleware/access_control.go`
  - 新增 `IPAllowlistConfig`。
  - 新增 `allow_localhost_bypass`。
  - 新增 `trusted_proxy_cidrs`。
  - 只有直接来源属于可信代理 CIDR 时才信任 `X-Forwarded-For`。
- Launcher 启动时记录 allowlist bypass 风险提示：
  - `web/backend/main.go`
  - 当公网绑定、配置了 CIDR 且 localhost bypass 仍开启时输出 info/warn。
- API handler 保存新的访问控制选项：
  - `web/backend/api/router.go`
  - `web/backend/api/version.go`
  - `web/backend/api/startup.go`
- 移动端未登录跳转保留 redirect：
  - `web/backend/middleware/launcher_dashboard_auth.go`
  - `/mobile` 或 `/mobile/...` 未授权时跳到 `/launcher-login?redirect=...`。
- 前端系统配置类型增加：
  - `allow_localhost_bypass`
  - `trusted_proxy_cidrs`
- Windows 子进程启动工具拆出平台文件：
  - `web/backend/utils/exec_nonwindows.go`
  - `web/backend/utils/exec_windows.go`
  - 用于减少 Windows 启动子进程时控制台闪现等问题。

## 4. Agent 循环、上下文预算和稳定性

目的：提升长上下文、重试、终止、并发和异常场景下的 Agent 稳定性。

主要变更：

- 新增上下文预算裁剪逻辑：
  - `pkg/agent/context_budget.go`
  - 在上下文超限时按完整历史 turn 裁剪旧消息，尽量保护当前轮次消息和工具调用顺序。
- 上下文重建时区分稳定历史和当前活跃轮次：
  - `pkg/agent/turn_state.go`
  - `splitHistoryForActiveTurn`
  - `matchingTurnMessageTail`
  - 防止上下文压缩或恢复时误删当前用户消息/工具结果。
- LLM 临时错误重试改为统一分类：
  - `pkg/agent/pipeline_llm.go`
  - 使用 provider error classifier 区分 timeout、network、rate_limit、server_error 等。
- active turn 清理更加安全：
  - 避免不同 turn 之间错误删除 active state。
- Agent loop 使用条件变量计数替代原先 WaitGroup 相关逻辑，提高停止/清理稳定性。
- 为运行时事件、turn coordination、turn state、context seahorse、evolution bridge 等增加大量测试：
  - `pkg/agent/agent_test.go`
  - `pkg/agent/context_budget_test.go`
  - `pkg/agent/context_seahorse_test.go`
  - `pkg/agent/evolution_bridge_test.go`
  - `pkg/agent/runtime_event_logger_test.go`
  - `pkg/agent/turn_coord_test.go`
  - `pkg/agent/turn_state_test.go`

## 5. 会话、配置和隔离语义

目的：让不同聊天来源的会话隔离更明确，并让历史消息在 UI 和 API 中有稳定时间信息。

主要变更：

- 会话消息增加 `created_at`：
  - `pkg/session/manager.go`
  - `web/backend/api/session.go`
  - `web/frontend/src/api/sessions.ts`
  - Web API 返回历史消息时缺失 `created_at` 的消息会回填 session 更新时间。
- `dm_scope` 配置加入并和内部 `dimensions` 双向映射：
  - `pkg/config/config.go`
  - `web/backend/api/config.go`
  - `web/frontend/src/components/config/config-page.tsx`
  - `web/frontend/src/components/config/config-sections.tsx`
  - 支持 `per-channel-peer`、`per-channel`、`per-peer`、`global`。
- Patch config 时如果修改 `dm_scope`，会清理旧的派生 dimensions，避免旧配置残留。
- 增加 `RegisterChannelSettings` hook：
  - `pkg/config/config_channel.go`
  - 支持 out-of-tree channel 注册配置字段。
- 主会话 alias 晋升逻辑更严格，避免误把 main-session alias 当成普通历史。
- PID 单例检查增强：
  - `pkg/pid/pidfile.go`
  - `pkg/pid/pidfile_unix.go`
  - `pkg/pid/pidfile_windows.go`
  - Windows 平台新增对应实现。

## 6. 工具系统、安全边界和 Web 访问保护

目的：增强工具调用能力，同时收紧 SSRF、本地文件访问、远程命令和序列化错误处理。

主要变更：

- 新增安全 HTTP guard：
  - `pkg/utils/http_guard.go`
  - `pkg/utils/http_guard_test.go`
  - 阻止 loopback、私网、链路本地、多播、未指定地址、CGNAT、198.18.0.0/15 等。
  - 覆盖 IPv6 6to4、Teredo、ISATAP 内嵌 IPv4 场景。
  - 支持私网白名单和代理首跳例外。
- Web 工具增强和安全修复：
  - `pkg/tools/integration/web.go`
  - 增强 SSRF 防护。
  - 修复 Sogou 结果解析。
  - 新增 Kagi native web search provider 支持。
  - 对 Brave 空结果增加诊断日志。
- Cron 工具新增查询和更新能力：
  - `pkg/tools/cron.go`
  - `pkg/cron/service.go`
  - 新增 `get`、`update` action。
  - 远程渠道只能查看/更新当前 channel/chat 可访问的任务。
  - 新增 `command_allowed_remotes`，允许配置特定远程渠道执行 command cron。
- Shell/exec 工具增强：
  - `pkg/tools/shell.go`
  - 支持工作区相对路径。
  - 对 scheme-less URL（如 `wttr.in/Beijing`）避免误判为本地路径，同时保留本地路径存在时的安全校验。
  - 后台 session 的 goroutine 增加 panic recover。
  - 多处 JSON marshal 错误不再忽略。
- 消息工具支持媒体附件：
  - `pkg/tools/integration/message.go`
  - `pkg/tools/integration/message_test.go`
  - 新增配置 `tools.message.media_enabled`。
- 工具 loop、spawn、subagent、base context 等多处类型断言增加 ok 检查，减少 panic 风险。

## 7. 模型 Provider 和模型能力

目的：扩展可用模型供应商，补齐认证方式和流式输出兼容性。

主要变更：

- Azure OpenAI 支持 Entra ID / DefaultAzureCredential：
  - `pkg/providers/azure/identity.go`
  - `pkg/providers/azure/identity_stub.go`
  - `pkg/providers/azure/provider.go`
  - `pkg/providers/factory_provider.go`
  - 没有 API key 时可通过 Azure Identity 获取 bearer token。
- 新增 NEAR AI Cloud provider：
  - `pkg/providers/provider_metadata.go`
  - `pkg/providers/factory_provider.go`
  - 作为 OpenAI-compatible provider 接入。
- OpenAI-compatible provider 增强：
  - 支持 DeepSeek thinking fields 映射。
  - `native_search` 类型异常时记录结构化 warning。
  - 解析 stream tool call 参数失败时使用结构化日志。
- Codex OAuth provider 流式输出修复：
  - `pkg/providers/oauth/codex_provider.go`
  - 保留 streamed text delta。
  - 保留 streamed output item，避免工具调用或文本被丢弃。
- Bedrock provider：
  - 对不再支持 temperature 的模型禁用 temperature。
- Anthropic：
  - 默认模型 ID 更新为 canonical Sonnet ID。
  - 适配新版 SDK。
- 增加 Zhipu、MiMo、DeepSeek、Kagi、Gemini thought signature 等兼容修复。

## 8. 聊天渠道和外部平台适配

目的：提升 Telegram、Discord、OneBot、Slack、Feishu、Pico、Weixin 等渠道在媒体、流式工具调用和路由场景下的正确性。

主要变更：

- Discord：
  - 下载附件以进入 vision pipeline。
- Telegram：
  - 支持 location message。
  - forum topic 使用 composite chat ID。
  - 大量 Telegram 单元测试补充。
- OneBot：
  - group reply 使用带前缀 chatID。
  - 入站媒体下载更严格，阻止私网媒体 fetch。
  - 增加 OneBot 测试。
- Slack / Feishu / Pico / Weixin：
  - 补充媒体 fallback、文本语义和 channel manager 测试。
- 流式回复中的 `tool_calls` 不再被丢弃。
- outbound message tool 支持媒体附件。

## 9. Seahorse、记忆和自进化相关

目的：提升短期记忆裁剪、历史重建和自进化存储的健壮性。

主要变更：

- Seahorse fresh tail 预算控制和重建路径修复：
  - `pkg/seahorse/short_assembler.go`
  - `pkg/seahorse/short_engine.go`
  - `pkg/seahorse/store.go`
- 保留 active tool-call turn，避免裁剪时破坏工具调用序列。
- Session history bootstrap 保留 `created_at`。
- JSONL memory 读写更稳健：
  - `pkg/memory/jsonl.go`
  - `pkg/memory/jsonl_test.go`
- Evolution store：
  - 写入/关闭错误处理增强。
  - `lockStoreFile` repair 使用 CAS。
  - heartbeat turn 跳过 cold path。

## 10. 音频、TTS、事件总线和运行时

目的：提升音频流、TTS 和运行时事件处理的可靠性。

主要变更：

- TTS：
  - `pkg/audio/tts/openai_tts.go`
  - `pkg/audio/tts/tts.go`
  - 支持 OpenRouter voice override 和 fallback。
  - 增加错误响应读取、文件关闭等错误处理。
  - 补充测试。
- Message bus：
  - `pkg/bus/bus.go`
  - 对音频流增加 backpressure drop budget。
  - 新增事件类型和相关测试。
- Health server：
  - 修复 ready 状态。
  - 显式处理 JSON encode 错误。
- Gateway：
  - startup info 断言和 nil 场景增强，避免 panic。

## 11. 前端桌面聊天、代码块和国际化

目的：改善 Web 聊天的可读性、代码查看体验、图片输入体验和多语言覆盖。

主要变更：

- 代码块：
  - `web/frontend/src/components/chat/message-code-block.tsx`
  - `web/frontend/src/components/chat/message-code-block.utils.ts`
  - 新增行号和自动换行切换。
  - 新增 `web/frontend/src/store/code-block.ts` 保存代码块显示偏好。
- 聊天 UI：
  - 新增 context usage ring。
  - chat composer hint 调整。
  - shift-enter 提示。
  - assistant/user message 接入 timestamp 和附件显示。
- 国际化：
  - 新增 Bangla `bn-in`。
  - 新增 Czech `cs`。
  - 更新英文、中文、葡萄牙语文案。
- 路由树更新：
  - `web/frontend/src/routeTree.gen.ts`

## 12. 文档、资源、构建和依赖

目的：同步新功能文档、资源图片、构建脚本和安全依赖。

主要变更：

- README News 增加 `v0.2.5` 到 `v0.2.9` release highlights。
- 新增 PicoPaw banner：
  - `assets/picopaw-banner-en.webp`
  - `assets/picopaw-banner-zh.webp`
- 更新微信二维码：
  - `assets/wechat.png`
- 新增 Android Termux 指南：
  - `docs/guides/android-termux.md`
- 配置、provider、cron、tool security 文档更新：
  - `docs/guides/configuration.md`
  - `docs/guides/providers.md`
  - `docs/reference/cron.md`
  - `docs/reference/tools_configuration.md`
  - `docs/security/security_configuration.md`
- 多语言 README 添加 PicoPaw banner 或同步文案：
  - `docs/project/README.*.md`
- GitHub Actions 和依赖更新：
  - `actions/checkout` 升级。
  - Go 版本从 `1.25.10` 到 `1.25.11`。
  - 多个 Go/npm 依赖升级，包括 sqlite、AWS SDK、Azure SDK、Anthropic SDK、telego、vite、eslint 等。
- 新增 GitHub Sponsors 配置：
  - `.github/FUNDING.yml`

## 13. Onboard workspace、skills 和本地运行时产物

目的：扩展默认 onboard workspace 和技能文档；同时当前差异里也包含了一批本地运行时生成文件，需要单独看待。

源码/文档性质的新增：

- `onboard_workspace_embed.go`
- `workspace/skills/picoclaw-agent/SKILL.md`
- `workspace/skills/skill-creator/SKILL.md`
- `2026-06-26-picoclaw-wendi-mobile-h5-final-plan-zh`

运行时产物性质的新增：

- `.runtime/picoclaw-home/launcher-auth.db`
- `.runtime/picoclaw-home/logs/*`
- `.runtime/picoclaw-home/workspace/AGENT.md`
- `.runtime/picoclaw-home/workspace/HEARTBEAT.md`
- `.runtime/picoclaw-home/workspace/SOUL.md`
- `.runtime/picoclaw-home/workspace/USER.md`
- `.runtime/picoclaw-home/workspace/cron/jobs.json`
- `.runtime/picoclaw-home/workspace/memory/MEMORY.md`
- `.runtime/picoclaw-home/workspace/skills/*`
- `.runtime/picoclaw-home/workspace/state/state.json`

说明：这些 `.runtime` 文件是本地运行 launcher/agent 时生成或安装的状态、日志、认证数据库、workspace 和技能文件。它们出现在 `v0.2.9..HEAD` 差异中，但不属于常规源码功能变更；如果目标是整理可提交源码，应考虑把这类运行时产物从版本变更中剥离或加入忽略策略。

## 14. 测试覆盖变化

目的：为新增功能和历史 bug 修复补充回归测试。

新增或显著扩展的测试覆盖包括：

- Agent turn/context/media/routing：`pkg/agent/*_test.go`
- HTTP SSRF guard：`pkg/utils/http_guard_test.go`
- Web backend access/auth/config：`web/backend/**/*_test.go`
- Cron tool：`pkg/tools/cron_test.go`
- Shell exec：`pkg/tools/shell_test.go`
- Web search：`pkg/tools/integration/web_test.go`
- Message tool media：`pkg/tools/integration/message_test.go`
- Config dm_scope / channel settings：`pkg/config/*_test.go`
- Providers Azure/Codex/OpenAI-compatible/Bedrock：`pkg/providers/**/*_test.go`
- Channels Telegram/OneBot/Slack/Feishu/Pico/Weixin：`pkg/channels/**/*_test.go`
- Memory / Seahorse / Session / Bus / Gateway / TTS：对应包内测试均有补充。

## 15. 变更影响总结

按目的归类后，当前版本相对 `picoclaw-v0.2.9` 的核心变化可以概括为：

- 面向移动端：新增独立 `/mobile` DiAgent H5 聊天入口，配套认证回跳、移动会话历史、图片发送和 nginx 反代示例。
- 面向多模态：让图片在前端、API、Agent 路由和模型候选间完整流转，同时避免非视觉模型误收图片。
- 面向公网部署：launcher 访问控制引入 trusted proxy 和 localhost bypass 开关，移动端反代可只开放必要路径。
- 面向稳定性：大量 panic 防护、类型断言检查、错误处理、上下文预算裁剪和临时 LLM 错误重试。
- 面向工具能力：Cron 支持 get/update，Web 搜索增加 Kagi，message tool 支持媒体，exec/path guard 更准确。
- 面向生态：新增/增强 Azure Entra ID、NEAR AI Cloud、DeepSeek thinking、Codex OAuth streaming、MiMo common models 等 provider 能力。
- 面向维护：补充大量测试、文档、i18n、依赖和资源更新。

