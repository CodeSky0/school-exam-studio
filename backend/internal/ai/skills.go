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
			Description: "上传 PDF、Word、图片或文本，自动切题、识别答案与选项，先预览再导入草稿题库。",
			Icon:        "file", AcceptsAttachments: true,
			Prompt:        "请解析我上传的文件，先预览识别到的题目，然后帮我把完整的题目导入题库。",
			Steps:         []string{"提取附件文本", "切分题目", "预览确认", "写入草稿"},
			RequiredTools: []string{"extract_questions", "import_document_questions"},
		},
		{
			ID: "paper_split", Title: "整卷拆题", Category: "document",
			Description: "把整份试卷拆成独立题目，保留题号、选项、答案与解析，便于逐题校对。",
			Icon:        "split", AcceptsAttachments: true,
			Prompt:        "这是一份完整试卷，请按题号拆成独立题目并列出预览，暂时不要入库。",
			Steps:         []string{"提取附件文本", "按题号切分", "列出预览"},
			RequiredTools: []string{"extract_questions"},
		},
		{
			ID: "bank_cleanup", Title: "题库体检与整理", Category: "question",
			Description: "检查重复题、缺失答案/解析/知识点/分类，并批量整理草稿元数据。",
			Icon:        "shield", AcceptsAttachments: false,
			Prompt:        "请对题库做一次质量体检，然后整理缺少分类、知识点和标签的草稿。",
			Steps:         []string{"题库体检", "汇总问题", "整理草稿元数据"},
			RequiredTools: []string{"bank_audit", "organize_drafts"},
		},
		{
			ID: "lesson_practice", Title: "课时练习生成", Category: "question",
			Description: "按年级、知识点和题量生成课时练习草稿，可直接进入组卷流程。",
			Icon:        "spark", AcceptsAttachments: false,
			Prompt:        "请为八年级数学「一元一次方程」生成 5 道由易到难的课时练习草稿。",
			Steps:         []string{"确认知识点", "生成题目", "标记待复核"},
			RequiredTools: []string{"generate_questions"},
		},
		{
			ID: "final_review", Title: "期末复习卷", Category: "paper",
			Description: "从已发布题目中按知识点与难度分布组一份 100 分复习卷。",
			Icon:        "paper", AcceptsAttachments: false,
			Prompt:        "请用已发布题目为八年级数学组一份 100 分的期末复习卷，覆盖主要知识点。",
			Steps:         []string{"统计覆盖度", "智能组卷", "输出试卷草稿"},
			RequiredTools: []string{"compose_paper"},
		},
		{
			ID: "variant_pack", Title: "错题变式包", Category: "question",
			Description: "针对一道题生成换数、换情境、逆向提问的变式题，用于订正与分层练习。",
			Icon:        "repeat", AcceptsAttachments: false,
			Prompt:        "请帮我选中的题目生成 3 道变式题，保留知识点并适当调整难度。",
			Steps:         []string{"读取原题", "生成变式", "标记待复核"},
			RequiredTools: []string{"generate_variants"},
		},
		{
			ID: "bank_gap_analysis", Title: "题库覆盖度分析", Category: "analytics",
			Description: "统计题库在题型、难度与知识点上的分布，找出覆盖不足的知识点并给出补题建议。",
			Icon:        "chart", AcceptsAttachments: false,
			Prompt:        "请分析当前题库的覆盖度：先统计学科、年级、题型、难度和知识点分布，再指出覆盖不足的知识点，并给出补题建议。",
			Steps:         []string{"统计分布", "识别缺口", "给出补题建议"},
			RequiredTools: []string{"bank_stats", "bank_audit"},
		},
		{
			ID: "duplicate_cleanup", Title: "重复题排查", Category: "analytics",
			Description: "按内容指纹找出重复题分组，给出保留建议，避免同题重复入库。",
			Icon:        "copy", AcceptsAttachments: false,
			Prompt:        "请找出题库中的重复题，按组列出题目 ID 与题干摘要，并说明建议保留哪一道、其余如何处理。",
			Steps:         []string{"内容指纹分组", "列出重复组", "给出处理建议"},
			RequiredTools: []string{"find_duplicates", "bank_audit"},
		},
		{
			ID: "paper_quality_check", Title: "试卷质检", Category: "paper",
			Description: "检查试卷总分、题量、难度分布、知识点覆盖以及缺少答案或解析的题目。",
			Icon:        "check", AcceptsAttachments: false,
			Prompt:        "请对我指定的试卷做质检：核对总分与题量，检查难度分布、知识点覆盖，并列出缺少答案或解析的题目。",
			Steps:         []string{"读取试卷", "核对总分与题量", "输出质检报告"},
			RequiredTools: []string{"list_papers", "paper_audit"},
		},
		{
			ID: "review_assistant", Title: "AI 产出复核", Category: "review",
			Description: "列出 AI 生成的待复核草稿，逐条检查答案与解析后给出通过或修改建议。",
			Icon:        "eye", AcceptsAttachments: false,
			Prompt:        "请列出所有 AI 生成且待人工复核的草稿题目，逐条检查答案和解析，并给出通过或修改建议。",
			Steps:         []string{"读取待复核队列", "逐题检查", "给出复核结论"},
			RequiredTools: []string{"review_queue", "get_question"},
		},
		{
			ID: "exam_blueprint", Title: "双向细目表组卷", Category: "paper",
			Description: "先统计知识点与难度分布，再按双向细目表组卷，保证覆盖均衡。",
			Icon:        "table", AcceptsAttachments: false,
			Prompt:        "请先统计题库的知识点与难度分布，然后按双向细目表为指定年级和学科组一份覆盖均衡的试卷草稿。",
			Steps:         []string{"统计分布", "设计细目表", "智能组卷"},
			RequiredTools: []string{"bank_stats", "compose_paper"},
		},
		{
			ID: "similar_question_pack", Title: "相似题练习包", Category: "question",
			Description: "围绕一道题检索相似题，组成分层练习草稿，用于巩固同一知识点。",
			Icon:        "layers", AcceptsAttachments: false,
			Prompt:        "请以我指定的题目为中心，检索相似题并按难度排序，组成一份巩固练习草稿。",
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
