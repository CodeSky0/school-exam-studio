# School Exam Studio 设计规格

日期：2026-11-10
状态：已确认
仓库可见性：GitHub 私有仓库 `school-exam-studio`

## 1. 目标与范围

交付一套真实可运行的中小学智能题库与组卷系统，服务学校管理员和学科教师。核心能力包括学校初始化、JWT 认证、题目管理、本地数学 OCR、组卷、A4 打印、教师管理、题目审核和 CSV 全量事务导入。

采用单学校部署：一套实例只允许成功初始化一次，之后关闭初始化写入接口。所有业务数据仍通过 `school_id` 隔离，并使用第二所学校夹具进行越权测试。

不交付静态演示、前端假数据或 Node.js 核心后端。PostgreSQL 是唯一业务持久化存储；Redis 仅用于限流和短期缓存。原题图片不持久化，OCR 结果必须由教师校对后显式保存。

## 2. 技术架构

### 2.1 技术选型

- 前端：Next.js 16 App Router、React 19、TypeScript、Tailwind CSS v4、shadcn/ui、TanStack Query、Zustand、Zod、TipTap、KaTeX。
- 核心后端：Go、Echo、Ent，按 Handler、Service、Repository 划分模块边界。
- 数据：PostgreSQL、Ent 类型安全模型、版本化 SQL 迁移、Go 迁移命令。
- OCR：独立 Python FastAPI sidecar 调用本地 Pix2Text，不承担题库核心业务。
- 测试：Go testing、Vitest、Playwright，以及 OCR 适配层测试。
- 基础设施：Docker Compose 启动 PostgreSQL、Redis 和 OCR。

### 2.2 项目结构

```text
frontend/
  app/                     公共、管理员、教师路由
  components/              题目编辑器、筛选器、组卷购物车、A4 预览
  lib/                     API 客户端、Zod Schema、安全公式渲染
  stores/                  Zustand 组卷状态
backend/
  cmd/api/                 Echo 服务入口
  cmd/migrate/             Go 数据库迁移入口
  ent/schema/              Ent 实体、索引、软删除与权限策略
  internal/
    auth/                  初始化、注册、登录、JWT
    question/              题目管理
    paper/                 试卷、题目排序和分值
    admin/                 教师管理、题目审核、学校设置
    importer/              CSV 解析、校验和事务导入
    ocr/                   本地 OCR 客户端和资源限制
    platform/              数据库、Redis、中间件、错误响应
  migrations/              版本化 PostgreSQL SQL 迁移
ocr/
  app/main.py              FastAPI 适配层
  requirements.txt
  Dockerfile
docs/
  superpowers/specs/
docker-compose.yml
```

### 2.3 页面与布局

公共路由为 `/login`、`/register`、`/init`。管理员路由为 `/admin/dashboard`、`/admin/teachers`、`/admin/questions`、`/admin/settings`。教师路由为 `/teacher/dashboard`、`/teacher/questions`、`/teacher/papers`、`/teacher/profile`。

桌面端使用 240px 固定侧栏、64px 顶部工具栏和高密度主内容区，主色为学术蓝 `#2563eb`。总览数字、列表和状态均来自真实 API。学校设置允许管理员修改学校名称，8 位学校代码只读。个人中心支持查看资料和修改密码。

## 3. 数据设计

### 3.1 通用约束

`schools.id` 自身是学校标识，不重复设置 `school_id`。其余业务表均包含 UUID `id`、UUID `school_id`、`created_at`、`updated_at` 和可空 `deleted_at`，包括 `paper_questions`。

Ent 全局软删除拦截器自动排除已删除记录。需要访问归档数据时必须使用显式、受权限保护的管理查询。数据库迁移创建必要 UUID 扩展，ID 由 PostgreSQL 生成。

### 3.2 核心实体

- `schools`：名称、唯一 8 位数字字符串 `school_code`。
- `users`：学校、角色、姓名、学科、规范化邮箱、密码哈希、状态和内部会话版本。邮箱在学校内唯一。角色为 `admin` 或 `teacher`，状态为 `active` 或 `disabled`。
- `questions`：学校、创建人、学科、年级、题型、1–5 难度、JSONB 标签、受限 HTML 题干、JSONB 选项、答案、解析和状态。状态为 `draft`、`published` 或 `archived`。
- `papers`：学校、创建人、标题、学科、年级、两位小数总分和考试分钟数。
- `paper_questions`：学校、试卷、题目、顺序和两位小数分配分值。

标签采用字符串数组，选项采用 `[{key, content}]`。题型内部值为 `single_choice`、`multiple_choice`、`true_false`、`fill_blank`、`essay`，界面和 CSV 显示中文名称。

分值使用 PostgreSQL `NUMERIC(8,2)`，禁止二进制浮点直接汇总。`paper_questions` 对 `(paper_id, order_index)` 和 `(paper_id, question_id)` 建唯一约束。通过包含 `school_id` 的复合外键及服务层事务校验，阻止试卷与题目跨学校关联。

### 3.3 索引

至少包含：

- 题目 `(school_id, subject, grade, question_type)`。
- 题目 `(school_id, creator_id, status)`。
- 标签 GIN 索引。
- 试卷 `(school_id, creator_id, updated_at)`。
- 试卷题目 `(school_id, paper_id, order_index)`。

列表采用稳定排序和分页，单页最多 100 条。

## 4. 认证与权限

### 4.1 初始化与登录

初始化接口在 PostgreSQL 事务内使用咨询锁。并发请求也只能创建一所学校及首个管理员，重复初始化返回 409。学校代码由安全随机数生成，并处理极低概率的唯一冲突。

教师凭有效学校代码自助注册并直接启用。密码使用 bcrypt 哈希。登录失败返回不区分邮箱或密码原因的通用错误，并按 IP 与账号组合限流。

JWT 至少包含 `user_id`、`school_id`、`role` 和会话版本，放入 `HttpOnly`、`SameSite=Strict` Cookie；生产环境设置 `Secure`。后端校验修改请求的 Origin。认证中间件每次检查当前用户状态、角色和会话版本，禁用或重置密码后旧 Token 不可继续使用。

`school_id` 只能来自认证上下文，客户端传入的学校字段一律忽略或拒绝。跨学校资源访问返回 404，角色不足返回 403。

### 4.2 题目与试卷

- 教师可查看本校自己的全部题目，以及其他创建人的已发布题目。
- 教师只能创建草稿，编辑和软删除自己的草稿，不能直接发布或归档。
- 管理员可查看全校题目，并执行发布、归档和审核筛选。
- 只有已发布题目可加入试卷，防止后续草稿修改影响试卷。
- 教师只能查看和管理自己的试卷；管理员可管理本校全部试卷。
- 保存试卷在一个事务内验证学校、题目状态、唯一性、顺序和分值，替换明细并重新计算总分。
- 管理员可筛选教师、启用或禁用教师、重置密码。重置密码生成只显示一次的安全随机密码，并递增会话版本。

## 5. 富文本与本地 OCR

### 5.1 编辑器与公式

TipTap 数学节点保存 LaTeX 属性，KaTeX 负责屏幕和打印渲染。数据库保存受限 HTML 与 JSONB，不信任客户端提交的标签或属性。

后端使用白名单消毒所有富文本字段。前端不直接渲染 OCR 返回的原始 HTML；OCR 文本按 `$...$` 和 `$$...$$` 转换为编辑器文本和公式节点。KaTeX 禁止不受信任的 HTML 指令。所有外部内容均按不可信输入处理。

### 5.2 OCR 流程

1. 教师或管理员上传 PNG、JPEG 或 WebP 单题截图。
2. Go 限制文件最大 5 MB，校验真实 MIME、解码结果和像素上限。
3. Go 通过内网或回环地址调用 Pix2Text sidecar，使用有界队列、并发限制和超时。
4. sidecar 只在内存中处理图片，返回包含普通文本及 LaTeX 定界符的识别文本。
5. 前端显示可编辑结果，教师分别校对题干、选项、答案和解析。
6. 用户显式保存后才调用正常草稿创建接口，OCR 接口本身不创建数据库记录。

模型文件使用独立 Docker 缓存卷，不属于业务数据。原图不写数据库或磁盘。OCR 未就绪、超时、空结果或识别失败均返回明确错误，不生成模拟结果。

## 6. CSV 全量事务导入

模板表头固定为：

```text
题型,年级,学科,难度,知识点标签,题干,选项A,选项B,选项C,选项D,答案,解析
```

文件使用 UTF-8，可带 BOM；标签以英文分号分隔；题干、答案和解析可包含 LaTeX 定界符。单文件最大 5 MB、1,000 行。

校验接口只解析和校验，不写入数据库，返回有效行数、最多 50 行预览及带行号的全部校验错误。导入接口重新读取原文件并完整校验，避免预览后被篡改。

任何一行错误都返回 HTTP 422，数据库零写入。全部有效时才在一个 PostgreSQL 事务中批量创建当前教师、当前学校的草稿；任一批量写入失败均回滚。选择题校验选项和答案，难度限制为 1–5，CSV 学科必须与当前教师学科一致。

## 7. 组卷与打印

组卷页左侧提供学科、年级、题型、难度、标签、创建人筛选和分页，右侧为拖拽试卷购物车。题目顺序和分值可编辑，Zustand 状态按用户命名空间持久化。恢复购物车时重新验证题目仍属于本校且处于已发布状态。

前端 Zod 校验请求与响应。分值以两位小数字符串传输，前端转成整数百分计算总分，后端使用定点数复核。

试卷预览包含学校名称、试卷标题、学科、年级、时长、总分、密封线、大题编号和答题留白。打印采用 `@page size: A4` 与明确页边距，`@media print` 隐藏侧栏、工具栏、筛选器和按钮，只保留试卷内容。题目尽量避免跨页断裂，公式和中文使用可打印字体。PDF 导出优先使用浏览器原生打印，不增加伪 PDF 接口。

## 8. API 与错误处理

API 前缀为 `/api/v1`，主要资源包括：

- 初始化状态与初始化、注册、登录、退出、当前用户、修改密码。
- 题目列表、详情、创建、修改、软删除和审核状态。
- OCR 识别、CSV 校验和 CSV 导入。
- 试卷列表、详情、创建、修改和软删除。
- 管理员教师列表、状态修改、重置密码，以及学校设置。

统一错误格式：

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "请求内容未通过校验",
    "details": []
  },
  "request_id": "..."
}
```

使用 400、401、403、404、409、413、415、422、429、503、504 和 500 表达不同失败。客户端不接收 SQL、堆栈、内部路径或密钥。日志记录请求 ID 和必要诊断信息，不记录密码、Token、认证头或完整题目图片。

## 9. 测试与验收

### 9.1 后端

Go 单元测试覆盖鉴权、权限判断、CSV 解析、题型校验和分值计算。Ent 集成测试必须使用独立 PostgreSQL 测试库，不使用 SQLite 替代。

关键用例：

- 并发初始化只成功一次。
- 第二所学校夹具无法读取、修改或关联第一所学校的数据。
- 禁用用户和重置密码使旧会话失效。
- 100 行合法 CSV 全部入库；含一行错误时零写入；写入错误回滚。
- 非发布题目不能组卷，跨学校题目不能组卷。
- 试卷顺序、单项分值和总分在事务中一致保存。

### 9.2 前端与端到端

Vitest 覆盖 Zod Schema、筛选序列化、购物车恢复和整数百分计分。Playwright 覆盖初始化、注册登录、录题、审核、组卷、教师管理和打印预览。

浏览器打印测试验证 A4 页面尺寸、打印时 UI 隐藏和题目分页规则。OCR 适配层测试输入限制及错误响应，并单独执行真实 Pix2Text 模型冒烟测试；模拟 OCR 仅可用于单元测试，不能替代真实模型验收。

交付前运行 `go test ./...`、`go vet ./...`、前端 lint、typecheck、Vitest、生产构建及可运行的端到端测试。任何未执行或受环境阻塞的验证必须明确报告。

## 10. 启动与交付

`docker compose up -d` 启动 PostgreSQL、Redis 和 OCR。宿主机后端默认通过 `http://localhost:8080` 提供服务，OCR 通过仅绑定回环地址的端口供宿主机调用；前端运行于 `http://localhost:3000`。

提供 `.env.example`、Docker Compose、Go 迁移命令、CSV 模板、完整 README 和故障排查说明。README 不设置弱密码默认账号，明确引导用户通过 `/init` 创建首个管理员。

GitHub 使用私有仓库 `school-exam-studio`。按设计、基础认证、题目与 OCR、组卷打印、管理与导入、测试交付分别提交，验证通过后推送。任何聊天中出现过的访问令牌均不得写入文件、命令、Git 历史或文档；推送仅使用本机安全的 `gh` 授权。
