# AI Agent 工具与技能契约

本文档描述对话工作台暴露给内置 Agent 与外部调用方的工具、技能、权限边界与返回结构。运行中的服务通过以下接口提供同一份机器可读契约：

- `GET /api/v1/ai/tools`：工具名称、标签、说明、JSON Schema、是否为写操作。
- `GET /api/v1/ai/skills`：内置技能、提示词、所需工具、是否接受附件。

所有接口要求登录；请求必须来自配置的 `CORS_ORIGIN`，并携带认证 Cookie。

## 1. 工具

| 工具 | 写操作 | 关键参数 | 审计动作 | 说明 |
|---|---|---|---|---|
| `search_questions` | 否 | `keyword`、`subject`、`grade`、`status`、`category`、`knowledge_point`、`limit` | — | 检索题库，返回 ID、摘要与元数据，最多 50 条 |
| `get_question` | 否 | `question_id` | — | 读取单题完整快照 |
| `bank_audit` | 否 | `limit` | — | 缺答案/解析/知识点/分类/标签、重复题与分布统计 |
| `analyze_question` | 是 | `question_id`、`apply`、`model` | `agent_tool_analyze_question` | 分析题目；仅草稿可写回 |
| `organize_drafts` | 是 | `subject`、`grade`、`category`、`knowledge_point`、`tag`、`question_id`、`count`、`model` | `agent_tool_organize_drafts` | 批量整理草稿元数据，单次 ≤10 道 |
| `fill_explanations` | 是 | 同上 | `agent_tool_fill_explanations` | 补答案与解析，单次 ≤10 道 |
| `generate_questions` | 是 | `subject`、`grade`、`category`、`knowledge_point`、`tag`、`count`、`model` | `agent_tool_generate_questions` | 知识点出题，保存为草稿，单次 ≤10 道 |
| `generate_variants` | 是 | `question_id`、`count`、`model` | `agent_tool_generate_variants` | 原题变式，保存为草稿 |
| `compose_paper` | 是 | `subject`、`grade`、`category`、`knowledge_point`、`tag`、`count`、`model` | `agent_tool_compose_paper` | 从已发布题目组 100 分试卷草稿 |
| `list_attachments` | 否 | — | — | 列出本轮附件的类型、状态与文本预览 |
| `extract_questions` | 否 | `attachment_ids`、`subject`、`grade` | — | PDF/Word/文本切题，仅预览，最多 20 道 |
| `import_document_questions` | 是 | `attachment_ids`、`subject`、`grade`、`category`、`tags` | `agent_tool_import_document_questions`、每题 `agent_import_question` | 文档切题并写入草稿，缺少答案的题跳过 |
| `create_questions` | 是 | `subject`、`grade`、`items` | `agent_tool_create_questions` | 结构化题目批量创建草稿，单次 ≤10 道 |
| `update_draft` | 是 | `question_id`、`category`、`knowledge_points`、`tags`、`difficulty`、`answer`、`explanation` | `agent_tool_update_draft` | 修改草稿；已发布题目只读 |

### 1.1 参数与返回

- 参数严格校验：未知字段、错误类型、越界数值直接拒绝，不执行任何写入。
- 写工具参数中的 `subject` 必须与教师学科一致；管理员必须显式提供学科。
- 工具返回结构：

```json
{
  "summary": "已从文档导入 2 道题目到草稿题库",
  "data": {"question_ids": ["..."], "total_detected": 2, "skipped": 0},
  "question_ids": ["..."],
  "warnings": []
}
```

消息中的工具记录额外包含 `name`、`label`、`status`、`arguments`、`started_at`、`finished_at`。前端据此渲染可展开的时间线。

### 1.2 执行策略

- 模型可用：OpenAI 兼容 function calling，最多 4 轮、8 次工具调用；工具结果回填模型后生成最终回复。
- 模型不可用：确定性本地规划器按规则选择工具，同一次请求最多 3 个步骤。
- 模型在工具执行后中断：保留已执行工具结果并停止，不回退重跑，避免重复写入。
- 所有写操作只作用于草稿；每次写入写 `ai_audit_logs`，并标记待人工复核。

## 2. 技能

技能是前端的一键提示词与工具组合，不绕过上面的参数校验、权限与审计。

| 技能 ID | 名称 | 接受附件 | 工具组合 |
|---|---|---|---|
| `document_import` | 文档智能录题 | 是 | `extract_questions` → `import_document_questions` |
| `paper_split` | 整卷拆题 | 是 | `extract_questions` |
| `bank_cleanup` | 题库体检与整理 | 否 | `bank_audit` → `organize_drafts` |
| `lesson_practice` | 课时练习生成 | 否 | `generate_questions` |
| `final_review` | 期末复习卷 | 否 | `compose_paper` |
| `variant_pack` | 错题变式包 | 否 | `generate_variants` |

## 3. 外部 Agent 接入

外部 Agent 可以按以下顺序复用同一套能力：

1. `GET /api/v1/ai/tools` 获取工具 Schema。
2. `POST /api/v1/ai/attachments` 上传文件，得到附件 ID 与解析状态。
3. `POST /api/v1/ai/conversations` 创建会话。
4. `POST /api/v1/ai/conversations/:id/messages` 发送目标与附件 ID，读取返回的 `assistant_message.tool_calls`。
5. 如需自行编排，可按 Schema 直接构造参数；服务端的权限、草稿限制与审计仍然生效。

## 4. 附件约束

- 每轮最多 5 个附件；图片 ≤10 MiB，文档 ≤20 MiB。
- 支持 PNG/JPEG/WebP/GIF、PDF、DOCX、TXT、Markdown、CSV。
- 图片走 OCR sidecar；PDF 内置文本流与 ToUnicode 映射抽取；DOCX 解压读取 `word/document.xml`。
- 扫描版 PDF 无法抽取足够文本时返回 `failed`，提示改用图片 OCR。
- 抽取文本上限 200,000 字符；单题题干 6,000 字符、解析 4,000 字符、选项 1,000 字符。
- 文件落在 `AI_UPLOAD_DIR`，数据库只存元数据与抽取文本；权限目录 `0750`、文件 `0640`。

## 5. 安全边界

- 教师只能读写自己的会话与草稿；管理员可读本校会话，但只能写入自己的会话。
- 已发布题目永不自动修改；发布、归档、删除、改密码等操作不暴露给 Agent。
- 工具执行不读取环境密钥，不访问网络，不执行任意命令。
- 上传以 magic bytes 判定类型，不信任客户端 MIME；路径经过清洗，拒绝目录穿越。
- 最大 20 MiB 上传与 200,000 字符抽取文本限制防止资源滥用。