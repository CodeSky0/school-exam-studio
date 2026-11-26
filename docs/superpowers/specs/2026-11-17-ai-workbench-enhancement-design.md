# AI 工作台增强设计：弹性模型调用、工具与技能扩展、业务融合

日期：2026-11-17
状态：已确认并实施（2026-11-26 验收）
目标分支：`trae/agent-2AowYl` → 合并到 `main`

## 1. 背景

现有 AI 工作台已经具备会话、附件、审计、14 个工具与 6 个技能，但存在三个问题：

1. **稳定性差**：`Analyze`、`Generate`、`GenerateVariants` 在模型超时、429/5xx、JSON 解析失败时静默返回本地模板结果；对话失败时切换到确定性本地规划器。用户看到的是"AI 降级了"，而不是错误原因。
2. **能力面窄**：工具集中在题目侧，缺少试卷、统计、重复题、复核队列、相似题能力；技能只是静态提示词。
3. **业务融入浅**：AI 产出停留在对话页，题目列表、试卷页没有 AI 入口，复核链路断裂。

## 2. 目标

- 模型调用具备**错误分类、重试、多模型故障转移、熔断、JSON 修复**，默认不再静默降级本地。
- 工具从 14 个扩展到 22 个，技能从 6 个扩展到 10 个，并提供外部 Agent 可直接调用的工具执行端点。
- AI 产出进入题目列表与试卷页的真实工作流。
- 不新增第三方 Go/前端运行时依赖。

## 3. 非目标

- SSE 流式输出。
- 向量检索（无 embedding 依赖）。
- MCP 服务端与跨用户 API Token。
- 修改已发布题目、发布/归档/删除等破坏性操作。

## 4. 架构

```text
前端（工作台 / 题目列表 / 试卷页）
        │  POST /ai/tools/:name
        ▼
internal/ai.Handler ──► Service.RunTool（校验、权限、审计）
        │
        ▼
Service.llmRuntime（新增）
  ├─ 错误分类 ProviderError.Kind
  ├─ 重试（指数退避 + Retry-After）
  ├─ 模型故障转移（学校启用模型顺序）
  ├─ 熔断器（进程内，按学校+模型）
  └─ JSON 修复重试
        │
        ├─ LLMClient（OpenAI 兼容）
        └─ LocalEngine（仅未配置模型或显式开启兜底时）
```

### 4.1 错误模型

扩展 `ProviderError`：

```go
type ErrorKind string

const (
    ErrorKindConfig    ErrorKind = "config"    // 401/403/404、模型未启用：不重试
    ErrorKindRetryable ErrorKind = "retryable" // 408/409/429/5xx、网络、超时、空响应
    ErrorKindFatal     ErrorKind = "fatal"     // 400/422、内容策略：不重试
    ErrorKindParse     ErrorKind = "parse"     // 返回内容无法解析为 JSON
)

type ProviderError struct {
    StatusCode int
    Message    string
    Kind       ErrorKind
    RetryAfter time.Duration
}
```

`CompleteJSON` 与 `ChatTools` 在构造错误时设置 `Kind`；网络/超时统一归类为 `retryable`。

### 4.2 重试、故障转移、熔断

- **候选模型**：`resolveLLMCandidates(ctx, principal, requestedModel)` 返回按「默认模型 → 其余启用模型」排序的候选列表，最多 3 个；显式 `requestedModel` 时只尝试该模型。
- **重试**：每个模型最多 3 次；退避 300ms、900ms（±20% 抖动），有 `RetryAfter` 时取较大值；受 `ctx` 截止时间约束。
- **故障转移**：某模型重试耗尽且错误为 `retryable`/`parse` 时切换下一个模型；`config`/`fatal` 立即停止。
- **熔断器**：`map[schoolID|model]*breaker`，连续 3 次 `retryable` 失败后打开 60 秒，期间直接跳过该模型；成功一次即清零。状态通过 `GET /ai/status` 的 `models_health` 暴露。
- **JSON 修复**：首次解析失败时追加一次修复调用（"仅输出合法 JSON，不要解释"），仍失败则返回 `parse` 错误。
- **关键约束**：重试与故障转移只发生在模型 HTTP 调用层，**不重放已执行的工具写操作**。

### 4.3 降级策略

- 学校未配置模型（`Source=local`）：继续使用本地确定性引擎，响应明确标注 `provider=local`。
- 学校已配置模型但调用失败：返回 `model_unavailable` 结构化错误，包含尝试过的模型、错误类型与建议；**不返回本地模板结果**。
- 环境变量 `AI_ALLOW_LOCAL_FALLBACK=true` 时，模型失败可回退本地，并在响应中标注 `provider=fallback` 与降级原因。
- 对话：模型失败且已执行工具 → 保留工具结果并提示"继续"；未执行工具 → 返回可重试错误并展示模型诊断，不再调用本地规划器；仅未配置模型时使用本地规划器。

## 5. 工具扩展

| 工具 | 写 | 说明 |
|---|---|---|
| `list_papers` | 否 | 按学科/年级/关键词列出试卷，返回 ID、标题、题量、总分 |
| `get_paper` | 否 | 读取试卷详情、题目顺序、分值、题目快照 |
| `paper_audit` | 否 | 试卷质检：总分、难度分布、知识点覆盖、缺答案题 |
| `bank_stats` | 否 | 题库分布：学科/年级/题型/难度/知识点覆盖与缺口 |
| `find_duplicates` | 否 | 重复题分组，返回题目 ID、重复次数与相似摘要 |
| `review_queue` | 否 | AI 生成且待人工复核的草稿列表 |
| `update_paper_draft` | 是 | 替换本人草稿试卷的题目与分值（复用 `paper.Service.Update`） |
| `search_similar_questions` | 否 | 基于题干关键词与知识点重叠度检索相似题（确定性打分） |

工具描述新增 `category` 字段：`question`、`paper`、`analytics`、`document`。

新增端点：

- `POST /api/v1/ai/tools/:name`：直接执行工具，请求体为工具参数；复用 `RunTool` 的参数校验、学科约束、权限与审计。写工具返回审计动作。
- `GET /api/v1/ai/tools` 保持兼容，仅新增字段。

### 5.1 新工具返回约定

统一 `ToolResult`：`summary`、`data`、`question_ids`、`warnings`。试卷类工具同时在 `data.paper_id` 返回试卷 ID。

## 6. 技能扩展

| 技能 ID | 名称 | 工具组合 |
|---|---|---|
| `bank_gap_analysis` | 题库覆盖度分析 | `bank_stats` → `bank_audit` |
| `duplicate_cleanup` | 重复题排查 | `find_duplicates` → `bank_audit` |
| `paper_quality_check` | 试卷质检 | `list_papers` → `paper_audit` |
| `review_assistant` | AI 产出复核 | `review_queue` → `get_question` |
| `exam_blueprint` | 双向细目表组卷 | `bank_stats` → `compose_paper` |
| `similar_question_pack` | 相似题练习包 | `search_similar_questions` → `create_questions` |

新增 `POST /api/v1/ai/skills/:id/run`：以技能提示词发起一次会话消息，返回 `conversation_id`、`assistant_message`，供前端一键执行和外部 Agent 调用。

## 7. 前端

- `frontend/lib/ai-tools.ts`：`runTool(name, args)` 调用 `POST /ai/tools/:name`；统一错误解析。
- 题目列表（`app/teacher/questions`、`app/admin/questions`）：增加"AI 待复核"筛选与行内操作（复核、补解析、生成变式）。
- 试卷页（`app/teacher/papers`）：增加"AI 质检"与"AI 补题"入口。
- `ai-status-card.tsx`：展示模型健康、熔断模型、最近失败原因与重试提示。
- 会话消息：模型失败时展示可重试错误与 provider 徽标，不再伪装成正常回复。

## 8. 可观测与安全

- `AgentRun`/`ToolCall` 审计写入 `attempts`、最终 `model`、`failover_from`。
- 所有写工具仍然只作用于草稿；`update_paper_draft` 仅允许本人草稿试卷。
- 工具参数严格校验，未知字段拒绝；教师学科约束不变。
- 不记录 API Key、不输出模型原始响应全文（截断到 300 字符）。

## 9. 测试

- Go 单测：
  - 错误分类（401/429/500/超时/坏 JSON）。
  - 重试次数、退避边界、`Retry-After`。
  - 故障转移顺序、熔断打开/半开。
  - 降级策略：已配置模型失败不返回本地结果；未配置模型返回 local。
  - 8 个新工具的 handler 与参数校验；`POST /ai/tools/:name` 的权限与审计。
  - 技能目录与 `skills/:id/run`。
- 前端：`lib/ai-tools.ts` 单测；`tsc --noEmit`。
- 验证命令：`go test ./...`、`go build ./...`、`npm run lint`（如依赖可用）。

## 10. 交付顺序

1. 弹性层与降级策略（含测试）。
2. 工具扩展与直接执行端点。
3. 技能扩展与一键执行端点。
4. 前端集成。
5. 文档更新（`docs/ai-agent-tools.md`、`README.md`、`.env.example`）。
6. `go test ./...` + `go build ./...`，提交并推送到 GitHub。
