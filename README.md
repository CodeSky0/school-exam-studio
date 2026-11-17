# School Exam Studio

单学校部署的中小学智能题库与组卷系统。提供学校初始化、JWT 认证、题目管理、富文本公式、本地数学 OCR、组卷、A4 打印、教师管理、题目审核、CSV 全量事务导入，以及可替代教师重复操作的 AI Agent 工作台。

## 技术架构

- 前端：Next.js 16 App Router、React 19、TypeScript、Tailwind CSS v4、shadcn/ui、TanStack Query、Zustand、Zod、TipTap、KaTeX。
- 核心后端：Go、Echo、Ent，按 Handler、Service、Repository 分层；PostgreSQL 是唯一业务持久化存储。
- AI：OpenAI 兼容 Chat Completions（function calling）+ 内置确定性本地引擎；三栏对话工作台提供持久化会话、图片/文件附件与工具调用时间线，支持 PDF/Word/图片文档智能录题，所有写入受草稿权限约束并记录审计日志。
- OCR：独立 Python FastAPI sidecar，调用本地 Pix2Text，仅在内存中处理图片，不写题库。
- Redis：仅用于登录/注册限流与短期缓存，不保存业务事实数据。
- 测试：Go testing（真实 PostgreSQL）、Vitest、Playwright、OCR 适配层 pytest。

```
frontend/   Next.js 页面、题目编辑器、组卷购物车、A4 预览
backend/    Echo API、Ent 模型、版本化 SQL 迁移、CSV 模板
ocr/        Pix2Text FastAPI sidecar
docs/       设计规格、实施计划、部署、API 与 AI Agent 工具契约
```

## 环境要求

- Node.js 22.12+（sanitize-html 依赖的 htmlparser2@12 为纯 ESM，需要原生 `require(ESM)` 支持）
- pnpm 10
- Go 1.25+
- Python 3.11+（仅 OCR sidecar）
- PostgreSQL 16（需要 pgcrypto 扩展）
- Redis 7
- Docker 与 Docker Compose（可选，用于一键启动依赖）

## 快速启动

### 1. 启动依赖

```bash
docker compose up -d
```

Compose 启动 PostgreSQL 16、Redis 7 和 OCR sidecar。OCR 仅绑定 `127.0.0.1:8001`，模型缓存使用独立命名卷。

没有 Docker 时，请自行安装 PostgreSQL 16 与 Redis 7，并执行：

```bash
psql -U postgres -c "CREATE ROLE \"user\" LOGIN PASSWORD 'password' SUPERUSER;"
psql -U postgres -c "CREATE DATABASE school_db OWNER \"user\";"
```

### 2. 配置环境变量

```bash
cp .env.example .env
```

将 `JWT_SECRET` 替换为至少 32 字节的随机字符串，例如 `openssl rand -hex 32`。`.env` 不得提交到 Git。

### 3. 启动后端

```bash
cd backend
go mod tidy
export DATABASE_URL='postgres://user:password@localhost:5432/school_db?sslmode=disable'
export REDIS_URL='redis://localhost:6379'
export OCR_URL='http://localhost:8001'
export JWT_SECRET='替换为至少32字节的随机字符串'
# AI 可选：不设置 AI_API_KEY 时使用内置本地引擎
export AI_BASE_URL='https://api.openai.com/v1'
export AI_API_KEY=''
export AI_MODEL='gpt-4o-mini'
export AI_TIMEOUT_SECONDS='45'
# 学校设置中 API Key 的加密密钥；留空回退到 JWT_SECRET
export AI_SETTINGS_SECRET=''
# AI 对话附件落盘目录；生产环境需持久化并备份
export AI_UPLOAD_DIR='./data/ai-uploads'
# API 启动时默认自动执行迁移；设为 false 可关闭
export AUTO_MIGRATE='true'
# API 启动会自动迁移；也可在部署前手动执行 go run cmd/migrate/main.go
go run cmd/api/main.go
```

API 默认监听 `http://localhost:8080`，提供 `/healthz` 与 `/readyz`。

### 4. 启动前端

```bash
cd frontend
pnpm install
pnpm dev
```

访问 `http://localhost:3000`。

### 5. 首次使用

1. 打开 `http://localhost:3000/init`，创建学校与首位管理员。**系统没有预置账号或弱密码**，初始化只能成功一次。
2. 初始化成功后页面仅展示一次 8 位学校代码，请立即保存。
3. 教师访问 `/register`，凭学校代码自助注册并直接启用。
4. 管理员在 `/admin/questions` 审核发布草稿，教师才能将题目加入试卷。

### 6. AI Agent 对话工作台

教师与管理员从侧栏“AI 助手”或 `/ai` 进入，`/teacher/ai`、`/admin/ai` 继续兼容。工作台是三栏对话界面：左侧是持久化会话历史，中间是消息流与输入区，右侧是可发现、可展开参数的工具与技能目录。

**对话能力**

- 会话、消息与工具调用全部写入 PostgreSQL，刷新或换设备后继续。
- 支持一次发送最多 5 个附件：PNG/JPEG/WebP（≤10 MiB）、PDF、DOCX、TXT、Markdown、CSV（≤20 MiB）。
- 图片自动调用 OCR sidecar；PDF/DOCX 在后端抽取文本（扫描版 PDF 会提示改为图片 OCR）。
- 每条助手消息内嵌完整工具时间线：工具名、参数、状态、耗时、返回摘要、涉及题目 ID 与可展开的原始 JSON。
- 未配置模型时使用确定性本地规划器；模型调用失败时自动降级，已执行的工具结果不会重复执行。

**内置工具**（`GET /api/v1/ai/tools` 可供外部 Agent 发现）

`search_questions`、`get_question`、`bank_audit`、`analyze_question`、`organize_drafts`、`fill_explanations`、`generate_questions`、`generate_variants`、`compose_paper`、`list_attachments`、`extract_questions`、`import_document_questions`、`create_questions`、`update_draft`。

**内置技能**（`GET /api/v1/ai/skills`）

文档智能录题、整卷拆题、题库体检与整理、课时练习生成、期末复习卷、错题变式包。技能是前端一键提示词 + 工具组合模板，不绕过权限与审计。

**文档智能录题**

上传 PDF/Word/图片后说“把这份试卷按八年级数学导入题库”，Agent 会先 `extract_questions` 预览切分结果，再调用 `import_document_questions` 写入草稿题库；缺少答案的题会被跳过并在工具结果中列出。单次最多导入 20 道题。

**安全边界**

- AI 只自动修改草稿；已发布题目只读。
- 教师只能操作自己的草稿，管理员可检索本校题库，但写入仅限本人草稿。
- 所有 AI 写入记录 `ai_audit_logs`，生成内容标记待人工复核。
- 单次批量最多 10 道题，出题最多 10 道，文档导入最多 20 道。

**配置**

- `AI_UPLOAD_DIR`：附件落盘目录，默认 `./data/ai-uploads`；生产环境必须持久化并纳入备份。
- 管理员仍可在 `/admin/settings` 配置全校共享模型；学校配置优先于环境变量，API Key 加密存储且不会返回前端。
- 旧的一次性 Agent 运行接口 `/api/v1/ai/agent/runs` 保留，历史运行记录与审计仍可查询。

密钥只从环境变量或学校加密配置读取，不写入数据库明文、日志、前端响应或 Git。

### 7. 启动 OCR（可选）

```bash
cd ocr
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --host 127.0.0.1 --port 8001
```

首次识别会自动下载 Pix2Text 模型，需预留约 2 GB 磁盘与至少 4 GB 可用内存，CPU 识别单题通常需要数秒到数十秒。模型未加载完成时 `/readyz` 返回 503，识别接口不会返回模拟结果。OCR 结果必须由教师校对并显式保存后才进入题库，原图不持久化。

## 测试与验证

### 后端

```bash
psql -U postgres -c "CREATE DATABASE school_db_test OWNER \"user\";"
cd backend
export TEST_DATABASE_URL='postgres://user:password@localhost:5432/school_db_test?sslmode=disable'
go test ./...
go vet ./...
```

测试库隔离：每个 Go 测试用例会在 `TEST_DATABASE_URL` 基础上派生独立数据库（名称包含测试名与哈希），并行执行互不影响；未设置 `TEST_DATABASE_URL` 时回退到嵌入式 PostgreSQL 16。禁止把测试连接到业务库。

### 前端

```bash
cd frontend
pnpm lint
pnpm typecheck
pnpm test --run
pnpm build
```

### 端到端（Playwright）

E2E 使用真实后端、前端生产构建与 Chromium，不使用前端假数据。首次运行准备：

```bash
psql -U postgres -c "CREATE DATABASE school_db_e2e OWNER \"user\";"
cd frontend
pnpm build
pnpm exec playwright install chromium
pnpm test:e2e
```

`playwright.config.ts` 会自动执行迁移并启动后端（8080）与前端（3000），全局初始化通过真实 API 创建学校、管理员和教师，并保存登录态到 `tests/e2e/.auth/`（已被 Git 忽略）。覆盖登录注册、录题审核、组合筛选、组卷排序、0.10/0.20 定点总分、购物车刷新恢复、教师禁用/启用、重置密码、学校设置与 A4 PDF 打印。

- 账号可用 `E2E_ADMIN_EMAIL`、`E2E_ADMIN_PASSWORD`、`E2E_TEACHER_EMAIL`、`E2E_TEACHER_PASSWORD` 覆盖，默认值仅用于本地测试。
- 数据库连接可用 `E2E_DATABASE_URL`、`E2E_REDIS_URL` 覆盖。
- 测试创建的数据按唯一标签清理；需要全新环境时可删除并重建 `school_db_e2e` 与 `tests/e2e/.auth/`。

### OCR 适配层

```bash
cd ocr
python -m pytest app/main_test.py -v
```

适配层测试通过 monkeypatch 模拟引擎，覆盖空文件、错误 MIME、未就绪与空识别。真实模型冒烟需启动 sidecar 后调用：

```bash
curl -F "image=@/path/to/formula.png;type=image/png" http://127.0.0.1:8001/v1/recognize
```

应返回非空文本与 LaTeX 定界符，例如 `{"text":"已知 $x^2=4$"}`。模拟引擎只允许用于单元测试，不能替代真实模型验收。

## CSV 导入

- 模板：`backend/assets/questions-template.csv`，表头固定为 `题型,年级,学科,难度,知识点标签,题干,选项A,选项B,选项C,选项D,答案,解析`。
- UTF-8（可带 BOM），标签用英文分号分隔，题干/答案/解析可含 `$…$` LaTeX。
- 单文件最大 5 MB、1,000 数据行；学科必须与当前教师一致，难度 1–5。
- `POST /api/v1/questions/import/validate` 只校验预览，`POST /api/v1/questions/import` 重新读取原文件并在一个 PostgreSQL 事务中导入；任一行失败均零写入，错误包含真实行号。

## 打印

试卷打印页 `/teacher/papers/:id/print` 使用 `@page { size: A4; margin: 16mm 14mm 18mm; }`，打印时隐藏全部应用 UI，仅保留 `.paper-sheet` 试卷内容，题目尽量避免跨页断裂。PDF 导出使用浏览器原生打印，不提供伪 PDF 接口。

## 常见故障

- **pnpm test 报 `require() of ES Module ... htmlparser2`**：Node.js 版本低于 22.12，请升级。
- **迁移报 pgcrypto 错误**：PostgreSQL 未正确安装 pgcrypto 或其 OpenSSL 依赖不完整，请使用官方 PostgreSQL 16 构建。
- **Go 测试报 Dirty database**：测试库被并发迁移打断；删除重建 `school_db_test*` 数据库后重试，当前版本已按用例隔离。
- **E2E 无法登录教师**：E2E 库被其他流程初始化且学校代码未知，重建 `school_db_e2e` 并删除 `tests/e2e/.auth/`。
- **OCR 一直 503**：模型仍在下载/加载，查看 sidecar 日志与磁盘空间；模型未就绪时不会返回识别结果。
- **修改请求 403 ORIGIN_REQUIRED**：非浏览器调用必须携带与 `CORS_ORIGIN` 一致的 `Origin` 头。

## 文档

- 设计与计划：`docs/superpowers/`
- 部署：`docs/deployment.md`
- API：`docs/api.md`
- AI Agent 工具与技能契约：`docs/ai-agent-tools.md`
