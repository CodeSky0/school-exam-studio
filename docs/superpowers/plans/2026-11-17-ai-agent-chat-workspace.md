# AI Agent 对话工作台实施计划

日期：2026-11-17
状态：已完成（2026-11-17）
目标分支：`main`

> **For agentic workers:** 按任务顺序执行；每个任务完成后运行该任务的验证命令再进入下一任务。步骤使用复选框跟踪。

**Goal:** 把一次性表单式「AI 工作台」重构为带会话历史、图片/文件附件、完整工具调用时间线的 AI Agent 对话工作台，并新增文档智能录题、整卷拆题、题库质检、相似题检索、练习单/复习卷生成等可复用工具与技能。

**Architecture:** 后端在现有 `internal/ai` 上增加「会话 → 消息 → 工具调用」持久化模型与运行时：LLM 可用时走 OpenAI 兼容 function calling，不可用时走确定性本地规划器，两者共用同一套工具注册表、权限与审计。文档解析（PDF/DOCX/TXT）与图片 OCR 统一产出纯文本，再交给题目切分器与结构化导入工具写入草稿题库。前端将 `/ai` 替换为三栏对话工作台，复用现有 Yohaku 设计令牌与 TanStack Query。

**Tech Stack:** Go 1.25 / Echo / Ent / PostgreSQL 16 / OpenAI 兼容 Chat Completions；Next.js 16 / React 19 / TypeScript / Tailwind v4 / TanStack Query / Zod；Vitest / Go testing / Playwright。

## Global Constraints

- PostgreSQL 是唯一业务事实存储；文件二进制存入 `AI_UPLOAD_DIR`，数据库只存元数据与抽取文本。
- 教师只能读写自己的会话；管理员可读本校会话，但只能写入自己的会话。
- AI 自动修改只作用于草稿；已发布题目只读；每次写入必须写 `ai_audit_logs`。
- 无模型密钥时功能必须可用，并明确标注 `provider=local`，不得伪装成大模型结果。
- 所有上传必须校验扩展名、MIME、magic bytes 与大小：图片 ≤ 10 MiB，文档 ≤ 20 MiB，每轮附件 ≤ 5 个。
- 抽取文本上限 200,000 字符；单条消息文本上限 8,000 字符；`goal` 上限 500 字保持不变。
- 不新增第三方 Go 依赖（PDF/DOCX 使用标准库实现），前端不新增运行时依赖。
- 不提交任何密钥；`.env.example` 只新增占位配置。
- 既有 `/api/v1/ai/agent/runs` 接口与页面兼容性保持，旧运行记录可在新工作台右栏查看。

---

## 文件结构

```text
backend/
  migrations/000004_ai_chat.up.sql / .down.sql
  ent/schema/aiconversation.go, aimessage.go, aiattachment.go
  internal/ai/
    conversation_types.go    会话/消息/附件/工具调用 DTO
    conversation_repository.go
    conversation_service.go  会话 CRUD、消息编排、附件解析调度
    conversation_handler.go  /ai/conversations/*, /ai/attachments/*
    tools.go                 工具描述、注册表、参数校验、工具执行
    tool_import.go           文档 → 题目切分 → 草稿入库
    skills.go                内置技能目录 + 工具目录 API
    chat_planner.go          本地确定性多轮规划器
    llm_tools.go             OpenAI tools 调用与多轮循环
    attachment_store.go      文件落盘、magic bytes、容量限制
  internal/document/
    text.go                  统一文本提取入口
    docx.go                  DOCX（zip + word/document.xml）
    pdf.go                   PDF 文本流 + ToUnicode 基础映射
    splitter.go              题目切分与规范化
    *_test.go
frontend/
  lib/ai-chat.ts             会话/消息/工具/附件 Zod schema 与 API
  lib/ai-chat.test.ts
  components/ai/chat/
    agent-chat-workspace.tsx  三栏容器、会话状态、发送编排
    conversation-sidebar.tsx  新建/搜索/重命名/删除/历史
    message-list.tsx          消息流、空状态、滚动锚点
    message-bubble.tsx        用户/助手消息、附件、Markdown 安全渲染
    tool-call-timeline.tsx    工具调用状态、参数、结果、题目跳转
    composer.tsx              文本 + 图片/文件选择 + 拖拽 + 技能快捷入口
    attachment-tray.tsx       上传队列、预览、删除、错误
    skills-panel.tsx          技能与工具目录
    attachment-image.tsx      鉴权 blob 图片预览
  app/ai/page.tsx              新工作台（教师/管理员共用）
  app/teacher/ai/page.tsx      保持路由，渲染同一工作台
  app/admin/ai/page.tsx       保持路由，渲染同一工作台
docs/api.md                    新增会话、附件、工具、技能接口
README.md                      新功能与环境变量
.env.example / docker-compose.yml  AI_UPLOAD_DIR 与卷
```

---

### Task 1: 会话数据模型与持久化

**Files:**
- Create: `backend/migrations/000004_ai_chat.up.sql`
- Create: `backend/migrations/000004_ai_chat.down.sql`
- Create: `backend/ent/schema/aiconversation.go`
- Create: `backend/ent/schema/aimessage.go`
- Create: `backend/ent/schema/aiattachment.go`
- Regenerate: `backend/ent/*` via Ent codegen
- Create: `backend/internal/ai/conversation_types.go`
- Create: `backend/internal/ai/conversation_repository.go`

**Interfaces:**
- Produces:
  - `type Conversation struct { ID, Title, Status string; CreatorID string; MessageCount int; LastMessageAt time.Time; CreatedAt time.Time; UpdatedAt time.Time }`
  - `type ChatMessage struct { ID, ConversationID, Role, Content, Status, Provider, Model, Error string; ToolCalls []ToolCall; Attachments []AttachmentRef; RunID string; CreatedAt time.Time }`
  - `type ToolCall struct { ID, Name, Label, Status, Summary, Error string; Arguments map[string]any; Result map[string]any; StartedAt, FinishedAt *time.Time }`
  - `type AttachmentRef struct { ID, Name, MIME, Kind, Status, TextPreview string; SizeBytes int64; TextChars int; Error string }`
  - `Repository.CreateConversation(ctx, principal, Conversation) (Conversation, error)`
  - `Repository.ListConversations(ctx, principal, limit int) ([]Conversation, error)`
  - `Repository.GetConversation(ctx, principal, id uuid.UUID) (Conversation, error)`
  - `Repository.UpdateConversation(ctx, principal, id uuid.UUID, title string, pinned bool) (Conversation, error)`
  - `Repository.DeleteConversation(ctx, principal, id uuid.UUID) error`
  - `Repository.AppendMessage(ctx, principal, ChatMessage) (ChatMessage, error)`
  - `Repository.ListMessages(ctx, principal, conversationID uuid.UUID, limit int) ([]ChatMessage, error)`
  - `Repository.CreateAttachment(ctx, principal, Attachment) (Attachment, error)`
  - `Repository.GetAttachment(ctx, principal, id uuid.UUID) (Attachment, error)`

- [ ] **Step 1: 写迁移**

`000004_ai_chat.up.sql` 建立三张表，字段与 Ent schema 完全一致：

```sql
CREATE TABLE ai_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    creator_id uuid NOT NULL,
    title varchar(120) NOT NULL DEFAULT '新对话',
    status varchar(20) NOT NULL DEFAULT 'active',
    pinned boolean NOT NULL DEFAULT false,
    message_count int NOT NULL DEFAULT 0,
    last_message_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT ai_conversations_status_valid CHECK (status IN ('active', 'archived')),
    CONSTRAINT ai_conversations_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT ai_conversations_school_id_id_key UNIQUE (school_id, id),
    CONSTRAINT ai_conversations_creator_school_fk FOREIGN KEY (school_id, creator_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);

CREATE TABLE ai_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    conversation_id uuid NOT NULL,
    role varchar(20) NOT NULL,
    content text NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'succeeded',
    provider varchar(20) NOT NULL DEFAULT 'local',
    model varchar(80) NOT NULL DEFAULT '',
    tool_calls jsonb NOT NULL DEFAULT '[]'::jsonb,
    attachments jsonb NOT NULL DEFAULT '[]'::jsonb,
    run_id uuid,
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT ai_messages_role_valid CHECK (role IN ('user', 'assistant', 'tool', 'system')),
    CONSTRAINT ai_messages_status_valid CHECK (status IN ('running', 'succeeded', 'failed')),
    CONSTRAINT ai_messages_tool_calls_array CHECK (jsonb_typeof(tool_calls) = 'array'),
    CONSTRAINT ai_messages_attachments_array CHECK (jsonb_typeof(attachments) = 'array'),
    CONSTRAINT ai_messages_conversation_fk FOREIGN KEY (school_id, conversation_id) REFERENCES ai_conversations(school_id, id) ON DELETE CASCADE
);

CREATE TABLE ai_attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    creator_id uuid NOT NULL,
    conversation_id uuid,
    name varchar(255) NOT NULL,
    mime varchar(120) NOT NULL,
    kind varchar(20) NOT NULL,
    size_bytes bigint NOT NULL,
    storage_path varchar(500) NOT NULL DEFAULT '',
    sha256 varchar(64) NOT NULL DEFAULT '',
    extracted_text text NOT NULL DEFAULT '',
    text_chars int NOT NULL DEFAULT 0,
    status varchar(20) NOT NULL DEFAULT 'uploaded',
    error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CONSTRAINT ai_attachments_kind_valid CHECK (kind IN ('image', 'pdf', 'docx', 'text')),
    CONSTRAINT ai_attachments_status_valid CHECK (status IN ('uploaded', 'parsed', 'failed')),
    CONSTRAINT ai_attachments_size_positive CHECK (size_bytes > 0),
    CONSTRAINT ai_attachments_creator_school_fk FOREIGN KEY (school_id, creator_id) REFERENCES users(school_id, id) ON DELETE RESTRICT
);
```

索引：`(school_id, creator_id, pinned DESC, last_message_at DESC)`、`(school_id, conversation_id, created_at)`、`(school_id, sha256)`。
`000004_ai_chat.down.sql` 逆序删表。

- [ ] **Step 2: 写 Ent schema 并生成**

三个 schema 使用 `TenantMixin`，字段与迁移一一对应。`AIAttachment` 的 `storage_path`、`extracted_text` 不返回 JSON（在 DTO 中单独控制）。

```bash
cd backend
go run entgo.io/ent/cmd/ent generate ./ent/schema
go build ./...
```

Expected: `ent/aiconversation.go`、`ent/aimessage.go`、`ent/aiattachment.go` 及子包生成，编译通过；`git diff` 不应改动既有实体。

- [ ] **Step 3: 写 repository**

实现 CRUD；列表排序为 `pinned DESC, last_message_at DESC, id DESC`；`AppendMessage` 在同一事务内插入消息并把 `message_count + 1`、`last_message_at` 更新到会话。权限规则：

```go
func canReadConversation(principal auth.Principal, entity *ent.AIConversation) bool {
    if principal.Role == auth.RoleAdmin { return entity.SchoolID == principal.SchoolID }
    return entity.CreatorID == principal.UserID
}
func canWriteConversation(principal auth.Principal, entity *ent.AIConversation) bool {
    return entity.CreatorID == principal.UserID
}
```

- [ ] **Step 4: 单测纯映射函数**

`conversation_repository_test.go` 覆盖 `toolCallsToMaps`/`mapsToToolCalls` 往返、`canRead/canWrite` 角色矩阵。不连接数据库。

Run: `go test ./internal/ai -run 'TestConversation|TestToolCallMapping' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/migrations backend/ent backend/internal/ai
git commit -m "feat(ai): add conversation, message and attachment persistence"
```

---

### Task 2: 文档解析与题目切分

**Files:**
- Create: `backend/internal/document/text.go`
- Create: `backend/internal/document/docx.go`
- Create: `backend/internal/document/pdf.go`
- Create: `backend/internal/document/splitter.go`
- Create: `backend/internal/document/text_test.go`
- Create: `backend/internal/document/docx_test.go`
- Create: `backend/internal/document/pdf_test.go`
- Create: `backend/internal/document/splitter_test.go`

**Interfaces:**
- Produces:
  - `document.Extract(name, mime string, data []byte) (Result, error)`
  - `type Result struct { Text string; Kind string; Pages int; Truncated bool; Warnings []string }`
  - `document.SplitQuestions(text string, defaults Defaults) []DraftQuestion`
  - `type DraftQuestion struct { Number string; Content, Answer, Explanation, QuestionType, Category string; Options []Option; Confidence float64; Warnings []string }`

- [ ] **Step 1: 写失败测试（DOCX / 切分器）**

用标准库构造最小 DOCX zip：

```go
func buildDocx(t *testing.T, paragraphs ...string) []byte {
    // word/document.xml: <w:document><w:body><w:p><w:r><w:t>...</w:t></w:r></w:p>...</w:body></w:document>
    // 写入 [Content_Types].xml、_rels/.rels、word/document.xml
}
func TestExtractDocxKeepsParagraphs(t *testing.T) {
    result, err := Extract("exam.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", buildDocx(t, "1. 计算 1+1", "答案：2"))
    if err != nil || !strings.Contains(result.Text, "1. 计算 1+1") || !strings.Contains(result.Text, "答案：2") { t.Fatal(...) }
}
func TestSplitQuestionsRecognizesNumberedItemsAndAnswers(t *testing.T) {
    got := SplitQuestions("一、选择题\n1. 下列说法正确的是（ ）\nA. 甲 B. 乙\n答案：A\n2. 计算 2+2\n答案：4", Defaults{Subject: "数学", Grade: "七年级"})
    if len(got) != 2 || !strings.Contains(got[0].Answer, "A") { t.Fatal(...) }
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/document -v`
Expected: FAIL（包/函数不存在）

- [ ] **Step 3: 实现提取器**

- `docx.go`：`archive/zip` 读取 `word/document.xml`；`encoding/xml` 流式解析 `w:p`、`w:t`、`w:tab`、`w:br`、`w:tbl`；段落用 `\n` 合并；跳过 `w:instrText`。
- `pdf.go`：解析 `%PDF` 对象，支持 `FlateDecode` 与直接流；从内容流识别 `Tj`、`TJ`、`'`、`"`；解析字体的 `ToUnicode` CMap 的 `bfchar`/`bfrange`；按页 `BT/ET` 与 `Td/TD/T*` 断行。提取文本少于 20 字符时返回 `Warnings: ["未提取到可读文本，可能是扫描件，请上传图片或转为图片后 OCR"]`。
- `text.go`：按扩展名/MIME 分派；`.txt/.md/.csv` 直接 UTF-8（失败尝试 GB18030 不可用则不新增依赖，返回明确错误）；超过 200k 字符截断并标记 `Truncated`。
- `splitter.go`：按 `1.`、`1、`、`（1）`、`第1题` 切分；识别 `答案[:：]`、`解析[:：]`、`【答案】`、选项行 `A.`/`A、`；自动识别选择/判断/填空/解答；无编号时按空行分块并在 `Warnings` 提示。

- [ ] **Step 4: 运行测试**

Run: `go test ./internal/document -v`
Expected: PASS（PDF 测试使用内联合成的最小文本 PDF，断言英文与中文映射）

- [ ] **Step 5: Commit**

```bash
git add backend/internal/document
git commit -m "feat(document): extract pdf/docx text and split questions"
```

---

### Task 3: 附件存储与上传 API

**Files:**
- Create: `backend/internal/ai/attachment_store.go`
- Create: `backend/internal/ai/attachment_store_test.go`
- Modify: `backend/internal/platform/config/config.go`
- Modify: `backend/cmd/api/main.go`
- Modify: `.env.example`
- Modify: `docker-compose.yml`
- Modify: `backend/internal/ai/conversation_types.go`

**Interfaces:**
- Consumes: `document.Extract`
- Produces:
  - `type AttachmentStore struct { root string; maxImage, maxDoc int64 }`
  - `NewAttachmentStore(root string) (*AttachmentStore, error)`
  - `store.Save(ctx, schoolID, userID uuid.UUID, name, mime string, data []byte) (StoredFile, error)`
  - `store.Open(storagePath string) (io.ReadCloser, error)`
  - `store.Remove(storagePath string) error`
  - `Config.AIUploadDir string`，环境变量 `AI_UPLOAD_DIR`，默认 `./data/ai-uploads`

- [ ] **Step 1: 写失败测试**

覆盖 PNG magic bytes 接受、`.exe` 拒绝、超限拒绝、文件名清洗（`../`、空名）、sha256 稳定、`kind` 推断。

Run: `go test ./internal/ai -run TestAttachmentStore -v` → FAIL

- [ ] **Step 2: 实现**

- 允许类型：`image/png|jpeg|webp|gif`、`application/pdf`、DOCX MIME、`text/plain|text/markdown|text/csv`。
- 不信任客户端 MIME：以 magic bytes 为准（PNG `89 50 4E 47`、JPEG `FF D8 FF`、PDF `%PDF`、ZIP + `word/` 条目）。
- 存储路径 `{root}/{school_id}/{user_id}/{sha256[:2]}/{uuid}{ext}`，目录 `0750`，文件 `0640`。
- 上传 OCR 图片时调用现有 `ocr.Client.Recognize`；在 `internal/ai` 定义接口避免循环依赖：

```go
type ImageRecognizer interface {
    Recognize(ctx context.Context, image []byte, filename, contentType string) (string, error)
}
```

`Options.OCR ImageRecognizer`、`Options.UploadDir string`。

- [ ] **Step 3: 接线**

`main.go` 在创建 OCR client 后构建 AI Service（调整顺序），传入 `OCR: ocrClient`、`UploadDir: cfg.AIUploadDir`。`docker-compose.yml` 为 API 增加 `ai_uploads` 命名卷。

- [ ] **Step 4: 测试与构建**

Run: `go test ./internal/ai -run TestAttachmentStore -v && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat(ai): store attachments and extract text safely"
```

---

### Task 4: 工具注册表与文档录题工具

**Files:**
- Create: `backend/internal/ai/tools.go`
- Create: `backend/internal/ai/tool_import.go`
- Create: `backend/internal/ai/tools_test.go`
- Modify: `backend/internal/ai/agent.go`（抽取可复用的工具函数，不改变旧行为）

**Interfaces:**
- Produces:
  - `type ToolDescriptor struct { Name, Label, Description string; Parameters json.RawMessage; Writes bool; Scopes []string }`
  - `type ToolContext struct { Principal auth.Principal; ConversationID, RunID string; AttachmentIDs []string }`
  - `type ToolResult struct { Summary string; Data map[string]any; QuestionIDs []string; Warnings []string }`
  - `type Tool struct { Descriptor ToolDescriptor; Run func(context.Context, ToolContext, map[string]any) (ToolResult, error) }`
  - `func (s *Service) ToolCatalog(principal auth.Principal) []ToolDescriptor`
  - `func (s *Service) RunTool(ctx context.Context, principal auth.Principal, tc ToolContext, name string, args map[string]any) (ToolResult, error)`

工具清单（全部注册并带 JSON Schema）：

| name | 写操作 | 说明 |
|---|---|---|
| `search_questions` | 否 | 按关键词/学科/年级/状态/分类检索，最多 20 条 |
| `get_question` | 否 | 题目详情 |
| `bank_audit` | 否 | 题库体检（复用 `AuditBank`） |
| `analyze_question` | 是（草稿） | 分析并可应用补全 |
| `organize_drafts` | 是（草稿） | 补分类/知识点/标签 |
| `fill_explanations` | 是（草稿） | 补答案与解析 |
| `generate_questions` | 是（草稿） | 按知识点出题 |
| `generate_variants` | 是（草稿） | 原题变式 |
| `compose_paper` | 是（草稿试卷） | 智能组卷 |
| `list_attachments` | 否 | 返回本轮附件抽取文本预览 |
| `extract_questions` | 否 | 文档切题预览，不入库 |
| `import_document_questions` | 是（草稿） | 文档题目批量入库 |
| `create_questions` | 是（草稿） | 由模型给定结构化题目批量创建 |

- [ ] **Step 1: 写失败测试**

测试参数校验与 catalog：

```go
func TestToolCatalogCoversAllCapabilities(t *testing.T) {
    names := map[string]bool{}
    for _, item := range NewLocalEngine().ToolCatalogForTest() { names[item.Name] = true }
    for _, want := range []string{"extract_questions", "import_document_questions", "bank_audit", "compose_paper"} { ... }
}
func TestImportQuestionsRejectsPublishedTargets(t *testing.T) { ... }
```

- [ ] **Step 2: 实现注册表**

- 参数解码严格：拒绝未知字段；`limit` 1..50；`count` 1..10；`question_id` UUID；`attachment_ids` 最多 5 个且必须属于当前用户。
- 写工具必须调用 `s.agentAudit` 或等价的 `CreateAudit`，`target_type` 使用 `question`/`paper`/`attachment`。
- `organize_drafts`/`fill_explanations` 复用 `agentTag`/`agentFill`，把 `AgentParams` 从 args 构造。
- `import_document_questions`：读取附件文本 → `document.SplitQuestions` → 逐题构造 `ai.GenerateInput` 走本地/LLM 归一化 → `s.questions.Create` 以草稿创建 → 写审计。返回 `question_ids` 与失败明细。限制每次 ≤ 20 题。
- 教师学科固定：args 中学科与 `principal.Subject` 不一致时直接拒绝。
- `create_questions` 只接受完整结构化字段，复用 `persistGenerated` 的校验逻辑。

- [ ] **Step 3: 运行测试**

Run: `go test ./internal/ai -run 'TestTool|TestImport' -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ai
git commit -m "feat(ai): add auditable agent tool registry and document import"
```

---

### Task 5: 对话运行时（本地规划 + LLM 工具调用）

**Files:**
- Create: `backend/internal/ai/chat_planner.go`
- Create: `backend/internal/ai/chat_planner_test.go`
- Create: `backend/internal/ai/llm_tools.go`
- Create: `backend/internal/ai/llm_tools_test.go`
- Modify: `backend/internal/ai/llm.go`
- Create: `backend/internal/ai/conversation_service.go`
- Create: `backend/internal/ai/conversation_service_test.go`

**Interfaces:**
- Produces:
  - `func (s *Service) CreateConversation(ctx, principal, title string) (Conversation, error)`
  - `func (s *Service) ListConversations(ctx, principal, limit int) ([]Conversation, error)`
  - `func (s *Service) GetConversation(ctx, principal, id uuid.UUID) (Conversation, []ChatMessage, error)`
  - `func (s *Service) SendMessage(ctx, principal, conversationID uuid.UUID, input SendMessageInput) (SendMessageResult, error)`
  - `type SendMessageInput struct { Content string; AttachmentIDs []string; Model string }`
  - `type SendMessageResult struct { Conversation Conversation; UserMessage ChatMessage; AssistantMessage ChatMessage }`
  - `func (s *Service) ParseUploadedAttachment(ctx, principal, id uuid.UUID) (Attachment, error)`

- [ ] **Step 1: 写本地规划器失败测试**

```go
func TestPlannerRoutesDocumentIntent(t *testing.T) {
    plan := planChatTurn("把这份 PDF 里的题导入题库", []AttachmentRef{{ID: "a", Kind: "pdf", Status: "parsed"}}, nil)
    if len(plan.Calls) == 0 || plan.Calls[0].Name != "extract_questions" { t.Fatal(...) }
}
func TestPlannerExplainsUnknownWithoutTools(t *testing.T) { ... }
```

- [ ] **Step 2: 实现本地规划器**

规则顺序：附件意图 → 写操作意图 → 查询意图 → 帮助。最多 3 次工具调用；`import` 前先 `extract_questions` 预览；无附件却要求导入时返回澄清问题而不是报错。回复使用中文，列出工具名、数量、题目 ID（最多 10 个）。

上下文：取最近 6 条消息，仅将用户文本与附件摘要传给规划器；不在工具参数中注入系统提示。

- [ ] **Step 3: 扩展 LLM 客户端支持 tools**

```go
type LLMTool struct { Type string `json:"type"`; Function LLMFunction `json:"function"` }
type LLMMessage struct { Role, Content string; ToolCalls []LLMToolCall; ToolCallID string }
type LLMReply struct { Content string; ToolCalls []LLMToolCall; FinishReason string }
func (c *LLMClient) ChatTools(ctx context.Context, messages []LLMMessage, tools []LLMTool) (LLMReply, error)
```

保持 `CompleteJSON` 行为不变；用 httptest 验证请求体含 `tools` 与 `tool_choice=auto`、响应解析 `tool_calls`。

- [ ] **Step 4: 实现对话编排**

`SendMessage` 流程：

1. 校验会话、内容（文本或附件至少一项）、附件归属与解析状态。
2. 插入 user 消息（`attachments` 为摘要）。
3. 创建 `ai_runs` 记录（`goal` = 内容、`intent` = `chat`），用于审计关联。
4. 运行时：
   - LLM 可用：最多 4 轮 `ChatTools`；工具调用逐个执行，记录 `started_at/finished_at`；工具错误回填给模型；最终无 tool_calls 时使用模型文本。
   - LLM 不可用或失败：本地规划器执行，生成确定性文本；失败时 `provider=local_fallback` 且附 warning。
5. 插入 assistant 消息（含全部 tool_calls 与结果摘要），更新会话标题（首条用户消息前 30 字）与计数。
6. 失败也持久化 assistant 消息（`status=failed`、`error`），保证历史完整。

- [ ] **Step 5: 运行测试**

Run: `go test ./internal/ai -run 'TestPlanner|TestChatTools|TestSendMessageValidation' -v`
Expected: PASS（不依赖数据库的部分）

- [ ] **Step 6: Commit**

```bash
git commit -am "feat(ai): add conversational agent runtime with tool calling"
```

---

### Task 6: HTTP API、技能与工具目录

**Files:**
- Create: `backend/internal/ai/conversation_handler.go`
- Create: `backend/internal/ai/skills.go`
- Create: `backend/internal/ai/conversation_handler_test.go`
- Modify: `backend/internal/ai/handler.go`（注册新路由）
- Modify: `backend/internal/ai/types.go`（`IntentChat = "chat"`）
- Modify: `docs/api.md`

**Interfaces:**
- Routes:
  - `GET/POST /api/v1/ai/conversations`
  - `GET/PATCH/DELETE /api/v1/ai/conversations/:id`
  - `GET /api/v1/ai/conversations/:id/messages`
  - `POST /api/v1/ai/conversations/:id/messages`
  - `POST /api/v1/ai/attachments`（multipart `file`，可选 `conversation_id`）
  - `GET /api/v1/ai/attachments/:id`、`GET /api/v1/ai/attachments/:id/content`
  - `GET /api/v1/ai/tools`、`GET /api/v1/ai/skills`

- [ ] **Step 1: 写 handler 路由测试**

用 `echo.New()` + 伪造 principal 中间件，断言未认证 401、非法 UUID 400、未知字段 400、列表默认 limit 30。

- [ ] **Step 2: 实现 handler**

- 上传使用 `multipart`，先 `http.MaxBytesReader` 限制 21 MiB，再读文件；`Content-Disposition` 用 `inline`，设置 `X-Content-Type-Options: nosniff`、`Cache-Control: private, max-age=300`。
- 附件内容端点仅允许本人（管理员可读本校）访问。
- 错误映射复用 `aiError`，新增 `ErrConflict`（会话归档时不可写）。
- 技能目录：

```go
type Skill struct {
    ID, Title, Description, Icon, Prompt string
    RequiredTools []string
    AcceptsAttachments bool
}
```

内置 6 个：`document_import`、`paper_split`、`bank_cleanup`、`lesson_practice`、`final_review`、`variant_pack`。

- [ ] **Step 3: 验证**

Run: `go test ./internal/ai -run TestConversationHandler -v && go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git commit -am "feat(ai): expose conversation, attachment, tool and skill APIs"
```

---

### Task 7: 前端对话工作台

**Files:**
- Create: `frontend/lib/ai-chat.ts`
- Create: `frontend/lib/ai-chat.test.ts`
- Create: `frontend/components/ai/chat/agent-chat-workspace.tsx`
- Create: `frontend/components/ai/chat/conversation-sidebar.tsx`
- Create: `frontend/components/ai/chat/message-list.tsx`
- Create: `frontend/components/ai/chat/message-bubble.tsx`
- Create: `frontend/components/ai/chat/tool-call-timeline.tsx`
- Create: `frontend/components/ai/chat/composer.tsx`
- Create: `frontend/components/ai/chat/attachment-tray.tsx`
- Create: `frontend/components/ai/chat/skills-panel.tsx`
- Create: `frontend/components/ai/chat/attachment-image.tsx`
- Modify: `frontend/app/ai/page.tsx`
- Modify: `frontend/app/teacher/ai/page.tsx`
- Modify: `frontend/app/admin/ai/page.tsx`
- Modify: `frontend/components/mobile-navigation.tsx`

**Interfaces:**
- Consumes: Task 6 API
- Produces: `AgentChatWorkspace({ user })`

- [ ] **Step 1: 写 Zod schema 与失败测试**

覆盖：会话字段、tool call 状态、附件 kind、未知工具参数容忍、`formatToolStatus`、`attachmentAccept`。测试先失败。

- [ ] **Step 2: 实现 `lib/ai-chat.ts`**

```ts
export const toolCallSchema = z.object({
  id: z.string(), name: z.string(), label: z.string(),
  status: z.enum(["running", "succeeded", "failed", "skipped"]),
  summary: z.string().default(""), error: z.string().optional(),
  arguments: z.record(z.string(), z.unknown()).default({}),
  result: z.record(z.string(), z.unknown()).optional(),
});
export const chatMessageSchema = z.object({
  id: z.string(), conversation_id: z.string(),
  role: z.enum(["user", "assistant", "tool", "system"]),
  content: z.string(), status: z.string(),
  tool_calls: z.array(toolCallSchema).default([]),
  attachments: z.array(attachmentRefSchema).default([]),
  provider: z.string().default("local"), model: z.string().default(""),
  created_at: z.string(),
});
```

API：`listConversations`、`createConversation`、`updateConversation`、`deleteConversation`、`listMessages`、`sendMessage`、`uploadAttachment`、`fetchAttachmentBlob`、`listTools`、`listSkills`。上传用 `FormData`。

- [ ] **Step 3: 实现 UI**

布局：

```text
lg: [会话列表 260px] [消息流 minmax(0,1fr)] [技能/工具 300px]
md: 会话列表折叠为顶部按钮；右栏收进 Sheet/Dialog
```

细节：
- 空状态展示 6 个技能卡片，点击填入 composer 的 prompt；附件型技能自动打开文件选择。
- 用户消息右侧气泡，附图片缩略图与文件 chip；助手消息左侧无气泡边框，工具调用以时间线卡片嵌入。
- 工具卡片：图标按工具类别、状态点、时长、摘要；`<details>` 展开 JSON（`JSON.stringify` 截断 8k）；题目 ID 渲染为 `/teacher/questions?focus=<id>` 或管理员题库链接。
- 发送中：禁用输入、显示 assistant 占位消息；失败显示重试按钮，保留输入内容。
- 附件：拖拽与粘贴图片；上传前本地校验大小/数量；上传中进度占位；失败可移除重试。
- 会话列表：新建、按标题过滤、重命名（内联）、删除（确认）、置顶；显示最后消息时间与消息数。
- 图片预览：`fetch` 带 credentials 拿 Blob 后 `createObjectURL`，卸载时 revoke；失败显示文件名。
- 模型选择复用 `/ai/status`；显示 `provider=local` 徽标。
- 无障碍：`aria-live="polite"` 消息流、工具卡片 `aria-expanded`、焦点管理、`prefers-reduced-motion`。

- [ ] **Step 4: 路由与导航**

三个页面渲染同一组件；`/ai` 用当前用户角色决定 scope，管理员额外显示审计日志入口。导航文案改为「AI 助手」，保留 `/ai` 路径。

- [ ] **Step 5: 校验**

Run: `pnpm test --run lib/ai-chat.test.ts && pnpm typecheck && pnpm lint`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add frontend
git commit -m "feat(ai): replace workspace with conversational agent UI"
```

---

### Task 8: 文档、配置、回归与推送

**Files:**
- Modify: `README.md`
- Modify: `docs/api.md`
- Modify: `docs/deployment.md`
- Modify: `.env.example`
- Modify: `docker-compose.yml`
- Create: `docs/ai-agent-tools.md`（工具/技能契约、外部 agent 接入说明）

- [ ] **Step 1: 更新文档**

- README 增加「AI Agent 对话工作台」章节：会话、附件、工具调用、文档录题、技能、本地降级、`AI_UPLOAD_DIR`。
- `docs/api.md` 增加所有新端点的请求/响应示例与权限表。
- `docs/ai-agent-tools.md` 列出工具名、参数 JSON Schema、写操作审计动作、技能映射；说明可通过 `GET /api/v1/ai/tools` 让外部 Agent 发现能力。
- `docs/deployment.md` 说明上传目录持久化与备份、反向代理 `client_max_body_size 25m`。

- [ ] **Step 2: 全量回归**

```bash
cd backend && export PATH=$PATH:/usr/local/go/bin
go build ./... && go vet ./... && go test ./internal/document ./internal/ai ./internal/question ./internal/importer
cd ../frontend && pnpm lint && pnpm typecheck && pnpm test --run && pnpm build
```

Expected: 全部 PASS；无 TEST_DATABASE_URL 的集成测试如因环境跳过，记录在交付说明中。

- [ ] **Step 3: 手工冒烟（可用环境）**

启动 Postgres/Redis/OCR 后：
1. 新建会话 → 发送「体检题库」→ 看到工具卡片与结果。
2. 上传 DOCX → 发送「把这些题导入题库」→ 返回题目 ID，题库出现草稿。
3. 上传图片 → OCR 文本出现在附件详情。
4. 刷新后会话历史、工具调用完整恢复。
5. 发布题在整理操作中保持只读。

- [ ] **Step 4: 推送 GitHub**

当前环境 `github.com:443` 不可达（`api.github.com` 与 `codeload.github.com` 可达）。推送使用 GitHub Git Data API，基于远端 `main` 当前 commit 创建新 tree/commit 并更新 ref，保留历史：

1. `GET /repos/CodeSky0/school-exam-studio/git/ref/heads/main`
2. `POST /git/blobs` 上传每个新增/修改文件
3. `POST /git/trees`（`base_tree` = 远端 tree；删除文件用 `sha:null`）
4. `POST /git/commits`（`parents=[远端 head]`）
5. `PATCH /git/refs/heads/main`

推送后 `GET /commits/{sha}` 校验文件数量与关键文件内容。

- [ ] **Step 5: 完成**

在最终回复中提供：提交 SHA、变更摘要、验证结果、未覆盖项（如需要真实模型/扫描 PDF 的能力边界）。

---

## Self-Review

- 规格覆盖：历史会话（Task 1/5/6/7）、发图片文件（Task 2/3/6/7）、工具调用模块（Task 4/5/7）、PDF/Word 解析入题库（Task 2/4）、AI 工具与 Skill（Task 4/6/7/8）、推送（Task 8）均有任务。
- 类型一致性：`AttachmentRef`、`ToolCall`、`ChatMessage` 在 Task 1 定义，Task 4/5/6/7 只引用不重命名。
- 无占位符：每个任务包含文件、接口、实现要点与验证命令。