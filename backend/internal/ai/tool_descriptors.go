package ai

import "encoding/json"

var (
	toolSearchQuestions = ToolDescriptor{
		Name: "search_questions", Label: "检索题库", Writes: false,
		Description: "按关键词、学科、年级、状态、分类或知识点检索题目，返回题目 ID 与摘要。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"keyword":{"type":"string"},"subject":{"type":"string"},"grade":{"type":"string"},"status":{"type":"string","enum":["draft","published","archived"]},"category":{"type":"string"},"knowledge_point":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50}},"additionalProperties":false}`),
	}
	toolGetQuestion = ToolDescriptor{
		Name: "get_question", Label: "读取题目", Writes: false,
		Description: "读取一道题目的完整内容、选项、答案、解析与元数据。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"question_id":{"type":"string","format":"uuid"}},"required":["question_id"],"additionalProperties":false}`),
	}
	toolBankAudit = ToolDescriptor{
		Name: "bank_audit", Label: "题库体检", Writes: false,
		Description: "检查题库缺失答案、解析、知识点、分类、标签与重复题，输出质量报告。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":1000}},"additionalProperties":false}`),
	}
	toolAnalyzeQuestion = ToolDescriptor{
		Name: "analyze_question", Label: "分析题目", Writes: true,
		Description: "分析题目并生成分类、知识点、标签、答案与解析；草稿可直接写回，已发布题目只读。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"question_id":{"type":"string","format":"uuid"},"apply":{"type":"boolean"},"model":{"type":"string"}},"required":["question_id"],"additionalProperties":false}`),
	}
	toolOrganizeDrafts = ToolDescriptor{
		Name: "organize_drafts", Label: "整理草稿", Writes: true,
		Description: "为缺少分类、知识点或标签的草稿批量补全元数据。只处理本人（管理员为本校）草稿。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"category":{"type":"string"},"knowledge_point":{"type":"string"},"tag":{"type":"string"},"question_id":{"type":"string","format":"uuid"},"count":{"type":"integer","minimum":1,"maximum":10},"model":{"type":"string"}},"additionalProperties":false}`),
	}
	toolFillExplanations = ToolDescriptor{
		Name: "fill_explanations", Label: "补答案解析", Writes: true,
		Description: "为缺少答案或解析的草稿批量补全内容，写入后标记为待人工复核。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"category":{"type":"string"},"knowledge_point":{"type":"string"},"tag":{"type":"string"},"question_id":{"type":"string","format":"uuid"},"count":{"type":"integer","minimum":1,"maximum":10},"model":{"type":"string"}},"additionalProperties":false}`),
	}
	toolGenerateQuestions = ToolDescriptor{
		Name: "generate_questions", Label: "知识点出题", Writes: true,
		Description: "按学科、年级、知识点、题型与难度生成新题，默认保存为待复核草稿。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"category":{"type":"string"},"knowledge_point":{"type":"string"},"tag":{"type":"string"},"count":{"type":"integer","minimum":1,"maximum":10},"model":{"type":"string"}},"required":["grade"],"additionalProperties":false}`),
	}
	toolGenerateVariants = ToolDescriptor{
		Name: "generate_variants", Label: "生成变式", Writes: true,
		Description: "基于指定题目生成换数、换情境或难度调整的变式题草稿。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"question_id":{"type":"string","format":"uuid"},"count":{"type":"integer","minimum":1,"maximum":10},"model":{"type":"string"}},"required":["question_id"],"additionalProperties":false}`),
	}
	toolComposePaper = ToolDescriptor{
		Name: "compose_paper", Label: "智能组卷", Writes: true,
		Description: "从已发布题目中按学科、年级、知识点与难度分布组一份满分 100 的试卷草稿。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"category":{"type":"string"},"knowledge_point":{"type":"string"},"tag":{"type":"string"},"count":{"type":"integer","minimum":1,"maximum":10},"model":{"type":"string"}},"required":["grade"],"additionalProperties":false}`),
	}
	toolListAttachments = ToolDescriptor{
		Name: "list_attachments", Label: "查看附件", Writes: false,
		Description: "列出本轮附件的类型、解析状态与文本预览。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
	}
	toolReadAttachment = ToolDescriptor{
		Name: "read_attachment", Label: "阅读附件", Writes: false,
		Description: "读取本轮附件正文：PDF/Word/文本返回抽取文本并支持分段，图片已作为视觉输入直接提供给模型。不会写入题库。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"attachment_ids":{"type":"array","items":{"type":"string"}},"offset":{"type":"integer","minimum":0,"maximum":200000},"limit":{"type":"integer","minimum":500,"maximum":6000}},"additionalProperties":false}`),
	}
	toolSearchCloud = ToolDescriptor{
		Name: "search_cloud", Label: "检索云盘资料", Writes: false,
		Description: "按关键词或问题语义检索学校云盘中已自动向量化的资料，返回最相关的文件名称、原文片段与相似度。适合引用教案、试卷、教材章节或核对校内资料，不写入任何数据。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":20}},"required":["query"],"additionalProperties":false}`),
	}
	toolExtractQuestions = ToolDescriptor{
		Name: "extract_questions", Label: "文档切题", Writes: false,
		Description: "从本轮 PDF、Word、文本附件中抽取文本并切分为题目草稿，只预览不入库。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"attachment_ids":{"type":"array","items":{"type":"string"}}},"additionalProperties":false}`),
	}
	toolImportDocument = ToolDescriptor{
		Name: "import_document_questions", Label: "文档入库", Writes: true,
		Description: "从本轮 PDF、Word、文本附件中切题并批量写入草稿题库；缺少答案的题目会被跳过。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"category":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}},"attachment_ids":{"type":"array","items":{"type":"string"}}},"required":["subject","grade"],"additionalProperties":false}`),
	}
	toolCreateQuestions = ToolDescriptor{
		Name: "create_questions", Label: "创建题目", Writes: true,
		Description: "把结构化题目批量创建为草稿，用于模型直接生成的题目。question_type 必须使用英文枚举值：single_choice（单选题）、multiple_choice（多选题）、true_false（判断题）、fill_blank（填空题）、essay（解答题/主观题），不要填写中文题型名或其他别名。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"items":{"type":"array","items":{"type":"object","properties":{"content":{"type":"string"},"answer":{"type":"string"},"explanation":{"type":"string"},"question_type":{"type":"string","enum":["single_choice","multiple_choice","true_false","fill_blank","essay"],"description":"single_choice=单选题，multiple_choice=多选题，true_false=判断题，fill_blank=填空题，essay=解答题"},"difficulty":{"type":"integer","minimum":1,"maximum":5},"category":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}},"knowledge_points":{"type":"array","items":{"type":"string"}},"options":{"type":"array","items":{"type":"object","properties":{"key":{"type":"string"},"content":{"type":"string"}},"required":["key","content"]}}},"required":["content","answer","question_type"]}}},"required":["subject","grade","items"],"additionalProperties":false}`),
	}
	toolUpdateDraft = ToolDescriptor{
		Name: "update_draft", Label: "修改草稿", Writes: true,
		Description: "修改一道草稿题目的分类、知识点、标签、难度、答案或解析；已发布题目只读。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"question_id":{"type":"string","format":"uuid"},"category":{"type":"string"},"knowledge_points":{"type":"array","items":{"type":"string"}},"tags":{"type":"array","items":{"type":"string"}},"difficulty":{"type":"integer","minimum":1,"maximum":5},"answer":{"type":"string"},"explanation":{"type":"string"},"model":{"type":"string"}},"required":["question_id"],"additionalProperties":false}`),
	}
	toolListPapers = ToolDescriptor{
		Name: "list_papers", Label: "列出试卷", Writes: false,
		Description: "按学科、年级与标题关键词列出试卷，返回试卷 ID、题量、总分与时长。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"keyword":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50}},"additionalProperties":false}`),
	}
	toolGetPaper = ToolDescriptor{
		Name: "get_paper", Label: "读取试卷", Writes: false,
		Description: "读取试卷详情、题目顺序、分值以及每道题的完整快照。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"paper_id":{"type":"string","format":"uuid"}},"required":["paper_id"],"additionalProperties":false}`),
	}
	toolPaperAudit = ToolDescriptor{
		Name: "paper_audit", Label: "试卷质检", Writes: false,
		Description: "检查试卷题量、总分、难度分布、知识点覆盖以及缺少答案或解析的题目。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"paper_id":{"type":"string","format":"uuid"}},"required":["paper_id"],"additionalProperties":false}`),
	}
	toolUpdatePaperDraft = ToolDescriptor{
		Name: "update_paper_draft", Label: "调整试卷", Writes: true,
		Description: "替换本人草稿试卷的题目顺序与分值，可同时修改标题与时长；只能修改本人创建的试卷。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"paper_id":{"type":"string","format":"uuid"},"title":{"type":"string"},"duration_minutes":{"type":"integer","minimum":1,"maximum":600},"questions":{"type":"array","items":{"type":"object","properties":{"question_id":{"type":"string","format":"uuid"},"assigned_score":{"type":"number"}},"required":["question_id","assigned_score"]}}},"required":["paper_id","questions"],"additionalProperties":false}`),
	}
	toolBankStats = ToolDescriptor{
		Name: "bank_stats", Label: "题库统计", Writes: false,
		Description: "统计题库在学科、年级、题型、难度与知识点上的分布，并列出覆盖不足的知识点。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":1000}},"additionalProperties":false}`),
	}
	toolFindDuplicates = ToolDescriptor{
		Name: "find_duplicates", Label: "重复题排查", Writes: false,
		Description: "按内容指纹找出重复题分组，返回题目 ID、重复数量与题干摘要。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":1000}},"additionalProperties":false}`),
	}
	toolReviewQueue = ToolDescriptor{
		Name: "review_queue", Label: "AI 待复核", Writes: false,
		Description: "列出 AI 生成且标记为待人工复核的草稿题目，供教师逐条审核。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`),
	}
	toolSearchSimilar = ToolDescriptor{
		Name: "search_similar_questions", Label: "相似题检索", Writes: false,
		Description: "根据题目 ID 或题干关键词，按文本重合度检索相似题目，用于组卷补题与去重参考。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"question_id":{"type":"string","format":"uuid"},"keyword":{"type":"string"},"subject":{"type":"string"},"grade":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":30}},"additionalProperties":false}`),
	}
)
