# School Exam Studio Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付一套真实运行、权限严密、支持本地数学 OCR、富文本公式、CSV 全量事务导入和 A4 打印的单学校题库与组卷系统。

**Architecture:** Next.js 直接访问 Echo REST API，Go 按 Handler、Service、Repository 组织业务，Ent 提供类型安全数据访问，PostgreSQL 承担全部业务持久化。Pix2Text 作为独立本地 sidecar，只返回识别文本，不写题库；Redis 只承担限流与短期缓存。

**Tech Stack:** Next.js 16、React 19、TypeScript、Tailwind CSS v4、shadcn/ui、TanStack Query、Zustand、Zod、TipTap、KaTeX、Go、Echo、Ent、PostgreSQL、Redis、Python FastAPI、Pix2Text、Go testing、Vitest、Playwright。

## Global Constraints

- 核心后端只能使用 Go；Python 仅适配 Pix2Text，禁止 Node.js/Express 核心后端。
- PostgreSQL 是唯一业务持久化存储，禁止 SQLite、Firebase、Supabase 和文件业务库。
- 一套部署只允许初始化一所学校；所有业务 Repository 必须接收认证上下文中的 `school_id`。
- Redis 仅用于限流、验证码和短期缓存，不保存题目、用户或试卷事实数据。
- 原题图片最大 5 MB，仅在内存中处理，不写数据库或磁盘；OCR 结果必须人工确认后保存。
- CSV 任一行失败均零写入；合法数据在一个 PostgreSQL 事务内入库。
- 富文本必须在保存边界白名单消毒，KaTeX 禁止不受信任的 HTML 指令。
- 分值使用两位小数字符串和 PostgreSQL `NUMERIC(8,2)`，前端使用整数百分汇总。
- 打印必须使用真实试卷内容和 `@page size: A4`，打印时隐藏全部应用 UI。
- 每个任务先写失败测试，再实现，运行相关检查后提交；GitHub 授权可用时立即推送该提交。
- 禁止把访问令牌、密码、`.env` 或生成模型缓存提交到 Git。
- 最终必须明确报告实际执行、未执行及受环境阻塞的验证，不得把构建成功当作功能验收。

## File Structure

| 路径 | 责任 |
|---|---|
| `docker-compose.yml` | PostgreSQL、Redis、OCR 及健康检查 |
| `.env.example` | 无秘密的环境变量模板 |
| `backend/ent/schema/` | School、User、Question、Paper、PaperQuestion 模型 |
| `backend/migrations/` | 版本化 PostgreSQL up/down SQL |
| `backend/internal/platform/` | 配置、数据库、HTTP、错误、鉴权中间件和测试数据库 |
| `backend/internal/auth/` | 初始化、注册、登录、会话与密码 |
| `backend/internal/question/` | 题目 CRUD、组合筛选、审核和富文本消毒 |
| `backend/internal/ocr/` | Go OCR 客户端、图片限制和超时 |
| `backend/internal/paper/` | 试卷事务、排序、定点分值和权限 |
| `backend/internal/admin/` | 教师管理、学校设置和重置密码 |
| `backend/internal/importer/` | CSV 模板、预览、行级校验和事务导入 |
| `ocr/app/` | Pix2Text 引擎适配、 readiness 和内存图片识别 |
| `frontend/app/` | 公共、教师、管理员页面和打印路由 |
| `frontend/components/` | 应用壳、题目编辑器、筛选器、组卷和试卷预览 |
| `frontend/lib/` | API、Zod、类型、OCR 转换和整数百分计分 |
| `frontend/stores/` | 按用户隔离的组卷购物车 |
| `frontend/tests/` | Vitest 与 Playwright 测试 |
| `docs/` | 已确认设计、实施计划和交付文档 |

---

### Task 1: 可运行骨架、Ent 模型与数据库迁移

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`
- Create: `backend/go.mod`
- Create: `backend/cmd/api/main.go`
- Create: `backend/cmd/migrate/main.go`
- Create: `backend/internal/platform/config/config.go`
- Create: `backend/internal/platform/config/config_test.go`
- Create: `backend/internal/platform/database/database.go`
- Create: `backend/internal/platform/httpserver/httpserver.go`
- Create: `backend/internal/platform/migrator/migrator.go`
- Create: `backend/internal/platform/platformtest/postgres.go`
- Create: `backend/ent/schema/{school,user,question,paper,paperquestion}.go`
- Create: `backend/migrations/000001_initial.{up,down}.sql`
- Test: `backend/internal/platform/config/config_test.go`

**Interfaces:**
- Produces: `config.Load() (Config, error)`，其中 `Config` 包含 `DatabaseURL`、`RedisURL`、`OCRURL`、`JWTSecret`、`CORSOrigin`、`SecureCookie`。
- Produces: `database.Open(context.Context, string) (*ent.Client, error)`。
- Produces: `migrator.Up(string) error` 和 `platformtest.Open(t *testing.T) *ent.Client`。
- Produces: `GET /healthz`，返回 `{"status":"ok"}`；数据库未就绪时 `/readyz` 返回 503。

- [ ] **Step 1: 安装项目所需工具并写失败配置测试**

安装 Go 1.25 或更新稳定版，使用 Corepack 激活 pnpm 10，安装 GitHub CLI；Docker 不可用时安装真实本地 PostgreSQL 16 和 Redis 供验证，同时在最终报告注明 Compose 未运行原因。GitHub 授权必须由用户通过本机 `gh auth login` 完成，随后执行以下命令建立远端并推送已有设计提交：

```bash
gh auth status
gh repo create school-exam-studio --private --source=. --remote=origin
git push -u origin main
```

```go
func TestLoadRequiresStrongJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/school_db?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("OCR_URL", "http://localhost:8001")
	t.Setenv("JWT_SECRET", "short")
	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("expected JWT_SECRET error, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `cd backend && go test ./internal/platform/config -run TestLoadRequiresStrongJWTSecret -v`
Expected: FAIL，因为 `config.Load` 尚不存在。

- [ ] **Step 3: 实现配置、服务骨架和真实 PostgreSQL 模型**

```go
type Config struct {
	DatabaseURL string
	RedisURL    string
	OCRURL      string
	JWTSecret   string
	CORSOrigin  string
	SecureCookie bool
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL: os.Getenv("REDIS_URL"),
		OCRURL: os.Getenv("OCR_URL"),
		JWTSecret: os.Getenv("JWT_SECRET"),
		CORSOrigin: envOr("CORS_ORIGIN", "http://localhost:3000"),
		SecureCookie: os.Getenv("APP_ENV") == "production",
	}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" || cfg.OCRURL == "" {
		return Config{}, errors.New("DATABASE_URL, REDIS_URL and OCR_URL are required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	return cfg, nil
}
```

Ent 模型实现已确认设计中的全部字段。`School` 不含 `school_id`；其他模型包含软删除时间。`PaperQuestion` 同时保存 `school_id`、`paper_id`、`question_id`、`order_index` 和 `assigned_score`。SQL 使用 `pgcrypto`、`gen_random_uuid()`、JSONB、NUMERIC、复合外键、唯一约束和设计中的索引；down 迁移按依赖反序删表。

Compose 使用 PostgreSQL 16、Redis 7 和 OCR 构建上下文，数据库与模型使用命名卷，OCR 仅发布 `127.0.0.1:8001:8000`。API 启动只注册健康检查；后续任务在同一 `api` 分组追加路由。

- [ ] **Step 4: 生成 Ent 并验证迁移与服务**

Run: `cd backend && go generate ./ent && go test ./... && go vet ./...`
Run: `cd backend && go run cmd/migrate/main.go && go run cmd/api/main.go`
Expected: 测试和 vet 通过；迁移版本为 1；`/healthz` 返回 200，`/readyz` 在 PostgreSQL 正常时返回 200。

- [ ] **Step 5: 提交并推送**

```bash
git add docker-compose.yml .env.example backend
git commit -m "feat: add runnable backend and database foundation"
git push origin main
```

---

### Task 2: 初始化、认证、权限中间件与公共页面

**Files:**
- Create: `backend/internal/auth/{model,password,token,service,repository,handler}.go`
- Create: `backend/internal/auth/{password,service,token}_test.go`
- Create: `backend/internal/platform/middleware/{auth,origin,rate_limit}.go`
- Modify: `backend/cmd/api/main.go`
- Create: `frontend/package.json`
- Create: `frontend/app/{layout,page}.tsx`
- Create: `frontend/app/(public)/{init,login,register}/page.tsx`
- Create: `frontend/components/{app-shell,auth-form}.tsx`
- Create: `frontend/lib/{api,types,schemas}.ts`
- Create: `frontend/lib/schemas.test.ts`
- Test: `backend/internal/auth/service_test.go`

**Interfaces:**
- Consumes: Task 1 的 `config.Load`、`database.Open` 和 `platformtest.Open`。
- Produces: `POST /api/v1/init`、`GET /api/v1/init/status`、`POST /api/v1/auth/register`、`POST /api/v1/auth/login`、`POST /api/v1/auth/logout`、`GET /api/v1/auth/me`、`POST /api/v1/auth/change-password`。
- Produces: `Principal{UserID, SchoolID uuid.UUID, Role string, SessionVersion int}`，后续 Handler 只能通过 `middleware.PrincipalFrom(c)` 获取租户和角色。

- [ ] **Step 1: 写认证和单学校初始化失败测试**

```go
func TestInitializeOnlyOnce(t *testing.T) {
	client := platformtest.Open(t)
	svc := auth.NewService(auth.NewRepository(client), auth.NewPasswordHasher(12), "01234567890123456789012345678901", time.Hour)
	input := auth.InitializeInput{SchoolName: "实验中学", AdminName: "管理员", Email: "admin@example.com", Password: "StrongPass123!"}
	first, err := svc.Initialize(context.Background(), input)
	if err != nil || len(first.SchoolCode) != 8 {
		t.Fatalf("first initialization failed: %+v %v", first, err)
	}
	if _, err := svc.Initialize(context.Background(), input); !errors.Is(err, auth.ErrAlreadyInitialized) {
		t.Fatalf("expected ErrAlreadyInitialized, got %v", err)
	}
}
```

另写登录测试验证 bcrypt 错误密码使用相同外部错误，禁用用户和错误会话版本被认证中间件拒绝。

- [ ] **Step 2: 运行后端认证测试并确认失败**

Run: `cd backend && go test ./internal/auth -v`
Expected: FAIL，因为认证 Service 和错误类型尚未实现。

- [ ] **Step 3: 实现事务认证和真实页面**

初始化使用 `SELECT pg_advisory_xact_lock(...)`，在事务内检查学校数量并创建学校与管理员。学校代码由 `crypto/rand` 生成 8 位数字。注册查询代码对应学校并创建 active 教师。JWT 使用 HS256、两小时过期、用户/学校/角色/会话版本声明；Cookie 为 HttpOnly、SameSite Strict，生产环境 Secure。修改请求校验 Origin，登录和注册使用 Redis 滑动窗口限流。

Next.js 使用 shadcn/ui 表单，Zod 同时约束邮箱、最少 10 位密码和学校代码。`/init` 根据状态 API 在已初始化时跳转登录；成功后显示一次学校代码。应用壳按 `/api/v1/auth/me` 返回的角色显示管理员或教师菜单，未登录跳 `/login`。

```ts
export const registerSchema = z.object({
  schoolCode: z.string().regex(/^\d{8}$/),
  name: z.string().trim().min(2).max(50),
  subject: z.string().trim().min(1).max(30),
  email: z.string().trim().toLowerCase().email(),
  password: z.string().min(10).max(72),
});
```

- [ ] **Step 4: 验证注册登录闭环**

Run: `cd backend && go test ./... && go vet ./...`
Run: `cd frontend && pnpm install && pnpm lint && pnpm typecheck && pnpm test --run && pnpm build`
Expected: 全部通过；浏览器实际完成初始化、教师注册、登录和退出，数据库分别存在学校、管理员和教师记录。

- [ ] **Step 5: 提交并推送**

```bash
git add backend frontend
git commit -m "feat: implement school initialization and authentication"
git push origin main
```

---

### Task 3: 题目 CRUD、组合筛选与审核边界

**Files:**
- Create: `backend/internal/question/{model,sanitizer,service,repository,handler}.go`
- Create: `backend/internal/question/{sanitizer,service,repository}_test.go`
- Create: `backend/internal/platform/platformtest/fixtures.go`
- Modify: `backend/cmd/api/main.go`
- Test: `backend/internal/question/repository_test.go`

**Interfaces:**
- Consumes: `middleware.PrincipalFrom(c)` 和 Task 1 Ent Client。
- Produces: `QuestionService.Create/Update/Get/List/Delete`，所有方法首参为 `context.Context`，第二参为 `Principal`。
- Produces: `QuestionFilter{Subject, Grade, Type, Tag, CreatorID, Status string; Difficulty, Page, PageSize int}`。
- Produces: `POST/GET /api/v1/questions`、`POST /api/v1/questions/resolve`、`GET/PATCH/DELETE /api/v1/questions/:id`、`PATCH /api/v1/admin/questions/:id/status`。
- Produces: `ResolvePublished(context.Context, Principal, []uuid.UUID) ([]Question, error)`，只返回本校已发布题目，供购物车恢复校验。
- Produces: 测试辅助 `platformtest.Question(...)`、`QuestionCount(...)`、`TeacherPrincipal(...)` 及两校固定 UUID `SchoolA/SchoolB/TeacherA/TeacherB`。

- [ ] **Step 1: 写 XSS 与跨学校失败测试**

```go
func TestTeacherCannotDeleteOtherSchoolDraft(t *testing.T) {
	client := platformtest.Open(t)
	draft := platformtest.Question(t, client, platformtest.SchoolA, platformtest.TeacherA, "draft")
	principal := auth.Principal{UserID: platformtest.TeacherB, SchoolID: platformtest.SchoolB, Role: "teacher"}
	err := question.NewRepository(client).Delete(context.Background(), principal, draft.ID)
	if !errors.Is(err, question.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
```

消毒测试输入 `<p>已知<script>alert(1)</script>$x^2$</p>`，断言脚本被移除、段落和公式定界符保留；教师列表测试断言只返回自己的全部状态及他人 published，管理员返回本校全部状态。

- [ ] **Step 2: 运行题目测试并确认失败**

Run: `cd backend && go test ./internal/question -v`
Expected: FAIL，因为题目 Repository 和消毒器尚未实现。

- [ ] **Step 3: 实现题目服务**

使用 bluemonday 自定义白名单，仅保留段落、换行、基础排版、列表、表格和 TipTap 数学节点所需属性；删除脚本、事件属性、iframe 和危险 URL。选项逐字段消毒，标签去空白、去重并限制数量。

Repository 的每条查询先加 `school_id`，再叠加角色可见性和筛选；详情与更新同时按 ID、学校过滤。Create 强制 `creator_id=principal.UserID` 和 `draft`。教师 Update/Delete 额外要求本人和 draft；管理员状态接口只允许 draft→published 或 draft/published→archived。软删除只设置 `deleted_at`。

- [ ] **Step 4: 验证权限与筛选**

Run: `cd backend && go test ./internal/question -v && go test ./...`
Expected: 全部通过；两所学校夹具互不可见，六种筛选可组合，分页总数准确，恶意 HTML 不进入数据库。

- [ ] **Step 5: 提交并推送**

```bash
git add backend/internal/question backend/cmd/api/main.go
git commit -m "feat: add tenant-safe question management"
git push origin main
```

---

### Task 4: 富文本公式题目界面与真实本地 OCR

**Files:**
- Create: `ocr/{Dockerfile,requirements.txt,app/main.py,app/engine.py,app/main_test.py}`
- Create: `backend/internal/ocr/{client,handler}.go`
- Create: `backend/internal/ocr/client_test.go`
- Modify: `backend/cmd/api/main.go`
- Create: `frontend/components/question/{editor,math-extension,filter-bar,question-table,ocr-upload}.tsx`
- Create: `frontend/app/teacher/questions/page.tsx`
- Create: `frontend/lib/ocr-to-editor.ts`
- Create: `frontend/lib/ocr-to-editor.test.ts`
- Test: `ocr/app/main_test.py`

**Interfaces:**
- Consumes: Task 3 题目 API 和 Task 2 Principal。
- Produces: OCR sidecar `GET /healthz`、`GET /readyz`、`POST /v1/recognize`。
- Produces: Go `POST /api/v1/ocr/recognize`，响应 `{"text":"已知 $x^2=4$"}`，不写数据库。
- Produces: `ocrToEditorJSON(text string): JSONContent`，供 TipTap 直接插入。

- [ ] **Step 1: 写 OCR 转换和 sidecar 失败测试**

```ts
it("converts Chinese text and inline math without interpreting HTML", () => {
  const result = ocrToEditorJSON('已知 <img src=x onerror=alert(1)> $x^2=4$');
  expect(JSON.stringify(result)).toContain("x^2=4");
  expect(JSON.stringify(result)).toContain("img src=x onerror=alert(1)");
  expect(JSON.stringify(result)).not.toContain('"type":"image"');
});
```

Python 测试 monkeypatch 引擎，验证空文件返回 400、错误 MIME 返回 415、模型未就绪返回 503、识别为空返回 422；这些模拟只用于适配层单元测试。

- [ ] **Step 2: 运行前端与 OCR 测试并确认失败**

Run: `cd frontend && pnpm test --run lib/ocr-to-editor.test.ts`
Run: `cd ocr && python3 -m pytest app/main_test.py -v`
Expected: FAIL，因为转换函数和 OCR 路由尚未实现。

- [ ] **Step 3: 实现 Pix2Text、Go 客户端与题目编辑器**

Pix2Text 引擎在进程内懒加载并加锁，`/readyz` 只有模型加载完成才返回 200。识别接口使用 Pillow 从内存解码，限制 5 MB 和 20,000,000 像素，响应只包含识别文本。Go 客户端设置连接和总请求超时，有界 semaphore 控制并发，并将上游失败映射为 502/503/504。

TipTap 使用 StarterKit 和 Mathematics 扩展，KaTeX 设置 `throwOnError:false`、`strict:"error"`、`trust:false`。题目页支持真实列表、组合筛选、创建和编辑草稿；OCR 面板把结果写入当前活动字段，只有点击保存才调用题目 API。

- [ ] **Step 4: 执行真实模型冒烟与页面验证**

Run: `cd ocr && python3 -m pytest -v && uvicorn app.main:app --host 127.0.0.1 --port 8001`
Run: 使用包含中文题干和二次公式的测试图片调用 `POST /v1/recognize`，保存 JSON 结果供核对。
Run: `cd backend && go test ./... && cd ../frontend && pnpm test --run && pnpm typecheck && pnpm build`
Expected: 真实模型返回非空文本和可识别 LaTeX；页面显示公式、筛选和保存均来自真实 API，数据库不出现图片字段或图片字节。

- [ ] **Step 5: 提交并推送**

```bash
git add ocr backend/internal/ocr backend/cmd/api/main.go frontend
git commit -m "feat: add math question editor and local OCR"
git push origin main
```

---

### Task 5: 试卷事务、组卷购物车与 A4 打印

**Files:**
- Create: `backend/internal/paper/{model,score,service,repository,handler}.go`
- Create: `backend/internal/paper/{score,service}_test.go`
- Modify: `backend/cmd/api/main.go`
- Create: `frontend/stores/paper-store.ts`
- Create: `frontend/lib/score.ts`
- Create: `frontend/lib/score.test.ts`
- Create: `frontend/components/paper/{question-picker,paper-cart,paper-preview}.tsx`
- Create: `frontend/app/teacher/papers/page.tsx`
- Create: `frontend/app/teacher/papers/[id]/page.tsx`
- Create: `frontend/app/teacher/papers/[id]/print/page.tsx`
- Create: `frontend/app/print.css`
- Test: `backend/internal/paper/service_test.go`

**Interfaces:**
- Consumes: 已发布题目查询、Principal、题目 Zod 类型。
- Produces: `Score` 定点类型、`ParseScore(string) (Score, error)`、`SumScores([]Score) Score`。
- Produces: `POST/GET /api/v1/papers`、`GET/PATCH/DELETE /api/v1/papers/:id`。
- Produces: 前端 `toCents(string): number`、`fromCents(number): string` 和 Zustand `usePaperStore`。

- [ ] **Step 1: 写定点分值与事务失败测试**

```go
func TestSumScoresWithoutFloatingPointError(t *testing.T) {
	a, _ := paper.ParseScore("0.10")
	b, _ := paper.ParseScore("0.20")
	if got := paper.SumScores([]paper.Score{a, b}).String(); got != "0.30" {
		t.Fatalf("expected 0.30, got %s", got)
	}
}
```

事务测试提交一道他校 published 题目，断言返回 `ErrQuestionUnavailable` 且试卷和明细均为零；再测试草稿题目被拒绝、重复题目被拒绝、教师无法读取他人试卷。

- [ ] **Step 2: 运行试卷测试并确认失败**

Run: `cd backend && go test ./internal/paper -v`
Expected: FAIL，因为 Score 和 PaperService 尚未实现。

- [ ] **Step 3: 实现试卷事务与组卷界面**

PaperService 在事务内逐题按 ID、`school_id` 和 published 状态查询并锁定，验证学科年级与试卷一致，删除旧明细后按 `order_index` 批量插入，汇总 NUMERIC 并更新试卷。管理员可管理本校试卷，教师只能管理自己的试卷。

Zustand 使用 `paper-cart:${userID}` 持久化键，恢复后调用批量题目校验接口移除失效项。拖拽同时支持按钮上移/下移以保证可访问性；每题分值为两位小数字符串，前端用整数百分计算总分。

```ts
export function toCents(value: string): number {
  if (!/^(0|[1-9]\d{0,5})(\.\d{1,2})?$/.test(value)) throw new Error("invalid score");
  const [whole, fraction = ""] = value.split(".");
  return Number(whole) * 100 + Number(fraction.padEnd(2, "0"));
}
```

打印页使用学校名称、密封线、大题标题和答题留白。`print.css` 设置 `@page{size:A4;margin:16mm 14mm 18mm}`，打印时 `body *{visibility:hidden}` 后仅恢复 `.paper-sheet` 及其后代，并设置 `.paper-question{break-inside:avoid}`。

- [ ] **Step 4: 验证保存、刷新恢复和打印**

Run: `cd backend && go test ./... && cd ../frontend && pnpm test --run && pnpm typecheck && pnpm build`
Expected: 全部通过；实际创建试卷、拖拽排序、修改 0.10/0.20 分值、刷新恢复并打印，预览只有试卷且总分为 0.30。

- [ ] **Step 5: 提交并推送**

```bash
git add backend/internal/paper backend/cmd/api/main.go frontend
git commit -m "feat: implement paper builder and A4 printing"
git push origin main
```

---

### Task 6: 教师管理、学校设置与 CSV 全量事务导入

**Files:**
- Create: `backend/internal/admin/{service,repository,handler}.go`
- Create: `backend/internal/admin/service_test.go`
- Create: `backend/internal/importer/{model,parser,service,handler}.go`
- Create: `backend/internal/importer/{parser,service}_test.go`
- Modify: `backend/cmd/api/main.go`
- Create: `frontend/app/admin/{dashboard,teachers,questions,settings}/page.tsx`
- Create: `frontend/app/teacher/{dashboard,profile}/page.tsx`
- Create: `frontend/components/question/csv-import.tsx`
- Create: `backend/assets/questions-template.csv`
- Test: `backend/internal/importer/service_test.go`

**Interfaces:**
- Consumes: Principal、认证会话版本、题目消毒器和创建 Repository。
- Produces: `PATCH /api/v1/admin/teachers/:id/status`、`POST /api/v1/admin/teachers/:id/reset-password`、`GET/PATCH /api/v1/admin/school`。
- Produces: `GET /api/v1/admin/stats` 和 `GET /api/v1/teacher/stats`，分别返回本校管理统计及当前教师题目、试卷统计，供真实总览页面使用。
- Produces: `POST /api/v1/questions/import/validate` 和 `POST /api/v1/questions/import`，响应错误包含 CSV 实际行号。
- Produces: `ParseCSV(io.Reader) ([]ImportRow, []RowError, error)` 和 `Import(context.Context, Principal, []ImportRow) (int, error)`。

- [ ] **Step 1: 写 100 行成功和单行失败回滚测试**

```go
func validImportRows(n int) []importer.ImportRow {
	rows := make([]importer.ImportRow, n)
	for i := range rows {
		rows[i] = importer.ImportRow{Type: "single_choice", Grade: "七年级", Subject: "数学", Difficulty: 1, Tags: []string{"代数"}, Content: "<p>1+1=?</p>", Options: []question.Option{{Key: "A", Content: "1"}, {Key: "B", Content: "2"}}, Answer: "B", Explanation: "<p>基础加法</p>"}
	}
	return rows
}

func TestImportRollsBackAllRows(t *testing.T) {
	rows := validImportRows(100)
	rows[99].Difficulty = 6
	client := platformtest.Open(t)
	svc := importer.NewService(importer.NewRepository(client), question.NewSanitizer())
	count, err := svc.Import(context.Background(), platformtest.TeacherPrincipal(client), rows)
	if err == nil || count != 0 {
		t.Fatalf("expected full rejection, count=%d err=%v", count, err)
	}
	if got := platformtest.QuestionCount(t, client, platformtest.SchoolA); got != 0 {
		t.Fatalf("expected zero persisted questions, got %d", got)
	}
}
```

Parser 测试断言第 100 行之后的错误仍报告真实行号，BOM 被移除，标签分号拆分，选择题缺少选项或答案无效被拒绝。管理测试断言教师调用管理接口返回 403，禁用及重置密码立即使旧会话版本失效。

- [ ] **Step 2: 运行导入与管理测试并确认失败**

Run: `cd backend && go test ./internal/importer ./internal/admin -v`
Expected: FAIL，因为导入 Service 和管理 Service 尚未实现。

- [ ] **Step 3: 实现管理接口和 CSV 两阶段接口**

CSV 使用 `encoding/csv`，限制 5 MB 和 1,000 数据行，验证固定中文表头、学科、年级、题型、难度、选项和答案。Validate 只返回规范化预览和错误；Import 重新读取上传文件、完整校验后开启事务，逐行创建当前学校、当前教师的草稿，任何错误调用 rollback。

管理接口只允许 admin，Repository 所有操作包含学校条件。重置密码使用 `crypto/rand` 生成至少 16 位随机密码并递增会话版本，仅响应一次。前端提供教师筛选、启禁用确认、一次性密码展示、题目审核、学校名称设置、个人修改密码和 CSV 文件预览/确认导入。

- [ ] **Step 4: 验证真实 100 题导入和回滚**

Run: `cd backend && go test ./... && go vet ./... && cd ../frontend && pnpm test --run && pnpm build`
Expected: 全部通过；真实上传 100 行合法 CSV 后总数增加 100，再将一行难度改为 6 后总数不变，页面显示准确行号。

- [ ] **Step 5: 提交并推送**

```bash
git add backend frontend
git commit -m "feat: add administration and transactional CSV import"
git push origin main
```

---

### Task 7: 端到端验收、CI、文档与发布

**Files:**
- Create: `frontend/playwright.config.ts`
- Create: `frontend/tests/e2e/{auth,question,paper,admin,print}.spec.ts`
- Create: `.github/workflows/ci.yml`
- Create: `README.md`
- Create: `docs/deployment.md`
- Create: `docs/api.md`
- Modify: `.env.example`
- Test: `frontend/tests/e2e/*.spec.ts`

**Interfaces:**
- Consumes: 前六项全部真实 API、页面和基础设施。
- Produces: 一条可重复的本地启动、迁移、测试和打印验证流程。
- Produces: GitHub Actions Go/PostgreSQL、前端 Vitest 与构建、OCR 适配层测试。

- [ ] **Step 1: 写真实端到端测试**

```ts
test("teacher creates a formula draft and admin publishes it", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("邮箱").fill("teacher@example.com");
  await page.getByLabel("密码").fill(process.env.E2E_TEACHER_PASSWORD!);
  await page.getByRole("button", { name: "登录" }).click();
  await page.goto("/teacher/questions");
  await page.getByRole("button", { name: "新建题目" }).click();
  await page.getByLabel("题干").fill("求方程 $x^2=4$ 的正根");
  await page.getByRole("button", { name: "保存草稿" }).click();
  await expect(page.getByText("求方程")).toBeVisible();
});
```

E2E 使用通过初始化与管理 API 创建的真实测试账号，不在前端写死返回数据。每个用例清理自己的数据库记录，打印用例调用 Chromium PDF 并检查 CSS 页面尺寸和导航隐藏。

- [ ] **Step 2: 运行完整验证并修复失败**

Run: `cd backend && go test ./... && go vet ./...`
Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test --run && pnpm build && pnpm exec playwright test`
Run: `cd ocr && python3 -m pytest -v`
Run: 使用真实 Pix2Text 模型和测试公式图片执行一次端到端 OCR，再人工校对并保存题目。
Expected: 所有可运行检查通过；若 Docker 或模型资源阻塞，记录具体命令、错误和未验收行为，不宣称通过。

- [ ] **Step 3: 编写启动和交付文档**

README 必须列出 Node.js 22.9+、pnpm 10、Go 1.25+、Python 3.11+、PostgreSQL 16、Redis 7，提供以下真实命令：

```bash
docker compose up -d
cd backend && go mod tidy && go run cmd/migrate/main.go && go run cmd/api/main.go
cd frontend && pnpm install && pnpm dev
```

文档说明首次访问 `/init` 创建管理员，不存在预置弱密码；列出学校代码注册、CSV 模板、OCR 首次模型下载、资源需求、测试库隔离、A4 浏览器打印和常见故障。部署文档明确生产环境 TLS、Secure Cookie、强 JWT_SECRET、备份 PostgreSQL 和限制 OCR 内网访问。

- [ ] **Step 4: 最终权限与安全检查**

Run: `git diff --check && git status --short`
Run: 搜索仓库中的 `ghp_`、真实密码和私钥，确认 `.env`、模型缓存、测试报告及数据库文件未跟踪。
Expected: 无空白错误或敏感文件；第二所学校越权测试、CSV 回滚、禁用用户、XSS 和真实 OCR 结果均有验证记录。

- [ ] **Step 5: 提交、推送并确认远端**

```bash
git add .github README.md docs frontend backend ocr .env.example
git commit -m "test: complete end-to-end verification and delivery docs"
git push origin main
gh repo view --json name,visibility,url
```

Expected: GitHub 私有仓库 `school-exam-studio` 包含全部里程碑提交，工作区干净，最终回复列出启动方式、验证结果和任何真实阻塞。
