package ai

import (
	"fmt"
	"regexp"
	"strings"

	"school-exam-studio/backend/internal/auth"
)

var (
	chatUUIDPattern        = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	chatGenerateAskPattern = regexp.MustCompile(`出\s*(?:\d+|[一二三四五六七八九十两几])\s*道`)
)

type PlannedToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ChatTurnPlan struct {
	Calls   []PlannedToolCall `json:"calls"`
	Reply   string            `json:"reply,omitempty"`
	Clarify bool              `json:"clarify,omitempty"`
}

// planChatTurn 是确定性本地规划器：模型不可用时保证对话与工具调用仍可工作。
func planChatTurn(content string, attachments []AttachmentRef, prior []ChatMessage, principal auth.Principal) ChatTurnPlan {
	goal := strings.TrimSpace(content)
	lower := strings.ToLower(goal)
	attachmentIDs := make([]string, 0, len(attachments))
	hasDocument := false
	hasImage := false
	for _, item := range attachments {
		attachmentIDs = append(attachmentIDs, item.ID)
		if item.Kind == AttachmentKindImage {
			hasImage = true
		} else {
			hasDocument = true
		}
	}
	subject := detectSubject(goal)
	if principal.Role == auth.RoleTeacher {
		subject = principal.Subject
	}
	grade := detectGrade(goal)
	count := extractCount(goal)
	if count <= 0 {
		count = 3
	}
	if count > DefaultMaxGenerate {
		count = DefaultMaxGenerate
	}
	knowledgePoint := firstNonEmpty(extractQuoted(goal), matchKnowledgePoint(goal))

	if len(attachments) > 0 {
		if hasImage && !hasDocument {
			return ChatTurnPlan{Reply: "内置本地引擎无法直接识别图片。请在 /admin/settings 配置支持图片输入的多模态模型后重试，或上传含文本层的 PDF/Word 文档。"}
		}
		importIntent := containsAny(lower, "导入", "入库", "录题", "加入题库", "存到题库", "保存到题库", "录入题库")
		if importIntent {
			if subject == "" || grade == "" {
				return ChatTurnPlan{
					Clarify: true,
					Reply:   "我可以把附件中的题目导入草稿题库。请补充学科和年级，例如“把这份试卷按八年级数学导入题库”。",
				}
			}
			calls := make([]PlannedToolCall, 0, 2)
			if hasDocument {
				calls = append(calls, PlannedToolCall{Name: "extract_questions", Arguments: map[string]any{
					"subject": subject, "grade": grade, "attachment_ids": attachmentIDs,
				}})
			}
			calls = append(calls, PlannedToolCall{Name: "import_document_questions", Arguments: map[string]any{
				"subject": subject, "grade": grade, "attachment_ids": attachmentIDs,
			}})
			return ChatTurnPlan{Calls: calls}
		}
		if hasDocument {
			return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "extract_questions", Arguments: map[string]any{
				"subject": subject, "grade": grade, "attachment_ids": attachmentIDs,
			}}}}
		}
		if hasImage {
			return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "list_attachments", Arguments: map[string]any{}}}}
		}
	}

	if strings.Contains(lower, "体检") || strings.Contains(lower, "质检") || strings.Contains(lower, "重复题") || strings.Contains(lower, "题库检查") || strings.Contains(lower, "质量检查") {
		return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "bank_audit", Arguments: map[string]any{}}}}
	}
	if containsAny(lower, "补解析", "补答案", "缺少解析", "缺少答案", "补充解析", "补充答案") {
		return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "fill_explanations", Arguments: filterAgentArgs(map[string]any{
			"subject": subject, "grade": grade, "knowledge_point": knowledgePoint, "count": count,
		})}}}
	}
	if containsAny(lower, "整理", "分类", "打标签", "补知识点", "整理题库", "归类") {
		return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "organize_drafts", Arguments: filterAgentArgs(map[string]any{
			"subject": subject, "grade": grade, "knowledge_point": knowledgePoint, "count": count,
		})}}}
	}
	if containsAny(lower, "变式", "变体", "举一反三", "改一道题") {
		questionID := firstNonEmpty(chatUUIDPattern.FindString(goal), lastQuestionID(prior))
		if questionID == "" {
			return ChatTurnPlan{Clarify: true, Reply: "请提供要生成变式的题目 ID，或先在题库里打开一道题再来找我。"}
		}
		return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "generate_variants", Arguments: map[string]any{
			"question_id": questionID, "count": count,
		}}}}
	}
	if containsAny(lower, "组卷", "试卷", "练习单", "复习卷", "测试卷", "出一份卷") {
		if grade == "" {
			return ChatTurnPlan{Clarify: true, Reply: "我可以为你组卷。请补充年级和学科，例如“用已发布题目给八年级数学组一份 100 分试卷”。"}
		}
		return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "compose_paper", Arguments: filterAgentArgs(map[string]any{
			"subject": subject, "grade": grade, "knowledge_point": knowledgePoint, "count": count,
		})}}}
	}
	if containsAny(lower, "出题", "生成题", "新题", "出一道", "出几道", "来几道") || chatGenerateAskPattern.MatchString(goal) {
		if subject == "" || grade == "" {
			return ChatTurnPlan{Clarify: true, Reply: "请告诉我要出题的学科和年级，例如“给八年级数学出 3 道一元一次方程题”。"}
		}
		return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "generate_questions", Arguments: filterAgentArgs(map[string]any{
			"subject": subject, "grade": grade, "knowledge_point": knowledgePoint, "count": count,
		})}}}
	}
	if containsAny(lower, "搜索", "查找", "找一", "有哪些题", "查看题目", "查一下") {
		keyword := firstNonEmpty(extractQuoted(goal), knowledgePoint)
		return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "search_questions", Arguments: filterAgentArgs(map[string]any{
			"keyword": keyword, "subject": subject, "grade": grade, "limit": 20,
		})}}}
	}
	if containsAny(lower, "分析这道", "分析题目", "看看这道题") {
		if questionID := chatUUIDPattern.FindString(goal); questionID != "" {
			return ChatTurnPlan{Calls: []PlannedToolCall{{Name: "analyze_question", Arguments: map[string]any{
				"question_id": questionID, "apply": true,
			}}}}
		}
	}
	return ChatTurnPlan{
		Reply: "我可以调用题库工具帮你：文档智能录题、整卷拆题、题库体检、整理草稿、补答案解析、知识点出题、生成变式、智能组卷、相似题检索。\n\n" +
			"试试直接说目标，例如：“把这份 Word 里的题导入八年级数学题库”“体检一下题库”“给八年级数学出 3 道一元一次方程题”。",
	}
}

func filterAgentArgs(args map[string]any) map[string]any {
	result := make(map[string]any, len(args))
	for key, value := range args {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				result[key] = typed
			}
		case int:
			if typed > 0 {
				result[key] = typed
			}
		default:
			if value != nil {
				result[key] = value
			}
		}
	}
	return result
}

func lastQuestionID(messages []ChatMessage) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if id := chatUUIDPattern.FindString(messages[index].Content); id != "" {
			return id
		}
		for _, call := range messages[index].ToolCalls {
			if len(call.Result) == 0 {
				continue
			}
			switch ids := call.Result["question_ids"].(type) {
			case []any:
				for _, item := range ids {
					if text, ok := item.(string); ok && chatUUIDPattern.MatchString(text) {
						return text
					}
				}
			case []string:
				for _, text := range ids {
					if chatUUIDPattern.MatchString(text) {
						return text
					}
				}
			}
		}
	}
	return ""
}

func summarizeToolCalls(calls []ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	builder := strings.Builder{}
	builder.WriteString("本轮执行结果：\n")
	for _, call := range calls {
		builder.WriteString("- **")
		builder.WriteString(call.Label)
		builder.WriteString("**：")
		if call.Status == ToolStatusSucceeded {
			builder.WriteString(call.Summary)
		} else if call.Error != "" {
			builder.WriteString("执行失败：")
			builder.WriteString(call.Error)
		} else {
			builder.WriteString(call.Summary)
		}
		builder.WriteString("\n")
	}
	builder.WriteString("\n所有自动写入都只作用于草稿并已记录审计；生成内容需要人工复核。")
	return builder.String()
}

func previewAttachmentContext(attachments []AttachmentRef, limit int) string {
	if len(attachments) == 0 {
		return ""
	}
	builder := strings.Builder{}
	builder.WriteString("\n\n本轮附件：\n")
	for _, item := range attachments {
		builder.WriteString("- ")
		builder.WriteString(item.Name)
		builder.WriteString("（id=")
		builder.WriteString(item.ID)
		builder.WriteString("，")
		builder.WriteString(item.Kind)
		if item.Kind == AttachmentKindImage {
			builder.WriteString("，图片已作为视觉输入提供，可直接阅读")
		} else {
			builder.WriteString("，")
			builder.WriteString(fmt.Sprintf("%d 字符", item.TextChars))
			if item.TextChars == 0 && item.Status == AttachmentStatusParsed {
				builder.WriteString("，未抽取到文本层")
			}
		}
		if item.Status != AttachmentStatusParsed {
			builder.WriteString("，解析失败")
		}
		builder.WriteString("）\n")
		if preview := textPreview(item.TextPreview, limit); preview != "" {
			builder.WriteString("  预览：")
			builder.WriteString(preview)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}
