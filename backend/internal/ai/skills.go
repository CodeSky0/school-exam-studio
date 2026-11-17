package ai

// Skill 是可被前端一键调用的内置工作流模板。
type Skill struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Icon               string   `json:"icon"`
	Prompt             string   `json:"prompt"`
	RequiredTools      []string `json:"required_tools"`
	AcceptsAttachments bool     `json:"accepts_attachments"`
}

// ListSkills 返回内置技能目录，Agent 与外部调用方通过同一份契约发现能力。
func (s *Service) ListSkills() []Skill {
	return []Skill{
		{
			ID: "document_import", Title: "文档智能录题",
			Description: "上传 PDF、Word、图片或文本，自动切题、识别答案与选项，先预览再导入草稿题库。",
			Icon:        "file", AcceptsAttachments: true,
			Prompt:        "请解析我上传的文件，先预览识别到的题目，然后帮我把完整的题目导入题库。",
			RequiredTools: []string{"extract_questions", "import_document_questions"},
		},
		{
			ID: "paper_split", Title: "整卷拆题",
			Description: "把整份试卷拆成独立题目，保留题号、选项、答案与解析，便于逐题校对。",
			Icon:        "split", AcceptsAttachments: true,
			Prompt:        "这是一份完整试卷，请按题号拆成独立题目并列出预览，暂时不要入库。",
			RequiredTools: []string{"extract_questions"},
		},
		{
			ID: "bank_cleanup", Title: "题库体检与整理",
			Description: "检查重复题、缺失答案/解析/知识点/分类，并批量整理草稿元数据。",
			Icon:        "shield", AcceptsAttachments: false,
			Prompt:        "请对题库做一次质量体检，然后整理缺少分类、知识点和标签的草稿。",
			RequiredTools: []string{"bank_audit", "organize_drafts"},
		},
		{
			ID: "lesson_practice", Title: "课时练习生成",
			Description: "按年级、知识点和题量生成课时练习草稿，可直接进入组卷流程。",
			Icon:        "spark", AcceptsAttachments: false,
			Prompt:        "请为八年级数学「一元一次方程」生成 5 道由易到难的课时练习草稿。",
			RequiredTools: []string{"generate_questions"},
		},
		{
			ID: "final_review", Title: "期末复习卷",
			Description: "从已发布题目中按知识点与难度分布组一份 100 分复习卷。",
			Icon:        "paper", AcceptsAttachments: false,
			Prompt:        "请用已发布题目为八年级数学组一份 100 分的期末复习卷，覆盖主要知识点。",
			RequiredTools: []string{"compose_paper"},
		},
		{
			ID: "variant_pack", Title: "错题变式包",
			Description: "针对一道题生成换数、换情境、逆向提问的变式题，用于订正与分层练习。",
			Icon:        "repeat", AcceptsAttachments: false,
			Prompt:        "请帮我选中的题目生成 3 道变式题，保留知识点并适当调整难度。",
			RequiredTools: []string{"generate_variants"},
		},
	}
}
