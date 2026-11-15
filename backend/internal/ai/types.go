package ai

import (
	"errors"
	"time"
)

const (
	ProviderLocal    = "local"
	ProviderLLM      = "llm"
	ProviderFallback = "local_fallback"
	ProviderNone     = "none"

	DefaultMaxAnalyzeChars = 20000
	DefaultMaxGenerate     = 10
	DefaultMaxBatch        = 10
)

var (
	ErrValidation = errors.New("ai validation failed")
	ErrNotFound   = errors.New("ai resource not found")
	ErrForbidden  = errors.New("ai operation forbidden")
)

type FieldViolation struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct {
	Violation FieldViolation
}

func (e *ValidationError) Error() string { return ErrValidation.Error() }
func (e *ValidationError) Unwrap() error { return ErrValidation }

func validationError(field, message string) error {
	return &ValidationError{Violation: FieldViolation{Field: field, Message: message}}
}

type Option struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

type AnalyzeInput struct {
	Subject         string   `json:"subject"`
	Grade           string   `json:"grade"`
	QuestionType    string   `json:"question_type"`
	Difficulty      int      `json:"difficulty"`
	Tags            []string `json:"tags"`
	Category        string   `json:"category"`
	KnowledgePoints []string `json:"knowledge_points"`
	Content         string   `json:"content"`
	Options         []Option `json:"options"`
	Answer          string   `json:"answer"`
	Explanation     string   `json:"explanation"`
}

type Analysis struct {
	Subject         string   `json:"subject"`
	Grade           string   `json:"grade"`
	QuestionType    string   `json:"question_type"`
	Category        string   `json:"category"`
	Difficulty      int      `json:"difficulty"`
	Tags            []string `json:"tags"`
	KnowledgePoints []string `json:"knowledge_points"`
	Answer          string   `json:"answer"`
	Explanation     string   `json:"explanation"`
	Confidence      float64  `json:"confidence"`
	Provider        string   `json:"provider"`
	Model           string   `json:"model"`
	ReviewRequired  bool     `json:"review_required"`
	Rationale       string   `json:"rationale"`
	Warnings        []string `json:"warnings"`
}

type GenerateInput struct {
	Subject         string   `json:"subject"`
	Grade           string   `json:"grade"`
	QuestionType    string   `json:"question_type"`
	Difficulty      int      `json:"difficulty"`
	Category        string   `json:"category"`
	KnowledgePoints []string `json:"knowledge_points"`
	Tags            []string `json:"tags"`
	Count           int      `json:"count"`
	Reference       string   `json:"reference"`
	Persist         bool     `json:"persist"`
}

type GeneratedQuestion struct {
	Content         string   `json:"content"`
	Options         []Option `json:"options"`
	Answer          string   `json:"answer"`
	Explanation     string   `json:"explanation"`
	QuestionType    string   `json:"question_type"`
	Difficulty      int      `json:"difficulty"`
	Category        string   `json:"category"`
	Tags            []string `json:"tags"`
	KnowledgePoints []string `json:"knowledge_points"`
	Rationale       string   `json:"rationale"`
	ReviewRequired  bool     `json:"review_required"`
	Provider        string   `json:"provider,omitempty"`
}

type GenerateResult struct {
	Items       []GeneratedQuestion `json:"items"`
	Provider    string              `json:"provider"`
	Model       string              `json:"model"`
	Persisted   []GeneratedQuestion `json:"persisted,omitempty"`
	QuestionIDs []string            `json:"question_ids,omitempty"`
	Warnings    []string            `json:"warnings"`
}

type VariantInput struct {
	Count           int    `json:"count"`
	Strategy        string `json:"strategy"`
	DifficultyDelta int    `json:"difficulty_delta"`
	Persist         bool   `json:"persist"`
}

type VariantResult struct {
	Items       []GeneratedQuestion `json:"items"`
	Persisted   []GeneratedQuestion `json:"persisted,omitempty"`
	QuestionIDs []string            `json:"question_ids,omitempty"`
	Provider    string              `json:"provider"`
	Model       string              `json:"model"`
	Warnings    []string            `json:"warnings"`
}

type DuplicateGroup struct {
	ContentHash string   `json:"content_hash"`
	QuestionIDs []string `json:"question_ids"`
	Preview     string   `json:"preview"`
}

type AuditFinding struct {
	Code        string   `json:"code"`
	Severity    string   `json:"severity"`
	Message     string   `json:"message"`
	QuestionIDs []string `json:"question_ids"`
	Count       int      `json:"count"`
}

type AuditResult struct {
	Total                  int              `json:"total"`
	MissingAnswer          int              `json:"missing_answer"`
	MissingExplanation     int              `json:"missing_explanation"`
	MissingKnowledgePoints int              `json:"missing_knowledge_points"`
	MissingCategory        int              `json:"missing_category"`
	MissingTags            int              `json:"missing_tags"`
	DuplicateCount         int              `json:"duplicate_count"`
	Duplicates             []DuplicateGroup `json:"duplicates"`
	Difficulty             map[int]int      `json:"difficulty"`
	CategoryDistribution   map[string]int   `json:"category_distribution"`
	Findings               []AuditFinding   `json:"findings"`
	Provider               string           `json:"provider"`
	Warnings               []string         `json:"warnings"`
}

type Limits struct {
	MaxAnalyzeChars int `json:"max_analyze_chars"`
	MaxGenerate     int `json:"max_generate"`
	MaxBatch        int `json:"max_batch"`
}

type Status struct {
	Provider     string   `json:"provider"`
	Model        string   `json:"model"`
	LLMEnabled   bool     `json:"llm_enabled"`
	Limits       Limits   `json:"limits"`
	Capabilities []string `json:"capabilities"`
	Safety       []string `json:"safety"`
}

type QuestionSnapshot struct {
	ID               string
	CreatorID        string
	Subject          string
	Grade            string
	QuestionType     string
	Difficulty       int
	Tags             []string
	Category         string
	KnowledgePoints  []string
	Content          string
	Options          []Option
	Answer           string
	Explanation      string
	Status           string
	AIGenerated      bool
	ContentHash      string
	SourceQuestionID string
	CreatedAt        time.Time
}

type ApplyResult struct {
	Analysis Analysis `json:"analysis"`
	Applied  bool     `json:"applied"`
	Question any      `json:"question,omitempty"`
}

type Step struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Summary     string   `json:"summary"`
	Affected    int      `json:"affected"`
	QuestionIDs []string `json:"question_ids,omitempty"`
	Warning     string   `json:"warning,omitempty"`
}

type AgentRequest struct {
	Goal           string `json:"goal"`
	Subject        string `json:"subject"`
	Grade          string `json:"grade"`
	Category       string `json:"category"`
	KnowledgePoint string `json:"knowledge_point"`
	Tag            string `json:"tag"`
	QuestionID     string `json:"question_id"`
	Count          int    `json:"count"`
}

type AgentRun struct {
	ID        string         `json:"id"`
	Goal      string         `json:"goal"`
	Intent    string         `json:"intent"`
	Status    string         `json:"status"`
	Provider  string         `json:"provider"`
	Model     string         `json:"model"`
	Plan      []Step         `json:"plan"`
	Steps     []Step         `json:"steps"`
	Result    map[string]any `json:"result"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type AuditLog struct {
	ID         string         `json:"id"`
	ActorID    string         `json:"actor_id"`
	RunID      string         `json:"run_id,omitempty"`
	Action     string         `json:"action"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id,omitempty"`
	Detail     map[string]any `json:"detail"`
	CreatedAt  time.Time      `json:"created_at"`
}

type AgentParams struct {
	Subject        string `json:"subject,omitempty"`
	Grade          string `json:"grade,omitempty"`
	Category       string `json:"category,omitempty"`
	KnowledgePoint string `json:"knowledge_point,omitempty"`
	Tag            string `json:"tag,omitempty"`
	QuestionID     string `json:"question_id,omitempty"`
	Count          int    `json:"count,omitempty"`
}
