# AI Agent 对话工作台 · 实时工具链与收敛文案

日期：2026-11-17
状态：已完成
目标分支：`main`

## 背景

`/ai` 已完成第一版对话化重构（会话历史、图片/文件附件、工具调用时间线）。当前仍有三个体验缺口：

1. 发送消息后必须等待整轮结束才看到工具调用，思考过程不透明，不像 Agent 对话。
2. 工作台内仍有较多说明性、口号式文案（侧栏页脚、右栏能力说明、空状态长段落、只读长提示），干扰对话本身。
3. AI 与业务页面仍相对割裂：工具结果只能复制题目 ID，题目/试卷页面也不能带着上下文进入 AI 对话。

## 目标

- 对话消息通过 SSE 实时推送：用户消息、每个工具的开始/结束、最终回复逐步出现。
- 保留并强化完整工具调用链：参数、状态、耗时、返回摘要、涉及题目/试卷的可点击入口。
- 删除工作台内所有非功能性文案，只保留操作标签、状态与必要提示。
- AI 进一步融入应用：题目编辑器、题库 AI 面板、试卷质检面板可以直接带着上下文跳转到 AI 对话；工具结果可以跳回题库/试卷。

## 非目标

- 不引入新的第三方依赖，不改造 OCR/Pix2Text。
- 不改动旧的一次性 Agent 运行接口与会话存储模型。
- 不做模型 Token 级流式（工具链与最终消息实时即可）；LLM 客户端保持现有非流式调用。

---

## Task 1: 后端 SSE 实时消息

**Files**

- Modify: `backend/internal/ai/conversation_types.go`
- Modify: `backend/internal/ai/conversation_service.go`
- Modify: `backend/internal/ai/conversation_handler.go`
- Modify: `backend/internal/ai/handler.go`
- Create: `backend/internal/ai/conversation_stream_test.go`

**接口**

```go
type ChatStreamEvent struct {
    Type         string        `json:"type"` // user_message | tool_started | tool_finished | assistant_message | done | error
    Message      *ChatMessage  `json:"message,omitempty"`
    ToolCall     *ToolCall     `json:"tool_call,omitempty"`
    Conversation *Conversation `json:"conversation,omitempty"`
    Error        string        `json:"error,omitempty"`
}

type ChatEventSink func(ChatStreamEvent)

func (s *Service) SendMessageStream(ctx context.Context, principal auth.Principal, conversationID uuid.UUID, input SendMessageInput, sink ChatEventSink) (SendMessageResult, error)
```

- `SendMessage` 与 `SendMessageStream` 复用同一个 `sendMessage(..., sink)` 实现，旧接口行为不变。
- 在写入用户消息后推送 `user_message`。
- `executeToolCall` 在调用工具前后推送 `tool_started` / `tool_finished`，两个事件都携带完整 `ToolCall`。
- Handler 在服务返回后推送 `assistant_message`（含会话）与 `done`；出错时推送 `error` 并结束。
- 路由：`POST /api/v1/ai/conversations/:id/messages/stream`，`text/event-stream`，`X-Accel-Buffering: no`，每帧 `event:<type>\ndata:<json>\n\n` 后立即 Flush。
- 客户端断连时依赖请求 context 取消，不额外起 goroutine。

**验证**

- 新增纯函数测试：事件 JSON 帧格式、错误映射、事件类型序列（不依赖数据库）。
- `go build ./... && go vet ./internal/ai`。

## Task 2: 前端流式消费与 Agent 对话 UI

**Files**

- Modify: `frontend/lib/ai-chat.ts`
- Modify: `frontend/lib/ai-chat.test.ts`
- Modify: `frontend/components/ai/chat/agent-chat-workspace.tsx`
- Modify: `frontend/components/ai/chat/message-list.tsx`
- Modify: `frontend/components/ai/chat/message-bubble.tsx`
- Modify: `frontend/components/ai/chat/tool-call-timeline.tsx`
- Modify: `frontend/components/ai/chat/conversation-sidebar.tsx`

**实现**

- `streamMessage()`：`fetch` + `ReadableStream` 解析 SSE，按事件回调；支持 `AbortSignal`；非 2xx 复用 `ApiError` 语义。
- 工作台状态：`liveMessage`（用户消息 + 实时工具数组 + 最终文本/状态），替换现有只显示“正在规划”的占位。
- 发送时先乐观插入用户消息，工具开始/结束即时更新对应卡片；`done` 后刷新消息列表并清空实时态；`error` 展示并保留输入。
- 发送中提供“停止”按钮：abort 请求，恢复输入框。
- 最终回复晚于工具链到达时，实时气泡直接替换为服务端持久化消息，避免闪烁。
- 简化：删除只读长提示、`条消息` 徽标等冗余信息，仅保留标题、连接状态、工具数。

**验证**

- `pnpm test --run lib/ai-chat.test.ts`
- `pnpm typecheck && pnpm lint`

## Task 3: 删除工作台无用文案

**Files**

- Modify: `frontend/components/ai/chat/agent-chat-workspace.tsx`
- Modify: `frontend/components/ai/chat/conversation-sidebar.tsx`
- Modify: `frontend/components/ai/chat/message-list.tsx`
- Modify: `frontend/components/ai/chat/message-bubble.tsx`
- Modify: `frontend/components/ai/chat/composer.tsx`
- Modify: `frontend/components/ai/chat/skills-panel.tsx`
- Modify: `frontend/components/ai/chat/tool-call-timeline.tsx`

**删除清单**

- 侧栏页脚“会话保存在学校数据库中…”。
- 右栏“Agent 能力 / N 项能力 · N 个工具 / 安全说明”段落。
- 空状态营销段落与安全口号，只保留一句引导 + 技能快捷入口。
- 只读会话长句，改为“只读”。
- 输入框下方格式支持列表，改为上传按钮的 `title`/`aria-label`。
- 工具卡片“可在…搜索这些 ID 查看草稿”的说明。
- 消息底部 provider/model 长标签，收敛为时间 + 工具次数。
- 保留所有状态、错误、权限边界等功能性信息。

**验证**

- 人工检查 `/ai` 空状态、会话中、工具展开三种视图。

## Task 4: AI 融入业务页面

**Files**

- Modify: `frontend/components/ai/chat/agent-chat-workspace.tsx`（支持 `?prompt=` 预填）
- Modify: `frontend/components/question/question-bank-ai.tsx`
- Modify: `frontend/components/ai/question-ai-assistant.tsx`
- Modify: `frontend/components/paper/paper-list.tsx`
- Modify: `frontend/components/ai/chat/tool-call-timeline.tsx`

**实现**

- `/ai?prompt=...` 打开时预填输入框并聚焦；从业务页面跳转时携带上下文（题目 ID / 试卷 ID / 当前操作）。
- 题库 AI 面板增加“在 AI 助手中继续”，携带待复核数量或体检结果摘要。
- 题目编辑器 AI 面板增加“在 AI 助手中深入分析”，携带题目 ID。
- 试卷质检结果增加“在 AI 助手中处理”，携带试卷 ID。
- 工具时间线：题目 ID 渲染为可点击链接（教师/管理员题库），试卷 ID 渲染为试卷页链接；保留复制 ID。

**验证**

- `pnpm typecheck && pnpm lint`
- 手工点击各入口，确认 `/ai` 预填正确且可发送。

## Task 5: 清理旧工作台死代码

**Files**

- Delete: `frontend/components/ai/ai-workspace.tsx`
- Delete: `frontend/components/ai/agent-console.tsx`
- Delete: `frontend/components/ai/agent-run-report.tsx`
- Delete: `frontend/components/ai/ai-status-card.tsx`

- 先确认无引用后删除；`lib/ai.ts` 中仍在使用的状态/审核函数保留。

**验证**

- `pnpm typecheck` 无未使用导入；`pnpm build` 通过。

## Task 6: 文档、回归与推送

**Files**

- Modify: `README.md`
- Modify: `docs/api.md`

- README 对话工作台章节补充实时工具链与业务入口说明。
- `docs/api.md` 增加 SSE 端点、事件类型与示例。

**回归**

```bash
cd backend && go build ./... && go vet ./... && go test ./internal/ai ./internal/document
cd ../frontend && pnpm install --frozen-lockfile && pnpm lint && pnpm typecheck && pnpm test --run && pnpm build
```

- 无数据库环境下，跳过需要真实 PostgreSQL 的集成测试并在交付说明中标注。
- 通过 `git push` 更新 `main`；若历史不一致，基于远端 `main` 新建提交后推送。

## 完成记录

- 后端新增 `POST /api/v1/ai/conversations/:id/messages/stream`，复用原 SendMessage 执行路径，按 `user_message` / `tool_started` / `tool_finished` / `assistant_message` / `done` / `error` 推送；新增 SSE 帧与错误映射单元测试。
- 前端新增 `streamMessage()` SSE 解析与实时工具链渲染，支持停止生成；最终消息写入 React Query 缓存后再刷新，避免闪烁。
- 工作台删除侧栏页脚、右栏能力说明、空状态营销段落、只读长提示、输入格式说明与工具 ID 操作说明。
- 题目编辑器、题库 AI 面板、试卷质检结果新增 `/ai?prompt=...` 入口；工具时间线可跳转题库与试卷。
- 删除 `ai-workspace.tsx`、`agent-console.tsx`、`agent-run-report.tsx`、`ai-status-card.tsx` 四个已无引用的旧工作台文件。
- 验证：`pnpm lint`、`pnpm typecheck`、`pnpm test --run`（112 项）、`pnpm build`、`go build ./...`、`go vet ./...` 通过；后端依赖嵌入式 PostgreSQL 的集成测试因容器以 root 运行 `initdb` 被拒绝而跳过。

## 风险

- SSE 经过反向代理可能缓冲：已显式设置 `Cache-Control: no-cache`、`X-Accel-Buffering: no`，并每帧 Flush。
- 前端 abort 后后端可能已完成写入：`done` 缺失时前端以最终消息列表刷新为准，避免重复。
- 旧接口与非流式客户端保持兼容。
