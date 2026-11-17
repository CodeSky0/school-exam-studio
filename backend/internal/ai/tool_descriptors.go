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
		Description: "把结构化题目批量创建为草稿，用于模型直接生成的题目。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"subject":{"type":"string"},"grade":{"type":"string"},"items":{"type":"array","items":{"type":"object","properties":{"content":{"type":"string"},"answer":{"type":"string"},"explanation":{"type":"string"},"question_type":{"type":"string"},"difficulty":{"type":"integer","minimum":1,"maximum":5},"category":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}},"knowledge_points":{"type":"array","items":{"type":"string"}},"options":{"type":"array","items":{"type":"object","properties":{"key":{"type":"string"},"content":{"type":"string"}},"required":["key","content"]}}},"required":["content","answer","question_type"]}}},"required":["subject","grade","items"],"additionalProperties":false}`),
	}
	toolUpdateDraft = ToolDescriptor{
		Name: "update_draft", Label: "修改草稿", Writes: true,
		Description: "修改一道草稿题目的分类、知识点、标签、难度、答案或解析；已发布题目只读。",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"question_id":{"type":"string","format":"uuid"},"category":{"type":"string"},"knowledge_points":{"type":"array","items":{"type":"string"}},"tags":{"type":"array","items":{"type":"string"}},"difficulty":{"type":"integer","minimum":1,"maximum":5},"answer":{"type":"string"},"explanation":{"type":"string"},"model":{"type":"string"}},"required":["question_id"],"additionalProperties":false}`),
	}
)
