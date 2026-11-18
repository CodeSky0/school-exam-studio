# API 说明

- 前缀：`/api/v1`
- 认证：登录成功后签发 HS256 JWT（2 小时过期），写入 `HttpOnly`、`SameSite=Strict` Cookie；生产环境带 `Secure`。声明包含 `user_id`、`school_id`、`role`、`session_version`。
- 租户与角色只能来自认证上下文，客户端传入的学校字段一律忽略；跨学校资源返回 404，角色不足返回 403。
- 所有修改请求必须携带与 `CORS_ORIGIN` 一致的 `Origin` 头。
- 登录与注册按 IP 与账号组合限流（Redis 滑动窗口）。

## 统一错误格式

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

状态码：400、401、403、404、409、413、415、422、429、503、504、500。客户端不会收到 SQL、堆栈、内部路径或密钥。

## 初始化与认证

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/init/status` | 公开 | 返回 `{"initialized": boolean}` |
| POST | `/init` | 公开（仅一次） | 创建学校与管理员，事务内咨询锁保证只成功一次；重复返回 409；响应仅展示一次 `school_code`（8 位数字） |
| POST | `/auth/register` | 公开 | 凭学校代码注册 active 教师，邮箱校内唯一 |
| POST | `/auth/login` | 公开 | 错误邮箱/密码返回相同通用错误；成功设置认证 Cookie |
| POST | `/auth/logout` | 登录 | 退出登录 |
| GET | `/auth/me` | 登录 | 当前用户；禁用或会话版本变化返回 401 |
| POST | `/auth/change-password` | 登录 | 修改密码并递增会话版本 |

## 题目

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/questions` | 教师/管理员 | 组合筛选：`subject`、`grade`、`question_type`、`category`、`knowledge_point`、`tag`、`creator_id`、`status`、`difficulty`、`page`、`page_size`（≤100）。教师见自己全部状态及他人 published，管理员见本校全部 |
| POST | `/questions` | 教师 | 创建草稿，`creator_id` 强制为当前用户，学科须与教师一致；富文本白名单消毒 |
| POST | `/questions/resolve` | 教师/管理员 | 按 ID 数组批量返回本校已发布题目，供购物车恢复校验 |
| GET | `/questions/:id` | 教师/管理员 | 按 ID + 学校过滤 |
| PATCH | `/questions/:id` | 教师（本人草稿） | 更新草稿 |
| DELETE | `/questions/:id` | 教师（本人草稿） | 软删除 |
| PATCH | `/admin/questions/:id/status` | 管理员 | 仅允许 draft→published、draft/published→archived |

题型：`single_choice`、`multiple_choice`、`true_false`、`fill_blank`、`essay`。状态：`draft`、`published`、`archived`。

题目新增字段：`category`（分类）、`knowledge_points`（知识点数组）、`ai_generated`、`ai_meta`、`content_hash`、`source_question_id`。`category`、`knowledge_points` 可由教师编辑，也可由 AI 建议写回草稿。

## OCR

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/ocr/recognize` | 教师/管理员 | multipart 字段 `image`，PNG/JPEG/WebP，最大 5 MB、2000 万像素；返回 `{"text":"已知 $x^2=4$"}`，不写数据库。上游失败映射 502/503/504 |

OCR 结果必须由教师校对后通过正常题目接口保存，原图不持久化。

## CSV 导入

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | `/questions/import/validate` | 教师 | 只解析校验，返回有效行数、最多 50 行预览与带真实行号的错误，不写库 |
| POST | `/questions/import` | 教师 | 重新读取原文件并完整校验，单事务导入当前学校、当前教师的草稿；任一行失败返回 422 且零写入 |

限制：5 MB、1,000 数据行、固定中文表头、学科与当前教师一致、难度 1–5、选择题校验选项与答案。

## 试卷

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/papers` | 教师（本人）/管理员（本校） | 分页列表；每项包含 `question_count`，不含题目明细 |
| POST | `/papers` | 教师 | 事务内逐题校验学校与 published 状态、学科年级一致、题目不重复，替换明细并汇总总分 |
| GET | `/papers/:id` | 教师（本人）/管理员（本校） | 返回卷面信息与按 `order_index` 排序的题目明细 |
| PATCH | `/papers/:id` | 教师（本人）/管理员（本校） | 同一事务更新 |
| DELETE | `/papers/:id` | 教师（本人）/管理员（本校） | 软删除 |

分值为两位小数字符串，数据库 `NUMERIC(8,2)`；后端定点复核总分，前端使用整数百分计算（0.10 + 0.20 = 0.30）。

## 管理与统计

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/admin/teachers` | 管理员 | 按学科、状态筛选教师 |
| PATCH | `/admin/teachers/:id/status` | 管理员 | 启用/禁用，禁用立即使旧会话失效 |
| POST | `/admin/teachers/:id/reset-password` | 管理员 | 生成至少 16 位随机临时密码并递增会话版本，仅响应一次 |
| GET | `/admin/school` | 管理员 | 学校名称与只读学校代码 |
| PATCH | `/admin/school` | 管理员 | 修改学校名称 |
| GET | `/admin/stats` | 管理员 | 本校教师、题目、试卷统计 |
| GET | `/teacher/stats` | 教师 | 当前教师题目、试卷统计 |

## AI

所有 AI 接口要求登录。管理员可在“学校设置”中配置全校共享模型、厂商接口与 API Key，并选择启用模型；学校配置优先于服务器环境变量。未配置 `AI_API_KEY` 或管理员停用时使用内置确定性本地引擎；大模型调用失败会自动降级并在 `warnings` 中提示厂商返回的错误原因。API Key 加密存储，接口只返回掩码提示。兼容 `message.content` 为字符串、分片数组或仅返回推理内容的响应。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/admin/ai/settings` | 管理员 | 读取全校共享模型配置、已启用模型与密钥掩码 |
| PATCH | `/admin/ai/settings` | 管理员 | 更新厂商、接口地址、API Key、启用模型与默认模型；`api_key` 留空保留原密钥，`clear_api_key` 为 true 时清除 |
| POST | `/admin/ai/test` | 管理员 | 用一次最小请求测试已保存配置；可传 `{"model":"..."}` 指定模型，返回是否可用、耗时与厂商原始错误 |
| GET | `/ai/status` | 登录 | 返回当前 provider、模型、可用模型列表、来源、单次限制、能力与安全边界 |
| POST | `/ai/analyze` | 登录 | 分析题干，返回分类、知识点、标签、难度、答案与解析建议；不写库 |
| POST | `/ai/questions/:id/analyze` | 教师/管理员 | 分析指定题目；`{"apply":true}` 时按权限写回草稿元数据 |
| POST | `/ai/questions/:id/variants` | 教师/管理员 | 生成题目变式，`strategy` 支持 `numbers`、`context`、`reverse`、`difficulty`；`persist` 为 true 时保存为草稿 |
| POST | `/ai/generate` | 登录 | 按学科、年级、分类、知识点、标签生成新题；`persist` 为 true 时保存为草稿 |
| GET | `/ai/bank/audit?limit=500` | 登录 | 题库体检：缺答案/解析/知识点/分类、重复题与分布统计 |
| POST | `/ai/agent/runs` | 登录 | 提交自然语言目标，Agent 完成意图识别、计划与执行，返回运行报告 |
| GET | `/ai/agent/runs?limit=20` | 登录 | 最近运行记录；教师仅见自己的运行 |
| GET | `/ai/agent/runs/:id` | 教师/管理员 | 运行详情；教师仅可读自己的运行 |
| GET | `/ai/audit-logs?limit=50` | 管理员 | 本校 AI 写入审计日志 |

### AI Agent 对话

对话工作台使用会话-消息模型。会话、消息与工具调用写入 PostgreSQL；附件二进制存放在 `AI_UPLOAD_DIR`，数据库保存元数据与抽取文本。

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/ai/conversations?limit=30` | 教师/管理员 | 会话列表，按置顶与最后消息时间排序；教师仅见自己的会话，管理员可见本校会话 |
| POST | `/ai/conversations` | 教师/管理员 | 新建会话，可选 `{"title":"..."}`；创建者强制为当前用户 |
| GET | `/ai/conversations/:id` | 教师/管理员 | 返回 `{"conversation":..., "messages":[...]}`；管理员可读本校会话，教师仅限本人 |
| PATCH | `/ai/conversations/:id` | 创建者 | 更新 `title` 与/或 `pinned`；管理员也不能修改他人会话 |
| DELETE | `/ai/conversations/:id` | 创建者 | 软删除会话，题库中已生成的草稿不受影响 |
| GET | `/ai/conversations/:id/messages?limit=100` | 教师/管理员 | 消息历史，按时间升序，单页最多 200 条 |
| POST | `/ai/conversations/:id/messages` | 创建者 | 发送消息并同步执行 Agent，返回用户消息、助手消息与更新后的会话 |
| POST | `/ai/conversations/:id/messages/stream` | 创建者 | 同一条消息以 SSE 推送：用户消息、工具开始/结束、助手消息与完成事件 |
| POST | `/ai/attachments` | 教师/管理员 | multipart 字段 `file`，可选 `conversation_id`；上传后同步解析文本 |
| GET | `/ai/attachments/:id` | 上传者/管理员 | 附件摘要：类型、状态、文本预览、字符数 |
| GET | `/ai/attachments/:id/content` | 上传者/管理员 | 原始文件流，`inline` 展示，仅浏览器会话可访问 |
| GET | `/ai/tools` | 教师/管理员 | 工具目录：名称、标签、说明、JSON Schema、是否写操作 |
| GET | `/ai/skills` | 教师/管理员 | 内置技能目录：提示词、所需工具、是否接受附件 |

发送消息请求：

```json
{
  "content": "把这份试卷按八年级数学导入题库",
  "attachment_ids": ["5f8e..."],
  "model": ""
}
```

流式接口返回 `text/event-stream`，每帧格式为 `event: <type>` + `data: <json>`，`json.type` 与事件名一致：

| 事件 | 负载 | 说明 |
|---|---|---|
| `user_message` | `message` | 用户消息落库后立即推送 |
| `assistant_delta` | `content` | 模型正文 Token 增量，前端按序拼接 |
| `reasoning_delta` | `reasoning` | 模型推理增量（模型支持时返回） |
| `tool_started` | `tool_call` | 工具开始执行，状态为 `running` |
| `tool_finished` | `tool_call` | 工具结束，含耗时、摘要、参数与结果 |
| `assistant_message` | `message` + `conversation` | 本轮助手回复与更新后的会话 |
| `done` | — | 正常结束 |
| `error` | `error` | 处理失败，连接随后关闭 |

示例：

```
event: assistant_delta
data: {"type":"assistant_delta","content":"我"}

event: reasoning_delta
data: {"type":"reasoning_delta","reasoning":"先检索题库"}

event: tool_started
data: {"type":"tool_started","tool_call":{"id":"t1","name":"search_questions","label":"检索题目","status":"running","arguments":{"keyword":"函数"}}}

event: tool_finished
data: {"type":"tool_finished","tool_call":{"id":"t1","name":"search_questions","label":"检索题目","status":"succeeded","summary":"找到 8 道题","arguments":{"keyword":"函数"}}}
```

消息响应中的 `tool_calls` 是完整工具时间线：

```json
{
  "id": "msg_...",
  "role": "assistant",
  "content": "已完成：从文档切分 12 道题并导入 10 道草稿。",
  "provider": "local",
  "tool_calls": [
    {
      "name": "extract_questions",
      "label": "文档切题",
      "status": "succeeded",
      "arguments": {"attachment_ids": ["5f8e..."]},
      "summary": "已从文档切分 12 道题目，等待确认后入库",
      "result": {"data": {"total": 12}, "question_ids": []},
      "started_at": "2026-11-17T09:00:00Z",
      "finished_at": "2026-11-17T09:00:00.120Z"
    }
  ]
}
```

附件限制：每轮最多 5 个；图片 ≤10 MiB，文档 ≤20 MiB；仅接受 PNG/JPEG/WebP/GIF、PDF、DOCX、TXT、MD、CSV。解析状态为 `uploaded`、`parsed`、`failed`；失败原因在 `error` 中返回。图片不做 OCR，上传后直接标记 `parsed`，发送消息时以多模态输入交给模型。扫描版 PDF 没有文本层时仍为 `parsed`（`text_chars=0`），可改传页面图片由模型直接阅读。

Agent 运行策略：

1. 模型可用时使用 OpenAI 兼容 function calling，最多 4 轮、8 次工具调用；工具结果回填模型后再生成最终回复。
2. 模型不可用时使用确定性本地规划器，规则覆盖文档导入、体检、整理、补解析、出题、变式、组卷、检索。
3. 模型在工具执行后中断时保留已执行结果，不回退重跑，避免重复写入。
4. 所有写工具只作用于草稿；教师学科与调用参数不一致时拒绝执行。

Agent 支持的目标：题库体检、分类与知识点整理、补答案与解析、按知识点出题、生成变式、智能组卷、分析指定题目。Agent 工具单次最多处理 10 道题；AI 只自动修改草稿，已发布题目只读；教师只能操作自己的草稿；所有写入记录审计日志并标记待人工复核。

单次限制：分析题干 20,000 字符、一次生成 10 道题、批量处理 10 道题。

## 健康检查

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 进程存活，返回 `{"status":"ok"}` |
| GET | `/readyz` | 数据库未就绪返回 503 |

OCR sidecar 独立提供 `/healthz`、`/readyz`（模型加载完成才返回 200）与 `POST /v1/recognize`。
