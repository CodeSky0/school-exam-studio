# AI Agent 工具与技能契约

本文档描述对话工作台暴露给内置 Agent 与外部调用方的工具、技能、权限边界与返回结构。运行中的服务通过以下接口提供同一份机器可读契约：

- `GET /api/v1/ai/tools`：工具名称、标签、说明、JSON Schema、分类与是否为写操作。
- `POST /api/v1/ai/tools/:name`：按 Schema 直接执行工具，供外部 Agent 与业务页面调用。
- `GET /api/v1/ai/skills`：内置技能、提示词、所需工具、是否接受附件。
- `POST /api/v1/ai/skills/:id/run`：以技能提示词发起一次会话消息。
- `GET /api/v1/ai/status`：模型来源、降级策略、各模型熔断状态与能力清单。

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
| `read_attachment` | 否 | `attachment_ids`、`offset`、`limit` | — | 读取 PDF/Word/文本正文并分段；图片标记为视觉输入，不返回图片文本 |
| `extract_questions` | 否 | `attachment_ids`、`subject`、`grade` | — | PDF/Word/文本切题，仅预览，最多 20 道 |
| `import_document_questions` | 是 | `attachment_ids`、`subject`、`grade`、`category`、`tags` | `agent_tool_import_document_questions`、每题 `agent_import_question` | 文档切题并写入草稿，缺少答案的题跳过 |
| `create_questions` | 是 | `subject`、`grade`、`items` | `agent_tool_create_questions` | 结构化题目批量创建草稿，单次 ≤10 道 |
| `update_draft` | 是 | `question_id`、`category`、`knowledge_points`、`tags`、`difficulty`、`answer`、`explanation` | `agent_tool_update_draft` | 修改草稿；已发布题目只读 |
| `list_papers` | 否 | `subject`、`grade`、`keyword`、`limit` | — | 列出试卷，返回 ID、题量、总分与时长 |
| `get_paper` | 否 | `paper_id` | — | 读取试卷详情、题目顺序、分值与题目快照 |
| `paper_audit` | 否 | `paper_id` | — | 试卷质检：总分、难度分布、知识点覆盖、缺答案/解析题 |
| `update_paper_draft` | 是 | `paper_id`、`title`、`duration_minutes`、`questions[]` | `agent_tool_update_paper_draft` | 调整本人草稿试卷的题目与分值 |
| `bank_stats` | 否 | `subject`、`grade`、`limit` | — | 题库分布统计与知识点覆盖缺口 |
| `find_duplicates` | 否 | `limit` | — | 重复题分组，返回题目 ID 与摘要 |
| `review_queue` | 否 | `subject`、`grade`、`limit` | — | AI 生成且待人工复核的草稿列表 |
| `search_similar_questions` | 否 | `question_id` 或 `keyword`、`subject`、`grade`、`limit` | — | 基于文本重合度检索相似题 |

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

消息中的工具记录额外包含 `name`、`label`、`status`、`arguments`、`started_at`、`finished_at`。

助手消息还包含 `timeline` 数组，按时间顺序记录 `thought`（思考或中间说明）与 `tool`（工具调用，`tool_call_id` 指向 `tool_calls` 中的记录）两类条目。`content` 只保存最终答复；前端将 `timeline` 折叠为一行摘要，展开后按时间顺序展示思考与工具调用，最终答复显示在折叠区之后。

模型请求不发送 `max_tokens`，输出长度不设业务上限；仅保留防止异常响应耗尽内存的读取保护。

### 1.2 执行策略

- 模型可用：OpenAI 兼容 function calling，最多 4 轮、8 次工具调用；工具结果回填模型后生成最终回复。
- 弹性调用：每个模型最多重试 3 次（300ms/900ms 指数退避，尊重 `Retry-After`）；仍失败时按学校启用模型顺序切换，最多 3 个模型。
- 熔断：同一「学校 + 模型」连续 3 次可重试失败后熔断 60 秒，期间直接跳过；状态见 `GET /ai/status` 的 `models_health`。
- JSON 修复：模型返回非法 JSON 时追加一次“仅输出合法 JSON”的修复调用。
- 未配置模型：使用确定性本地规划器；已配置模型失败时**不再静默降级本地**，而是返回 `model_unavailable` 与尝试记录。
- 显式兜底：设置 `AI_ALLOW_LOCAL_FALLBACK=true` 后，模型失败可回退本地，并在 `warnings` 与 `fallback_policy` 中标注。
- 模型在工具执行后中断：保留已执行工具结果并停止，不回退重跑，避免重复写入。
- 重试与故障转移只作用于模型 HTTP 调用，绝不重放已执行的工具写操作。
- 所有写操作只作用于草稿；每次写入写 `ai_audit_logs`，并标记待人工复核。

## 2. 技能

技能是前端的一键提示词与工具组合，不绕过上面的参数校验、权限与审计。

| 技能 ID | 名称 | 接受附件 | 工具组合 |
|---|---|---|---|
| `document_reader` | 文档阅读 | 是 | `list_attachments` → `read_attachment` |
| `document_import` | 文档智能录题 | 是 | `extract_questions` → `import_document_questions` |
| `paper_split` | 整卷拆题 | 是 | `extract_questions` |
| `bank_cleanup` | 题库体检与整理 | 否 | `bank_audit` → `organize_drafts` |
| `lesson_practice` | 课时练习生成 | 否 | `generate_questions` |
| `final_review` | 期末复习卷 | 否 | `compose_paper` |
| `variant_pack` | 错题变式包 | 否 | `generate_variants` |
| `bank_gap_analysis` | 题库覆盖度分析 | 否 | `bank_stats` → `bank_audit` |
| `duplicate_cleanup` | 重复题排查 | 否 | `find_duplicates` → `bank_audit` |
| `paper_quality_check` | 试卷质检 | 否 | `list_papers` → `paper_audit` |
| `review_assistant` | AI 产出复核 | 否 | `review_queue` → `get_question` |
| `exam_blueprint` | 双向细目表组卷 | 否 | `bank_stats` → `compose_paper` |
| `similar_question_pack` | 相似题练习包 | 否 | `search_similar_questions` → `create_questions` |

技能也可通过 `POST /api/v1/ai/skills/:id/run` 直接执行，请求体为 `{"content": "可选的补充要求", "model": "可选的模型"}`，返回新建会话与助手消息。

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
- 图片不走 OCR：上传后保留原始文件，状态直接为 `parsed`，发送消息时以 base64 data URL 作为多模态输入交给模型直接阅读。
- PDF 内置文本流与 ToUnicode 映射抽取；DOCX 解压读取 `word/document.xml`；纯文本直接读取。
- 扫描版 PDF 无文本层时状态仍为 `parsed`，`text_chars=0`；Agent 会提示改为上传页面图片，由视觉模型直接识别。
- 视觉输入上限：单张图片 10 MiB，整轮原图合计 16 MiB；超限图片会跳过并在回复中提示。
- 抽取文本上限 200,000 字符；`read_attachment` 默认每次返回 4,000 字符，可按 `offset` 继续读取。
- 单题题干 6,000 字符、解析 4,000 字符、选项 1,000 字符。
- 文件落在 `AI_UPLOAD_DIR`，数据库只存元数据与抽取文本；权限目录 `0750`、文件 `0640`。

## 5. 安全边界

- 教师只能读写自己的会话与草稿；管理员可读本校会话，但只能写入自己的会话。
- 已发布题目永不自动修改；发布、归档、删除、改密码等操作不暴露给 Agent。
- 工具执行不读取环境密钥，不访问网络，不执行任意命令；图片仅在本轮消息中作为视觉输入发送，不写入题库。
- 上传以 magic bytes 判定类型，不信任客户端 MIME；路径经过清洗，拒绝目录穿越。
- 最大 20 MiB 上传与 200,000 字符抽取文本限制防止资源滥用。