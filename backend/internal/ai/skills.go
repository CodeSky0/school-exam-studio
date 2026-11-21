package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"school-exam-studio/backend/internal/auth"
)

// Skill 是可被前端一键调用的内置工作流模板。
type Skill struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Icon               string   `json:"icon"`
	Category           string   `json:"category"`
	Prompt             string   `json:"prompt"`
	Steps              []string `json:"steps"`
	RequiredTools      []string `json:"required_tools"`
	AcceptsAttachments bool     `json:"accepts_attachments"`
}

// SkillRunInput 是一次技能执行的参数。
type SkillRunInput struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

// SkillRunResult 返回技能执行产生的会话与助手消息。
type SkillRunResult struct {
	SkillID          string      `json:"skill_id"`
	ConversationID   string      `json:"conversation_id"`
	AssistantMessage ChatMessage `json:"assistant_message"`
	Provider         string      `json:"provider"`
	Model            string      `json:"model"`
}

// ListSkills 返回内置技能目录，Agent 与外部调用方通过同一份契约发现能力。
func (s *Service) ListSkills() []Skill {
	return []Skill{
		{
			ID: "document_import", Title: "文档智能录题", Category: "document",
			Description: "上传 PDF、Word 或图片，由模型直接阅读并识别题目，确认后写入草稿题库。",
			Icon:        "file", AcceptsAttachments: true,
			Prompt: "请直接阅读我上传的文件：PDF/Word 已作为文件输入、图片已作为视觉输入随消息发送，不需要我提供文本或做 OCR。" +
				"先说明文档类型与结构，按题号列出题型、题干摘要与答案预览供我确认；确认无误后调用 create_questions 把题目写入草稿题库（单次最多 10 道），缺少答案的题目请单独列出并跳过。",
			Steps:         []string{"直接阅读文件", "识别题目", "预览确认", "写入草稿"},
			RequiredTools: []string{"create_questions"},
		},
		{
			ID: "document_reader", Title: "文档阅读", Category: "document",
			Description: "直接阅读 PDF、Word 或图片，提取正文、公式与表格，输出结构化摘要或按需回答。",
			Icon:        "book", AcceptsAttachments: true,
			Prompt: "请直接阅读我上传的资料：PDF/Word 已作为文件输入、图片已作为视觉输入随消息发送，不需要我提供文本或做 OCR。" +
				"先说明文档类型与结构，然后给出结构化摘要，包括标题、章节要点、关键公式与表格、结论。" +
				"内容较长时按章节逐段总结；不要臆造原文中不存在的内容。",
			Steps:         []string{"直接阅读文件与图片", "提取正文与表格", "输出结构化摘要"},
			RequiredTools: []string{},
		},
		{
			ID: "cloud_research", Title: "云盘资料问答", Category: "document",
			Description: "在学校云盘已向量化的资料中做语义检索，基于原文片段回答或整理要点，并标注引用的文件名。",
			Icon:        "book", AcceptsAttachments: false,
			Prompt: "请根据我提出的问题使用 search_cloud 在学校云盘中检索相关资料。" +
				"基于检索到的原文片段回答，列出来源文件名与相关片段；若首次检索结果不理想，可换关键词再试一次；资料不足时明确说明，不要编造云盘中没有的内容。",
			Steps:         []string{"语义检索云盘资料", "核对原文片段", "输出引用与结论"},
			RequiredTools: []string{"search_cloud"},
		},
		{
			ID: "bank_cleanup", Title: "题库体检与整理", Category: "question",
			Description: "检查重复题、缺失答案/解析/知识点/分类，并批量整理草稿元数据。",
			Icon:        "shield", AcceptsAttachments: false,
			Prompt:        "请先调用 bank_audit 对题库做一次质量体检，汇总重复题与缺失项；然后用 organize_drafts 为缺少分类、知识点或标签的草稿批量补全元数据，最后报告整理了几道题并列出题目 ID，提醒我复核。",
			Steps:         []string{"题库体检", "汇总问题", "整理草稿元数据"},
			RequiredTools: []string{"bank_audit", "organize_drafts"},
		},
		{
			ID: "lesson_practice", Title: "课时练习生成", Category: "question",
			Description: "按年级、知识点和题量生成课时练习草稿，可直接进入组卷流程。",
			Icon:        "spark", AcceptsAttachments: false,
			Prompt:        "请用 generate_questions 为我指定的年级、学科和知识点生成课时练习草稿（默认 5 道，由易到难覆盖该知识点），生成后列出题目 ID、题型与难度分布，并提醒我逐题复核。",
			Steps:         []string{"确认知识点", "生成题目", "标记待复核"},
			RequiredTools: []string{"generate_questions"},
		},
		{
			ID: "final_review", Title: "期末复习卷", Category: "paper",
			Description: "从已发布题目中按知识点与难度分布组一份 100 分复习卷。",
			Icon:        "paper", AcceptsAttachments: false,
			Prompt:        "请先调用 bank_stats 了解目标年级学科的已发布题目覆盖情况，再用 compose_paper 组一份 100 分的期末复习卷，覆盖主要知识点；完成后报告试卷 ID、题量与知识点覆盖情况。",
			Steps:         []string{"统计覆盖度", "智能组卷", "输出试卷草稿"},
			RequiredTools: []string{"compose_paper"},
		},
		{
			ID: "variant_pack", Title: "错题变式包", Category: "question",
			Description: "针对一道题生成换数、换情境、逆向提问的变式题，用于订正与分层练习。",
			Icon:        "repeat", AcceptsAttachments: false,
			Prompt:        "请先用 get_question 读取我指定题目（或最近上下文中的题目）的完整内容，再用 generate_variants 生成 3 道变式题，保持知识点一致并适当调整难度；生成后列出各变式的题干摘要与题目 ID，提醒我复核。",
			Steps:         []string{"读取原题", "生成变式", "标记待复核"},
			RequiredTools: []string{"generate_variants"},
		},
		{
			ID: "bank_gap_analysis", Title: "题库覆盖度分析", Category: "analytics",
			Description: "统计题库在题型、难度与知识点上的分布，找出覆盖不足的知识点并给出补题建议。",
			Icon:        "chart", AcceptsAttachments: false,
			Prompt:        "请分析当前题库的覆盖度：先用 bank_stats 统计学科、年级、题型、难度和知识点分布，再用 bank_audit 检查缺失项；最后指出覆盖不足的知识点，按优先级给出补题数量与题型建议。",
			Steps:         []string{"统计分布", "识别缺口", "给出补题建议"},
			RequiredTools: []string{"bank_stats", "bank_audit"},
		},
		{
			ID: "duplicate_cleanup", Title: "重复题排查", Category: "analytics",
			Description: "按内容指纹找出重复题分组，给出保留建议，避免同题重复入库。",
			Icon:        "copy", AcceptsAttachments: false,
			Prompt:        "请用 find_duplicates 找出题库中的重复题，按组列出题目 ID 与题干摘要；对每组给出保留建议（保留信息更完整的一道），并用 bank_audit 交叉核对其他质量问题，说明建议处理方式，不要直接删除任何题目。",
			Steps:         []string{"内容指纹分组", "列出重复组", "给出处理建议"},
			RequiredTools: []string{"find_duplicates", "bank_audit"},
		},
		{
			ID: "paper_quality_check", Title: "试卷质检", Category: "paper",
			Description: "检查试卷总分、题量、难度分布、知识点覆盖以及缺少答案或解析的题目。",
			Icon:        "check", AcceptsAttachments: false,
			Prompt:        "请先调用 list_papers 确认我指定的试卷（或按我的描述检索），再用 paper_audit 做质检：核对总分与题量、检查难度分布与知识点覆盖、列出缺少答案或解析的题目，最后按严重程度输出质检报告。",
			Steps:         []string{"读取试卷", "核对总分与题量", "输出质检报告"},
			RequiredTools: []string{"list_papers", "paper_audit"},
		},
		{
			ID: "review_assistant", Title: "AI 产出复核", Category: "review",
			Description: "列出 AI 生成的待复核草稿，逐条检查答案与解析后给出通过或修改建议。",
			Icon:        "eye", AcceptsAttachments: false,
			Prompt:        "请用 review_queue 列出所有 AI 生成且待人工复核的草稿题目，逐条用 get_question 读取完整内容，检查答案正确性与解析合理性，对每道题给出「通过」「需修改（说明原因与建议）」的结论，并汇总统计。",
			Steps:         []string{"读取待复核队列", "逐题检查", "给出复核结论"},
			RequiredTools: []string{"review_queue", "get_question"},
		},
		{
			ID: "exam_blueprint", Title: "双向细目表组卷", Category: "paper",
			Description: "先统计知识点与难度分布，再按双向细目表组卷，保证覆盖均衡。",
			Icon:        "table", AcceptsAttachments: false,
			Prompt:        "请先用 bank_stats 统计目标年级学科的知识点与难度分布，据此设计一份双向细目表（知识点 × 题型 × 难度 × 分值）展示给我，然后按细目表用 compose_paper 组卷，完成后说明细目表与实际试卷的差异。",
			Steps:         []string{"统计分布", "设计细目表", "智能组卷"},
			RequiredTools: []string{"bank_stats", "compose_paper"},
		},
		{
			ID: "similar_question_pack", Title: "相似题练习包", Category: "question",
			Description: "围绕一道题检索相似题，组成分层练习草稿，用于巩固同一知识点。",
			Icon:        "layers", AcceptsAttachments: false,
			Prompt:        "请先用 get_question 读取我指定的题目，再用 search_similar_questions 按题干关键词检索相似题并按难度排序；若现有相似题不足，用 create_questions 补充 2-3 道同知识点题目，最后输出按难度分层的练习清单（含题目 ID）。",
			Steps:         []string{"读取原题", "检索相似题", "按难度排序"},
			RequiredTools: []string{"search_similar_questions", "create_questions"},
		},
	}
}

// RunSkill 以技能提示词发起一次会话消息，返回会话与助手消息。
func (s *Service) RunSkill(ctx context.Context, principal auth.Principal, skillID string, input SkillRunInput) (SkillRunResult, error) {
	if err := validatePrincipal(principal); err != nil {
		return SkillRunResult{}, err
	}
	if s.repo == nil {
		return SkillRunResult{}, fmt.Errorf("会话服务不可用")
	}
	var skill *Skill
	for _, item := range s.ListSkills() {
		if item.ID == skillID {
			copy := item
			skill = &copy
			break
		}
	}
	if skill == nil {
		return SkillRunResult{}, validationError("skill_id", "不支持的技能")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		content = skill.Prompt
	}
	conversation, err := s.CreateConversation(ctx, principal, skill.Title)
	if err != nil {
		return SkillRunResult{}, err
	}
	conversationID, err := uuid.Parse(conversation.ID)
	if err != nil {
		return SkillRunResult{}, fmt.Errorf("会话 ID 无效")
	}
	result, err := s.SendMessage(ctx, principal, conversationID, SendMessageInput{Content: content, Model: input.Model})
	if err != nil {
		return SkillRunResult{}, err
	}
	return SkillRunResult{
		SkillID:          skill.ID,
		ConversationID:   conversation.ID,
		AssistantMessage: result.AssistantMessage,
		Provider:         result.AssistantMessage.Provider,
		Model:            result.AssistantMessage.Model,
	}, nil
}
