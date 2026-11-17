package ai

import (
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"school-exam-studio/backend/internal/question"
)

// LocalEngine 是无模型密钥时的确定性引擎。它不臆造选择题答案，所有结果都标记为待人工复核。
type LocalEngine struct{}

func NewLocalEngine() *LocalEngine { return &LocalEngine{} }

var subjectKeywords = map[string][]string{
	"数学": {"函数", "方程", "不等式", "几何", "三角形", "圆", "概率", "统计", "数列", "向量", "导数", "集合", "分数", "小数", "整数", "面积", "体积", "求证", "化简", "解方程"},
	"语文": {"古诗", "文言文", "现代文", "阅读", "作文", "修辞", "病句", "词语", "句子", "拼音", "标点", "作者", "诗意", "下列各句"},
	"英语": {"english", "choose", "passage", "grammar", "vocabulary", "reading", "word", "sentence", "blank", "the best answer"},
	"物理": {"力", "速度", "加速度", "电路", "电阻", "电压", "电流", "浮力", "功率", "机械能", "光的", "声音", "压强"},
	"化学": {"化学", "元素", "分子", "原子", "溶液", "氧化", "反应", "化学式", "离子", "酸碱", "燃烧"},
	"生物": {"细胞", "基因", "生态", "蛋白质", "光合作用", "遗传", "种群", "免疫", "组织"},
	"历史": {"朝代", "历史", "革命", "战争", "世纪", "皇帝", "变法", "起义", "条约"},
	"地理": {"地球", "气候", "地图", "经纬", "地形", "洋流", "人口", "城市化", "板块"},
	"政治": {"政治", "法治", "经济", "社会", "国家", "权利", "义务", "市场", "政府"},
}

var knowledgeDictionary = map[string][]string{
	"数学": {"函数", "一次函数", "二次函数", "反比例函数", "方程", "一元一次方程", "一元二次方程", "不等式", "三角形", "全等三角形", "相似三角形", "圆", "四边形", "概率", "统计", "数列", "向量", "导数", "集合", "运算", "面积", "体积", "锐角三角函数"},
	"语文": {"古诗文默写", "文言文阅读", "现代文阅读", "词语运用", "病句辨析", "修辞手法", "标点符号", "作文", "名著阅读", "句子衔接"},
	"英语": {"词汇", "语法", "时态", "从句", "阅读理解", "完形填空", "书面表达", "非谓语动词"},
	"物理": {"力学", "运动学", "牛顿定律", "电学", "电路", "欧姆定律", "电功率", "光学", "声现象", "压强", "浮力", "机械能"},
	"化学": {"物质构成", "元素周期表", "化学方程式", "溶液", "酸碱盐", "氧化还原", "燃烧与灭火"},
	"生物": {"细胞结构", "遗传与变异", "生态系统", "光合作用", "人体生理", "免疫调节"},
	"历史": {"中国古代史", "中国近代史", "世界近代史", "改革开放", "辛亥革命", "工业革命"},
	"地理": {"地球运动", "气候类型", "地形地貌", "洋流", "人口迁移", "区域发展"},
	"政治": {"法治意识", "市场经济", "国家制度", "公民权利", "社会责任"},
}

// subjectOrder 保证本地引擎遍历学科时使用固定顺序，避免 map 迭代随机导致结果不可重复。
var subjectOrder = []string{"数学", "语文", "英语", "物理", "化学", "生物", "历史", "地理", "政治"}

var gradeKeywords = []string{
	"一年级", "二年级", "三年级", "四年级", "五年级", "六年级",
	"七年级", "八年级", "九年级", "初一", "初二", "初三",
	"高一", "高二", "高三",
}

var (
	htmlTagPattern  = regexp.MustCompile(`<[^>]*>`)
	spacesPattern   = regexp.MustCompile(`\s+`)
	arithmeticWords = []string{"加法", "减法", "乘法", "除法", "四则运算", "口算", "混合运算", "运算"}
)

func (e *LocalEngine) Analyze(input AnalyzeInput) Analysis {
	text := plainText(input.Content)
	subject := firstNonEmpty(input.Subject, detectSubject(text), "通用")
	grade := firstNonEmpty(input.Grade, detectGrade(text))
	questionType := detectQuestionType(input)
	knowledgePoints := extractKnowledgePoints(input, text)
	category := firstNonEmpty(input.Category, inferCategory(subject, knowledgePoints, text), "综合应用")
	difficulty := inferDifficulty(input.Difficulty, text, questionType)
	tags := buildTags(input.Tags, knowledgePoints, category, questionType, difficulty)
	warnings := []string{"本地模式未调用大模型，建议人工复核分析结果"}

	answer := strings.TrimSpace(input.Answer)
	if answer == "" && (questionType == question.TypeSingleChoice || questionType == question.TypeMultipleChoice || questionType == question.TypeTrueFalse) {
		warnings = append(warnings, "本地模式不会臆造客观题答案，请人工确认")
	}
	explanation := strings.TrimSpace(input.Explanation)
	if explanation == "" {
		explanation = buildExplanation(subject, knowledgePoints, category, answer)
	}
	return Analysis{
		Subject:         subject,
		Grade:           grade,
		QuestionType:    questionType,
		Category:        category,
		Difficulty:      difficulty,
		Tags:            tags,
		KnowledgePoints: knowledgePoints,
		Answer:          answer,
		Explanation:     explanation,
		Confidence:      0.55,
		Provider:        ProviderLocal,
		Model:           "",
		ReviewRequired:  true,
		Rationale:       "基于学科关键词、题型与结构规则生成",
		Warnings:        warnings,
	}
}

func (e *LocalEngine) Generate(input GenerateInput) ([]GeneratedQuestion, []string) {
	count := input.Count
	if count < 1 {
		count = 3
	}
	subject := firstNonEmpty(input.Subject, "通用")
	grade := input.Grade
	knowledgePoint := firstNonEmpty(strings.Join(input.KnowledgePoints, "、"), input.Category, input.Reference, "核心知识")
	category := firstNonEmpty(input.Category, inferCategory(subject, input.KnowledgePoints, knowledgePoint), "综合应用")
	questionType := input.QuestionType
	if !question.ValidType(questionType) {
		questionType = question.TypeSingleChoice
	}
	difficulty := input.Difficulty
	if difficulty < 1 || difficulty > 5 {
		difficulty = 3
	}
	warnings := []string{"本地模式生成的是可编辑草稿，答案解析需人工复核"}
	items := make([]GeneratedQuestion, 0, count)
	for index := 0; index < count; index++ {
		item := generateLocalItem(subject, grade, knowledgePoint, category, questionType, difficulty, index)
		tags := append([]string{}, input.Tags...)
		tags = append(tags, item.Tags...)
		tags = append(tags, subject, category, typeLabel(item.QuestionType))
		if grade != "" {
			tags = append(tags, grade)
		}
		item.Tags = uniqueStrings(tags)
		if len(item.Tags) > 10 {
			item.Tags = item.Tags[:10]
		}
		item.KnowledgePoints = uniqueStrings(append(append([]string{}, input.KnowledgePoints...), knowledgePoint))
		if len(item.KnowledgePoints) > 8 {
			item.KnowledgePoints = item.KnowledgePoints[:8]
		}
		item.Provider = ProviderLocal
		if item.Answer == "" {
			warnings = append(warnings, "部分题目未生成答案，请在保存前补充")
		}
		items = append(items, item)
	}
	return items, uniqueStrings(warnings)
}

func (e *LocalEngine) Variants(source QuestionSnapshot, input VariantInput) ([]GeneratedQuestion, []string) {
	count := input.Count
	if count < 1 {
		count = 3
	}
	knowledgePoint := firstNonEmpty(strings.Join(source.KnowledgePoints, "、"), source.Category, "原题知识点")
	category := firstNonEmpty(source.Category, "综合应用")
	warnings := []string{"本地变式通过改写题干数字生成，请逐题核验答案与解析"}
	// 本地变式不修改选项，因此客观题可沿用原题答案；否则占位答案无法通过校验，草稿将无法保存。
	answer := "请根据变式后的条件重新计算并核对（待人工确认）"
	switch source.QuestionType {
	case question.TypeSingleChoice, question.TypeMultipleChoice, question.TypeTrueFalse:
		if value, err := question.ValidateAnswer(source.QuestionType, toQuestionOptions(source.Options), source.Answer); err == nil {
			answer = value
			warnings = append(warnings, "客观题选项未改动，答案沿用原题，请核对题干改写后的正确性")
		}
	}
	options := append([]Option{}, source.Options...)
	knowledgePoints := append([]string{}, source.KnowledgePoints...)
	items := make([]GeneratedQuestion, 0, count)
	for index := 0; index < count; index++ {
		difficulty := source.Difficulty + input.DifficultyDelta
		if difficulty < 1 {
			difficulty = 1
		}
		if difficulty > 5 {
			difficulty = 5
		}
		content, changed := question.ShiftRichTextNumbers(source.Content, index+1)
		if !changed {
			content = fmt.Sprintf(`<p>变式 %d：请参考原题结构，围绕“%s”调整题设条件后完成作答。</p>`, index+1, html.EscapeString(knowledgePoint))
			warnings = append(warnings, "原题未包含可替换数字，已生成情境改写框架")
		}
		tags := uniqueStrings(append(append([]string{}, source.Tags...), "变式"))
		if len(tags) > 10 {
			tags = tags[:10]
		}
		items = append(items, GeneratedQuestion{
			Content:         content,
			Options:         append([]Option{}, options...),
			Answer:          answer,
			Explanation:     fmt.Sprintf(`<p>本题为原题的变式草稿，考查知识点“%s”。请根据调整后的条件重新推导并核对答案。</p>`, html.EscapeString(knowledgePoint)),
			QuestionType:    source.QuestionType,
			Difficulty:      difficulty,
			Category:        category,
			Tags:            tags,
			KnowledgePoints: append([]string{}, knowledgePoints...),
			Rationale:       "本地引擎对题干文本节点中的数字做了偏移改写",
			ReviewRequired:  true,
			Provider:        ProviderLocal,
		})
	}
	return items, uniqueStrings(warnings)
}

func (e *LocalEngine) Audit(items []QuestionSnapshot) AuditResult {
	result := AuditResult{
		Total:                len(items),
		Difficulty:           map[int]int{},
		CategoryDistribution: map[string]int{},
		Duplicates:           []DuplicateGroup{},
		Findings:             []AuditFinding{},
		Provider:             ProviderLocal,
		Warnings:             []string{"题库体检基于可见题目快照，最多统计最近1000道"},
	}
	hashes := map[string][]string{}
	previews := map[string]string{}
	var missingAnswerIDs, missingExplanationIDs, missingKnowledgeIDs, missingCategoryIDs, missingTagIDs, pendingReviewIDs []string
	for _, item := range items {
		if strings.TrimSpace(item.Answer) == "" {
			result.MissingAnswer++
			missingAnswerIDs = append(missingAnswerIDs, item.ID)
		}
		if strings.TrimSpace(item.Explanation) == "" {
			result.MissingExplanation++
			missingExplanationIDs = append(missingExplanationIDs, item.ID)
		}
		if len(item.KnowledgePoints) == 0 {
			result.MissingKnowledgePoints++
			missingKnowledgeIDs = append(missingKnowledgeIDs, item.ID)
		}
		if strings.TrimSpace(item.Category) == "" {
			result.MissingCategory++
			missingCategoryIDs = append(missingCategoryIDs, item.ID)
		}
		if len(item.Tags) == 0 {
			result.MissingTags++
			missingTagIDs = append(missingTagIDs, item.ID)
		}
		result.Difficulty[item.Difficulty]++
		category := item.Category
		if category == "" {
			category = "未分类"
		}
		result.CategoryDistribution[category]++
		hash := item.ContentHash
		if hash == "" {
			hash = question.ContentHash(question.Input{
				Content: item.Content,
				Options: toQuestionOptions(item.Options),
			})
		}
		hashes[hash] = append(hashes[hash], item.ID)
		if _, exists := previews[hash]; !exists {
			previews[hash] = truncate(plainText(item.Content), 40)
		}
		if item.AIGenerated && item.Status == question.StatusDraft {
			pendingReviewIDs = append(pendingReviewIDs, item.ID)
		}
	}
	hashKeys := make([]string, 0, len(hashes))
	for hash := range hashes {
		hashKeys = append(hashKeys, hash)
	}
	sort.Strings(hashKeys)
	for _, hash := range hashKeys {
		ids := hashes[hash]
		if len(ids) > 1 {
			result.Duplicates = append(result.Duplicates, DuplicateGroup{
				ContentHash: hash,
				QuestionIDs: ids,
				Preview:     previews[hash],
			})
		}
	}
	sort.Slice(result.Duplicates, func(i, j int) bool {
		if len(result.Duplicates[i].QuestionIDs) != len(result.Duplicates[j].QuestionIDs) {
			return len(result.Duplicates[i].QuestionIDs) > len(result.Duplicates[j].QuestionIDs)
		}
		return result.Duplicates[i].ContentHash < result.Duplicates[j].ContentHash
	})
	result.DuplicateCount = len(result.Duplicates)
	result.Findings = append(result.Findings,
		finding("missing_answer", "high", "缺少答案", missingAnswerIDs),
		finding("missing_explanation", "medium", "缺少解析", missingExplanationIDs),
		finding("missing_knowledge_points", "high", "缺少知识点", missingKnowledgeIDs),
		finding("missing_category", "medium", "缺少分类", missingCategoryIDs),
		finding("missing_tags", "low", "缺少标签", missingTagIDs),
	)
	if len(result.Duplicates) > 0 {
		ids := make([]string, 0)
		for _, group := range result.Duplicates {
			ids = append(ids, group.QuestionIDs...)
		}
		result.Findings = append(result.Findings, finding("duplicates", "high", "存在内容重复的题目", ids))
	}
	if len(pendingReviewIDs) > 0 {
		result.Findings = append(result.Findings, finding("ai_pending_review", "medium", "AI 生成草稿待复核", pendingReviewIDs))
	}
	return result
}

func generateLocalItem(subject, grade, knowledgePoint, category, questionType string, difficulty, index int) GeneratedQuestion {
	item := GeneratedQuestion{
		Options:         []Option{},
		QuestionType:    questionType,
		Difficulty:      difficulty,
		Category:        category,
		KnowledgePoints: []string{knowledgePoint},
		ReviewRequired:  true,
		Rationale:       "本地模板生成，需人工完善",
	}
	if isArithmetic(knowledgePoint) {
		first := 12 + index*3
		second := 7 + index*2
		item.QuestionType = question.TypeFillBlank
		item.Content = fmt.Sprintf(`<p>计算：%d + %d = ______。</p>`, first, second)
		item.Answer = strconv.Itoa(first + second)
		item.Explanation = fmt.Sprintf(`<p>本题考查两位数加法。把个位与十位分别相加，得到 %d。</p>`, first+second)
		item.Difficulty = minInt(5, maxInt(1, difficulty))
		return item
	}
	switch questionType {
	case question.TypeTrueFalse:
		item.Content = fmt.Sprintf(`<p>判断：学习“%s”时，需要结合具体条件进行分析。（ ）</p>`, html.EscapeString(knowledgePoint))
		item.Answer = "true"
		item.Explanation = fmt.Sprintf(`<p>本题考查“%s”的基本理解。该知识点通常需要结合条件判断，不能只背结论。</p>`, html.EscapeString(knowledgePoint))
	case question.TypeMultipleChoice:
		item.Content = fmt.Sprintf(`<p>关于“%s”，下列说法正确的有（ ）。</p>`, html.EscapeString(knowledgePoint))
		item.Options = []Option{
			{Key: "A", Content: "需要结合具体条件进行分析"},
			{Key: "B", Content: "可以通过举例或验证加深理解"},
			{Key: "C", Content: "与本知识点完全无关"},
			{Key: "D", Content: "只需要记忆结论即可"},
		}
		item.Answer = "A,B"
		item.Explanation = fmt.Sprintf(`<p>本题考查“%s”。A、B 两项体现了结合条件与举例验证的学习方法；C、D 两项表述错误。</p>`, html.EscapeString(knowledgePoint))
	case question.TypeFillBlank:
		item.Content = fmt.Sprintf(`<p>请写出“%s”的一个核心结论：______。</p>`, html.EscapeString(knowledgePoint))
		item.Answer = "核心结论（请依据教材补充并核对）"
		item.Explanation = fmt.Sprintf(`<p>本题考查“%s”的核心结论，请依据教材表述作答。</p>`, html.EscapeString(knowledgePoint))
	case question.TypeEssay:
		item.Content = fmt.Sprintf(`<p>请结合“%s”，说明其基本思路，并给出一个例子。</p>`, html.EscapeString(knowledgePoint))
		item.Answer = "参考要点：说明基本思路，并给出与知识点匹配的例子。"
		item.Explanation = fmt.Sprintf(`<p>评分要点：准确说明“%s”的基本思路；例子与知识点匹配；表述完整。</p>`, html.EscapeString(knowledgePoint))
	default:
		item.QuestionType = question.TypeSingleChoice
		item.Content = fmt.Sprintf(`<p>下列关于“%s”的说法，正确的是（ ）。</p>`, html.EscapeString(knowledgePoint))
		item.Options = []Option{
			{Key: "A", Content: "该知识点只适用于记忆，不涉及分析"},
			{Key: "B", Content: "该知识点需要结合条件进行分析"},
			{Key: "C", Content: "该知识点与其他知识没有联系"},
			{Key: "D", Content: "以上说法都不正确"},
		}
		item.Answer = "B"
		item.Explanation = fmt.Sprintf(`<p>本题考查“%s”的基本理解。B 项强调结合条件分析，符合学科学习方法。</p>`, html.EscapeString(knowledgePoint))
	}
	if grade != "" {
		item.Tags = append(item.Tags, grade)
	}
	if subject != "" {
		item.Tags = append(item.Tags, subject)
	}
	return item
}

func detectSubject(text string) string {
	lower := strings.ToLower(text)
	for _, subject := range subjectOrder {
		if strings.Contains(text, subject) {
			return subject
		}
	}
	best, bestScore := "", 0
	for _, subject := range subjectOrder {
		score := 0
		for _, keyword := range subjectKeywords[subject] {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = subject, score
		}
	}
	if bestScore == 0 {
		return ""
	}
	return best
}

func detectGrade(text string) string {
	for _, grade := range gradeKeywords {
		if strings.Contains(text, grade) {
			return grade
		}
	}
	return ""
}

func detectQuestionType(input AnalyzeInput) string {
	if question.ValidType(input.QuestionType) {
		return input.QuestionType
	}
	if len(input.Options) > 0 {
		return question.TypeSingleChoice
	}
	content := plainText(input.Content)
	switch {
	case strings.Contains(content, "___") || strings.Contains(content, "____"):
		return question.TypeFillBlank
	case strings.Contains(content, "判断"):
		return question.TypeTrueFalse
	case strings.Contains(content, "多选"):
		return question.TypeMultipleChoice
	default:
		return question.TypeEssay
	}
}

func extractKnowledgePoints(input AnalyzeInput, text string) []string {
	points := append([]string{}, input.KnowledgePoints...)
	search := strings.ToLower(text + " " + strings.Join(input.Tags, " ") + " " + input.Answer)
	for _, subject := range subjectOrder {
		for _, keyword := range knowledgeDictionary[subject] {
			if strings.Contains(search, strings.ToLower(keyword)) {
				points = append(points, keyword)
			}
		}
	}
	points = uniqueStrings(points)
	return points[:minInt(len(points), 6)]
}

func inferCategory(subject string, knowledgePoints []string, text string) string {
	joined := strings.Join(knowledgePoints, " ") + " " + text
	switch subject {
	case "数学":
		switch {
		case containsAny(joined, "函数", "方程", "不等式", "代数"):
			return "函数与方程"
		case containsAny(joined, "三角形", "圆", "四边形", "几何", "面积", "体积"):
			return "图形与几何"
		case containsAny(joined, "概率", "统计", "数据"):
			return "统计与概率"
		case containsAny(joined, "运算", "分数", "小数", "整数"):
			return "数与运算"
		}
	case "语文":
		switch {
		case containsAny(joined, "文言文", "古诗", "默写"):
			return "古诗文阅读"
		case containsAny(joined, "阅读", "现代文"):
			return "现代文阅读"
		case containsAny(joined, "作文", "写作"):
			return "写作"
		default:
			return "语言文字运用"
		}
	case "英语":
		switch {
		case containsAny(joined, "阅读", "reading", "passage"):
			return "阅读理解"
		case containsAny(joined, "作文", "表达", "writing"):
			return "书面表达"
		case containsAny(joined, "完形", "cloze"):
			return "完形填空"
		default:
			return "语法与词汇"
		}
	case "物理":
		return pickCategory(joined, map[string][]string{
			"力学":    {"力", "运动", "速度", "压强", "浮力"},
			"电磁学":   {"电", "电路", "电阻", "电压", "电流"},
			"光学与声学": {"光", "声"},
		}, "综合应用")
	case "化学":
		return pickCategory(joined, map[string][]string{
			"物质构成": {"元素", "分子", "原子", "离子"},
			"化学反应": {"反应", "氧化", "燃烧", "方程式"},
			"溶液":   {"溶液", "酸碱"},
		}, "综合应用")
	}
	return "综合应用"
}

func inferDifficulty(provided int, text, questionType string) int {
	if provided >= 1 && provided <= 5 {
		return provided
	}
	score := 2
	length := len([]rune(text))
	if length > 120 {
		score++
	}
	if length > 300 {
		score++
	}
	if containsAny(text, "证明", "综合", "探究", "应用", "分析") {
		score++
	}
	if questionType == question.TypeEssay {
		score++
	}
	return minInt(5, maxInt(1, score))
}

func buildTags(existing, knowledgePoints []string, category, questionType string, difficulty int) []string {
	tags := append([]string{}, existing...)
	tags = append(tags, category, typeLabel(questionType), fmt.Sprintf("难度%d", difficulty))
	for _, point := range knowledgePoints {
		tags = append(tags, point)
	}
	return uniqueStrings(tags)[:minInt(len(uniqueStrings(tags)), 10)]
}

func buildExplanation(subject string, knowledgePoints []string, category, answer string) string {
	points := "本题"
	if len(knowledgePoints) > 0 {
		points = "“" + html.EscapeString(strings.Join(knowledgePoints, "、")) + "”"
	}
	var builder strings.Builder
	builder.WriteString("<p>本题考查")
	builder.WriteString(html.EscapeString(subject))
	builder.WriteString(points)
	builder.WriteString("，属于")
	builder.WriteString(html.EscapeString(category))
	builder.WriteString("。</p><p>解题思路：先提取题干中的已知条件，再依据该知识点的基本方法逐步推导。</p>")
	if strings.TrimSpace(answer) != "" {
		builder.WriteString("<p>参考答案：")
		builder.WriteString(html.EscapeString(strings.TrimSpace(answer)))
		builder.WriteString("</p>")
	} else {
		builder.WriteString("<p>请结合课堂方法完成推导，并核对最终答案。</p>")
	}
	return builder.String()
}

func typeLabel(questionType string) string {
	switch questionType {
	case question.TypeSingleChoice:
		return "单选题"
	case question.TypeMultipleChoice:
		return "多选题"
	case question.TypeTrueFalse:
		return "判断题"
	case question.TypeFillBlank:
		return "填空题"
	case question.TypeEssay:
		return "解答题"
	default:
		return "综合题"
	}
}

func finding(code, severity, message string, ids []string) AuditFinding {
	ids = uniqueStrings(ids)
	limited := ids
	if len(limited) > 10 {
		limited = limited[:10]
	}
	return AuditFinding{
		Code:        code,
		Severity:    severity,
		Message:     message,
		QuestionIDs: limited,
		Count:       len(ids),
	}
}

func isArithmetic(value string) bool {
	return containsAny(value, arithmeticWords...)
}

func pickCategory(text string, mapping map[string][]string, fallback string) string {
	categories := make([]string, 0, len(mapping))
	for category := range mapping {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		if containsAny(text, mapping[category]...) {
			return category
		}
	}
	return fallback
}

func containsAny(value string, keywords ...string) bool {
	lower := strings.ToLower(value)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func plainText(value string) string {
	text := htmlTagPattern.ReplaceAllString(value, " ")
	text = html.UnescapeString(text)
	return strings.TrimSpace(spacesPattern.ReplaceAllString(text, " "))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func toQuestionOptions(options []Option) []question.Option {
	result := make([]question.Option, 0, len(options))
	for _, option := range options {
		result = append(result, question.Option{Key: option.Key, Content: option.Content})
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}
