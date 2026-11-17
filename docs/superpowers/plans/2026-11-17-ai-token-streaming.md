# AI Token 级流式与真 Agent 循环

日期：2026-11-17
状态：已完成
目标分支：`main`

## 目标

把现有「工具事件流 + 最终整段回复」升级为模型 Token 级流式：模型边生成边显示，工具调用仍然是真正的多轮 Agent 循环，用户可以随时停止。

## 方案

1. `LLMClient` 新增 `ChatToolsStream`：请求 `stream: true`，逐行解析 OpenAI 兼容 SSE，聚合 `delta.content`、`delta.reasoning_content` 与按 `index` 分片的 `tool_calls`。
2. Agent 循环每轮改用流式调用，新增事件：
   - `assistant_delta`：正文增量
   - `reasoning_delta`：推理增量（模型支持时）
   - 保留 `tool_started` / `tool_finished` / `assistant_message` / `done` / `error`
3. 已输出 Token 后不再重试同一模型，避免重复文本；未输出时仍走原有重试与多模型故障转移。
4. 前端按增量拼接实时气泡，显示流式光标与可折叠思考过程；最终消息落库后替换实时气泡。停止按钮中断 SSE 与后端上下文。
5. 多轮工具结果继续回填模型上下文，保持「模型决定调用工具 → 执行 → 再决策」的真 Agent 循环；轮次与工具上限小幅提升。

## 完成记录

- `llm_stream.go`：OpenAI 兼容 `stream: true` 客户端，支持 `content`、`reasoning_content` / `reasoning`、分片 `tool_calls`，兼容注释/心跳行与 `[DONE]`；已输出 Token 后不再重试。
- Agent 循环：每轮流式调用，新增 `assistant_delta` / `reasoning_delta`；保留中间轮次说明并与最终答复拼接；上一轮工具结果回填上下文；轮次上限 6、工具上限 12。
- 前端：实时拼接正文与思考过程，显示流式光标，Token 流期间即时滚动；历史消息 `memo` 化；最终消息先入缓存再替换实时气泡。
- 文档：README 与 `docs/api.md` 补充事件说明。

## 验证

- Go：流解析单测（分片工具参数、reasoning、`[DONE]`、HTTP 错误）、SSE 帧单测、`go build`/`go vet`。
- 前端：`pnpm lint`、`pnpm typecheck`、`pnpm test --run`、`pnpm build`。
- 推送 GitHub `main`。

## 风险

- 不同厂商的流式字段存在差异，解析器对 `reasoning` / `reasoning_content` 与空行、注释行做兼容。
- 流式请求不受整体 HTTP 超时限制，依赖请求上下文取消；停止生成即取消。
- Token 级更新频率高，前端只更新当前实时气泡，最终落库后统一刷新。
