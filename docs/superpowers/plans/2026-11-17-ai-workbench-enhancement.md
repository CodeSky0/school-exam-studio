# AI 工作台增强实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 AI 模型调用具备重试、多模型故障转移、熔断与 JSON 修复，默认不再静默降级本地；同时把工具扩展到 22 个、技能扩展到 10 个，并接入题目列表与试卷页。

**Architecture:** 新增 `llm_runtime.go` 作为模型调用弹性层，统一被 `Analyze`/`Generate`/`GenerateVariants`/对话轮次调用。工具与技能继续走 `tools.go` 注册表，新增试卷、统计、复核类工具，并暴露 `POST /ai/tools/:name`、`POST /ai/skills/:id/run` 给外部 Agent。前端通过 `lib/ai-tools.ts` 直接调用工具端点。

**Tech Stack:** Go 1.25 / Echo / Ent / PostgreSQL 16 / OpenAI 兼容 Chat Completions；Next.js 16 / React 19 / TypeScript / TanStack Query；Go testing / Vitest。

**Spec:** `docs/superpowers/specs/2026-11-17-ai-workbench-enhancement-design.md`

## Global Constraints

- 不新增第三方 Go 依赖与前端运行时依赖。
- 重试/故障转移只作用于模型 HTTP 调用，绝不重放已执行的工具写操作。
- 写工具只作用于草稿；已发布题目只读。
- 未配置模型时允许本地引擎，但必须标注 `provider=local`。
- 已配置模型失败时默认返回 `model_unavailable` 错误，除非 `AI_ALLOW_LOCAL_FALLBACK=true`。
- 每次写操作写 `ai_audit_logs`。
- 所有新工具必须有参数白名单与测试。

## 文件结构

```text
backend/internal/ai/
  llm_error.go            新增：ErrorKind、错误分类、ProviderError 扩展
  llm_runtime.go          新增：重试、故障转移、熔断、JSON 修复
  llm_runtime_test.go     新增
  service.go              修改：Analyze/Generate/GenerateVariants 使用运行时
  conversation_service.go 修改：对话轮次使用运行时，取消静默本地规划器
  tools.go                修改：注册新工具、参数白名单、直接执行
  tools_paper.go          新增：试卷类工具
  tools_analytics.go      新增：统计/重复题/复核/相似题工具
  tools_paper_test.go     新增
  tools_analytics_test.go 新增
  tool_descriptors.go     修改：新增 8 个描述 + category 字段
  skills.go               修改：新增 4 个技能 + 分组字段
  handler.go              修改：/ai/tools/:name、/ai/skills/:id/run
  status_test.go          修改：模型健康字段
frontend/
  lib/ai-tools.ts         新增：runTool
  lib/ai-tools.test.ts    新增
  components/ai/ai-status-card.tsx 修改
  app/teacher/questions/page.tsx / app/admin/questions/page.tsx 修改
  app/teacher/papers/page.tsx 修改
docs/ai-agent-tools.md    修改
README.md / .env.example  修改
```

---

### Task 1: 错误分类

**Files:**
- Create: `backend/internal/ai/llm_error.go`
- Create: `backend/internal/ai/llm_error_test.go`
- Modify: `backend/internal/ai/llm.go`
- Modify: `backend/internal/ai/llm_tools.go`

- [x] **Step 1: 写失败测试**

```go
func TestClassifyProviderError(t *testing.T) {
    cases := []struct {
        status int
        want   ErrorKind
    }{
        {http.StatusUnauthorized, ErrorKindConfig},
        {http.StatusTooManyRequests, ErrorKindRetryable},
        {http.StatusInternalServerError, ErrorKindRetryable},
        {http.StatusBadRequest, ErrorKindFatal},
    }
    for _, tc := range cases {
        got := classifyStatus(tc.status)
        if got != tc.want {
            t.Fatalf("status %d: got %s want %s", tc.status, got, tc.want)
        }
    }
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/ai -run TestClassifyProviderError`
Expected: FAIL `undefined: ErrorKind`

- [x] **Step 3: 实现**

```go
type ErrorKind string

const (
    ErrorKindConfig    ErrorKind = "config"
    ErrorKindRetryable ErrorKind = "retryable"
    ErrorKindFatal     ErrorKind = "fatal"
    ErrorKindParse     ErrorKind = "parse"
)

type ProviderError struct {
    StatusCode int
    Message    string
    Kind       ErrorKind
    RetryAfter time.Duration
}

func classifyStatus(status int) ErrorKind {
    switch {
    case status == http.StatusUnauthorized, status == http.StatusForbidden,
        status == http.StatusNotFound:
        return ErrorKindConfig
    case status == http.StatusRequestTimeout, status == http.StatusConflict,
        status == http.StatusTooManyRequests, status >= http.StatusInternalServerError:
        return ErrorKindRetryable
    default:
        return ErrorKindFatal
    }
}
```

在 `llm.go`/`llm_tools.go` 构造 `ProviderError` 时填入 `Kind: classifyStatus(response.StatusCode)`；网络错误包装为 `&ProviderError{Kind: ErrorKindRetryable, Message: ...}`；JSON 解码失败包装为 `ErrorKindParse`。

- [x] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/ai -run TestClassifyProviderError`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add backend/internal/ai/llm_error.go backend/internal/ai/llm_error_test.go backend/internal/ai/llm.go backend/internal/ai/llm_tools.go
git commit -m "feat(ai): classify model provider errors"
```

---

### Task 2: 弹性运行时

**Files:**
- Create: `backend/internal/ai/llm_runtime.go`
- Create: `backend/internal/ai/llm_runtime_test.go`
- Modify: `backend/internal/ai/service.go`（Service 增加 runtime 字段）

**Interfaces:**
- `type modelCandidate struct { Client *LLMClient; Config LLMConfig }`
- `(s *Service) resolveCandidates(ctx, principal, requestedModel string) ([]modelCandidate, string, error)`
- `(s *Service) completeJSON(ctx, principal, requestedModel, system, user string) (string, LLMConfig, []string, error)`
- `(s *Service) chatToolsTurn(ctx, principal, requestedModel string, run func(*LLMClient, LLMConfig) (LLMReply, []ToolCall, error)) (...)`
- `breaker`：`allow(modelKey) bool`、`failure(modelKey)`、`success(modelKey)`

- [x] **Step 1: 写失败测试（重试与熔断）**

```go
func TestBreakerOpensAfterConsecutiveFailures(t *testing.T) {
    b := newBreakerSet(time.Minute, 3)
    key := "school:model"
    if !b.allow(key, time.Now()) { t.Fatal("initial allow") }
    for i := 0; i < 3; i++ { b.failure(key, time.Now()) }
    if b.allow(key, time.Now()) { t.Fatal("breaker should be open") }
    if !b.allow(key, time.Now().Add(2*time.Minute)) { t.Fatal("breaker should half-open after cooldown") }
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/ai -run TestBreaker`
Expected: FAIL `undefined: newBreakerSet`

- [x] **Step 3: 实现 `llm_runtime.go`**

实现要点：
- `resolveCandidates`：学校配置启用时返回默认模型优先、去重后最多 3 个候选；显式 `requestedModel` 只返回该模型；环境变量配置作为单候选。
- `completeJSON`：遍历候选；每个候选最多 3 次；仅 `retryable`/`parse` 继续；`config`/`fatal` 立即返回；退避 300ms、900ms（±20% 抖动）；`RetryAfter` 取大；每次调用前检查 `ctx.Err()`。
- 解析失败时调用 `client.CompleteJSON(ctx, "只输出合法 JSON，不要解释。", 原问题+修复指令)`。
- 熔断器按 `schoolID|model` 计数，连续 3 次可重试失败后 60s 内 `allow=false`；成功清零。
- 返回尝试过的模型列表与最后错误；`AI_ALLOW_LOCAL_FALLBACK=true` 时由调用方决定是否用本地。

- [x] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/ai -run 'TestBreaker|TestRetry'`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add backend/internal/ai/llm_runtime.go backend/internal/ai/llm_runtime_test.go backend/internal/ai/service.go
git commit -m "feat(ai): add resilient model runtime with retry and circuit breaker"
```

---

### Task 3: Analyze/Generate/Variants 接入运行时并取消静默降级

**Files:**
- Modify: `backend/internal/ai/service.go`
- Create: `backend/internal/ai/resilience_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestAnalyzeDoesNotSilentlyFallbackWhenConfiguredModelFails(t *testing.T) {
    svc := newTestServiceWithFailingLLM(t)
    _, err := svc.Analyze(context.Background(), teacherPrincipal(), AnalyzeInput{
        Subject: "数学", Content: "1+1=?", QuestionType: "single_choice",
    })
    if !errors.Is(err, ErrModelUnavailable) {
        t.Fatalf("got %v want ErrModelUnavailable", err)
    }
}

func TestAnalyzeUsesLocalWhenNoModelConfigured(t *testing.T) {
    svc := newTestService(t) // no LLM
    analysis, err := svc.Analyze(context.Background(), teacherPrincipal(), AnalyzeInput{
        Subject: "数学", Content: "1+1=?", QuestionType: "single_choice",
    })
    if err != nil || analysis.Provider != ProviderLocal {
        t.Fatalf("got %+v err %v", analysis, err)
    }
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/ai -run TestAnalyze`
Expected: FAIL `undefined: ErrModelUnavailable`

- [x] **Step 3: 实现**

- 在 `types.go` 增加 `ErrModelUnavailable = errors.New("ai model unavailable")`。
- `Analyze`：先 `local := s.local.Analyze(input)` 用于字段兜底；未配置模型时返回 local；配置了模型时调用 `s.completeJSON`，失败返回 `fmt.Errorf("%w: %v", ErrModelUnavailable, err)`，除非 `allowLocalFallback()` 为真。
- `generate`/`generateVariants`：同样改为 `s.completeJSON`；失败时返回 `ErrModelUnavailable`（或开启兜底时使用本地并标注 `ProviderFallback`）。
- `allowLocalFallback()` 读取 `AI_ALLOW_LOCAL_FALLBACK`，默认 false。

- [x] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/ai -run 'TestAnalyze|TestGenerate'`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add backend/internal/ai/service.go backend/internal/ai/types.go backend/internal/ai/resilience_test.go
git commit -m "fix(ai): stop silent local fallback when configured model fails"
```

---

### Task 4: 对话轮次接入运行时

**Files:**
- Modify: `backend/internal/ai/conversation_service.go`
- Create: `backend/internal/ai/conversation_resilience_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestChatDoesNotUseLocalPlannerWhenModelConfiguredButFails(t *testing.T) {
    svc := newTestServiceWithFailingLLM(t)
    reply, calls, provider, _, _ := svc.runChatTurn(context.Background(), teacherPrincipal(),
        Conversation{ID: uuid.NewString()}, nil, "体检一下题库", nil, "", uuid.NewString())
    if provider == ProviderLocal || len(calls) > 0 {
        t.Fatalf("got provider %s calls %d reply %q", provider, len(calls), reply)
    }
    if !strings.Contains(reply, "模型") {
        t.Fatalf("expected model error reply, got %q", reply)
    }
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/ai -run TestChatDoesNotUseLocalPlanner`
Expected: FAIL（当前会返回本地规划结果）

- [x] **Step 3: 实现**

- `runChatTurn`：有模型配置时只走 `runLLMTurn`；失败且 `len(calls)>0` 保留摘要并提示"继续"；失败且无工具结果返回可读错误（`ProviderLLM` + 错误文本），不再调用 `planChatTurn`。
- 未配置模型时才调用 `planChatTurn`，保持现有离线能力。
- `runLLMTurn` 内部改为按候选模型尝试；仅在**尚未执行任何工具**时允许切换模型，已执行工具后失败立即停止。

- [x] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/ai -run TestChat`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add backend/internal/ai/conversation_service.go backend/internal/ai/conversation_resilience_test.go
git commit -m "fix(ai): keep chat failures visible instead of switching to local planner"
```

---

### Task 5: 新增 8 个工具

**Files:**
- Create: `backend/internal/ai/tools_paper.go`
- Create: `backend/internal/ai/tools_analytics.go`
- Create: `backend/internal/ai/tools_paper_test.go`
- Create: `backend/internal/ai/tools_analytics_test.go`
- Modify: `backend/internal/ai/tool_descriptors.go`
- Modify: `backend/internal/ai/tools.go:51-84,742-789`

**工具签名（统一）**

```go
func (s *Service) toolListPapers(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
func (s *Service) toolGetPaper(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
func (s *Service) toolPaperAudit(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
func (s *Service) toolUpdatePaperDraft(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
func (s *Service) toolBankStats(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
func (s *Service) toolFindDuplicates(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
func (s *Service) toolReviewQueue(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
func (s *Service) toolSearchSimilar(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
```

- [x] **Step 1: 写失败测试**

```go
func TestBankStatsCountsDistribution(t *testing.T) {
    svc, principal := newToolTestService(t)
    result, err := svc.RunTool(context.Background(), principal, ToolContext{}, "bank_stats", map[string]any{})
    if err != nil { t.Fatal(err) }
    if result.Data["total"].(int) != 3 { t.Fatalf("total=%v", result.Data["total"]) }
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/ai -run TestBankStats`
Expected: FAIL `不支持的工具`

- [x] **Step 3: 实现工具与描述**

- `list_papers` 参数：`subject`、`grade`、`keyword`、`limit`。
- `get_paper`：`paper_id`；返回 `paper` 与题目快照。
- `paper_audit`：`paper_id`；输出总分、题量、难度分布、知识点覆盖、缺答案题。
- `update_paper_draft`：`paper_id`、`questions[]`（`question_id`、`assigned_score`）、`title`、`duration_minutes`；仅本人草稿试卷，调用 `papers.Update`。
- `bank_stats`：`subject`、`grade`、`limit`；统计题型/难度/知识点/分类分布与缺口。
- `find_duplicates`：`limit`；返回 `qa.Duplicates` 分组。
- `review_queue`：`subject`、`grade`、`limit`；筛选 `AIGenerated && AIMeta["review_required"]==true && Status==draft`。
- `search_similar_questions`：`question_id` 或 `keyword`、`subject`、`grade`、`limit`；对快照做题干分词重叠度 + 知识点重叠打分，返回 top N。
- 在 `toolRegistry` 注册，在 `validateToolArguments` 增加白名单，在 `tool_descriptors.go` 增加描述与 `category`。

- [x] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/ai -run 'TestBankStats|TestPaper|TestDuplicate|TestReviewQueue|TestSimilar'`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add backend/internal/ai/tools_paper.go backend/internal/ai/tools_analytics.go backend/internal/ai/tools_*_test.go backend/internal/ai/tool_descriptors.go backend/internal/ai/tools.go
git commit -m "feat(ai): add paper, analytics, review and similarity tools"
```

---

### Task 6: 工具直接执行端点

**Files:**
- Modify: `backend/internal/ai/handler.go:29-56`
- Modify: `backend/internal/ai/handler_test.go`（或新增）

- [x] **Step 1: 写失败测试**

```go
func TestRunToolEndpointRejectsUnknownField(t *testing.T) {
    h, e, principal := newTestHandler(t)
    req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/tools/bank_stats", strings.NewReader(`{"bogus":1}`))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req.WithContext(principalContext(principal)))
    if rec.Code != http.StatusBadRequest { t.Fatalf("got %d", rec.Code) }
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/ai -run TestRunToolEndpoint`
Expected: 404 / FAIL

- [x] **Step 3: 实现**

```go
group.POST("/ai/tools/:name", h.RunTool)
group.POST("/ai/skills/:id/run", h.RunSkill)

func (h *Handler) RunTool(c echo.Context) error {
    principal, ok := platformmiddleware.PrincipalFrom(c)
    if !ok { return unauthorized() }
    var args map[string]any
    if err := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, maxAIBodyBytes)).Decode(&args); err != nil && !errors.Is(err, io.EOF) {
        return aiError(validationError("body", "请求体必须是 JSON 对象"))
    }
    result, err := h.service.RunTool(c.Request().Context(), principal, ToolContext{RunID: uuid.NewString()}, c.Param("name"), args)
    if err != nil { return aiError(err) }
    return c.JSON(http.StatusOK, result)
}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/ai -run TestRunToolEndpoint`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add backend/internal/ai/handler.go backend/internal/ai/*_test.go
git commit -m "feat(ai): expose direct tool execution endpoint"
```

---

### Task 7: 技能扩展与一键执行

**Files:**
- Modify: `backend/internal/ai/skills.go`
- Modify: `backend/internal/ai/handler.go`
- Modify: `backend/internal/ai/conversation_service.go`（新增 `RunSkill` service 方法）
- Create: `backend/internal/ai/skills_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestListSkillsIncludesNewSkills(t *testing.T) {
    svc := &Service{}
    ids := map[string]bool{}
    for _, skill := range svc.ListSkills() { ids[skill.ID] = true }
    for _, want := range []string{"bank_gap_analysis", "duplicate_cleanup", "paper_quality_check", "review_assistant", "exam_blueprint", "similar_question_pack"} {
        if !ids[want] { t.Fatalf("missing skill %s", want) }
    }
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd backend && go test ./internal/ai -run TestListSkills`
Expected: FAIL

- [x] **Step 3: 实现**

- `Skill` 增加 `Category string`、`Steps []string`。
- 新增 6 个技能（spec 第 6 节）。
- `Service.RunSkill(ctx, principal, skillID, content, model)`：查找技能，创建会话，调用 `SendMessage`，返回 `SkillRunResult{ConversationID, Message, SkillID}`。
- `POST /ai/skills/:id/run` 绑定 `{content?, model?}`。

- [x] **Step 4: 运行测试确认通过**

Run: `cd backend && go test ./internal/ai -run 'TestListSkills|TestRunSkill'`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add backend/internal/ai/skills.go backend/internal/ai/skills_test.go backend/internal/ai/handler.go backend/internal/ai/conversation_service.go
git commit -m "feat(ai): expand skills catalog and add skill run endpoint"
```

---

### Task 8: 状态与可观测

**Files:**
- Modify: `backend/internal/ai/types.go`（`Status` 增加 `ModelsHealth []ModelHealth`、`FallbackPolicy string`）
- Modify: `backend/internal/ai/service.go:68-121`
- Create: `backend/internal/ai/status_test.go`

- [x] **Step 1: 写失败测试**

```go
func TestStatusExposesModelHealth(t *testing.T) {
    status := (&Service{}).Status(context.Background(), teacherPrincipal())
    if status.FallbackPolicy == "" { t.Fatal("fallback policy missing") }
}
```

- [x] **Step 2: 实现**：`Status` 填充 `FallbackPolicy`（`error` 或 `local`）与 `ModelsHealth`（模型、是否熔断、失败次数）。

- [x] **Step 3: 运行测试并提交**

Run: `cd backend && go test ./internal/ai -run TestStatus`
Expected: PASS

```bash
git add backend/internal/ai/types.go backend/internal/ai/service.go backend/internal/ai/status_test.go
git commit -m "feat(ai): expose model health and fallback policy"
```

---

### Task 9: 前端工具调用与业务入口

**Files:**
- Create: `frontend/lib/ai-tools.ts`
- Create: `frontend/lib/ai-tools.test.ts`
- Modify: `frontend/components/ai/ai-status-card.tsx`
- Modify: `frontend/app/teacher/questions/page.tsx`
- Modify: `frontend/app/admin/questions/page.tsx`
- Modify: `frontend/app/teacher/papers/page.tsx`

- [x] **Step 1: 写失败测试**

```ts
import { describe, expect, it, vi } from "vitest";
import { runTool } from "./ai-tools";

describe("runTool", () => {
  it("posts arguments and returns result", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ summary: "ok", data: {} }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    const result = await runTool("bank_stats", {});
    expect(result.summary).toBe("ok");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/ai/tools/bank_stats", expect.objectContaining({ method: "POST" }));
  });
});
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd frontend && npm run test -- ai-tools`
Expected: FAIL（依赖未安装时先 `npm ci`）

- [x] **Step 3: 实现**

```ts
export async function runTool<T = unknown>(name: string, args: Record<string, unknown> = {}): Promise<{ summary: string; data: T; warnings: string[] }> {
  const response = await fetch(`/api/v1/ai/tools/${name}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(args),
  });
  if (!response.ok) throw new Error(await readError(response));
  return response.json();
}
```

- 状态卡展示 `models_health` 与 `fallback_policy`。
- 题目列表加"AI 待复核"筛选：调用 `review_queue`，展示返回题目并可跳转。
- 试卷页加"AI 质检"按钮：选中试卷后调用 `paper_audit`，以卡片展示结果。
- 所有操作完成后 `invalidateQueries` 刷新题库/试卷缓存。

- [x] **Step 4: 运行测试确认通过**

Run: `cd frontend && npm run test -- ai-tools && npx tsc --noEmit`
Expected: PASS

- [x] **Step 5: 提交**

```bash
git add frontend/lib/ai-tools.ts frontend/lib/ai-tools.test.ts frontend/components/ai/ai-status-card.tsx frontend/app/teacher/questions/page.tsx frontend/app/admin/questions/page.tsx frontend/app/teacher/papers/page.tsx
git commit -m "feat(ai): add tool client and business entry points"
```

---

### Task 10: 文档、验证与推送

**Files:**
- Modify: `docs/ai-agent-tools.md`
- Modify: `README.md`
- Modify: `.env.example`

- [x] **Step 1: 更新文档**：工具表新增 8 行、技能表新增 6 行、新增两个端点、补充弹性策略与 `AI_ALLOW_LOCAL_FALLBACK`。
- [x] **Step 2: 全量验证**

Run:
```bash
cd backend && go test ./... && go build ./...
cd ../frontend && npx tsc --noEmit
```
Expected: 全部通过。

- [x] **Step 3: 提交并推送**

```bash
git add docs/ai-agent-tools.md README.md .env.example
git commit -m "docs(ai): document resilient runtime and new tools"
git push origin HEAD
```

## Self-Review

- 规格覆盖：弹性层（Task 1-4）、工具（Task 5-6）、技能（Task 7）、状态（Task 8）、前端（Task 9）、文档与推送（Task 10）。
- 无占位符；所有新增类型与函数在任务中给出签名。
- 类型一致：`ErrorKind`、`modelCandidate`、`ToolResult`、`SkillRunResult` 在各任务中命名一致。

---

## 完成记录（2026-11-26）

- 弹性运行时、错误分类、熔断与 JSON 修复（`llm_error.go` / `llm_runtime.go`）、8 个新工具、`POST /ai/tools/:name`、技能扩展与 `POST /ai/skills/:id/run`、模型健康状态、前端 `lib/ai-tools.ts` 与业务入口均已在先前提交落地。
- 2026-11-26 全量回归：`go test ./...`、`go vet ./...`、前端 lint/typecheck/124 项单测/生产构建、Playwright 24 项 E2E 全部通过。
