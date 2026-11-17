import { z } from "zod";
import { apiRequest } from "./api";
import { questionSchema, questionTypeSchema } from "./question-schemas";

export const aiProviderSchema = z.enum(["local", "llm", "local_fallback", "none"]);

const aiOptionSchema = z.object({ key: z.string(), content: z.string() });

export const aiAnalysisSchema = z.object({
  subject: z.string(),
  grade: z.string(),
  question_type: questionTypeSchema,
  category: z.string(),
  difficulty: z.number().int().min(1).max(5),
  tags: z.array(z.string()),
  knowledge_points: z.array(z.string()),
  answer: z.string(),
  explanation: z.string(),
  confidence: z.number(),
  provider: aiProviderSchema,
  model: z.string(),
  review_required: z.boolean(),
  rationale: z.string(),
  warnings: z.array(z.string()).default([]),
});

export const aiGeneratedQuestionSchema = z.object({
  content: z.string(),
  options: z.array(aiOptionSchema),
  answer: z.string(),
  explanation: z.string(),
  question_type: questionTypeSchema,
  difficulty: z.number().int().min(1).max(5),
  category: z.string(),
  tags: z.array(z.string()),
  knowledge_points: z.array(z.string()),
  rationale: z.string(),
  review_required: z.boolean(),
  provider: z.string().optional(),
});

export const aiGenerateResultSchema = z.object({
  items: z.array(aiGeneratedQuestionSchema),
  provider: z.string(),
  model: z.string(),
  persisted: z.array(aiGeneratedQuestionSchema).optional(),
  question_ids: z.array(z.string()).optional(),
  warnings: z.array(z.string()).default([]),
});

export const aiVariantResultSchema = aiGenerateResultSchema;

export const aiDuplicateGroupSchema = z.object({
  content_hash: z.string(),
  question_ids: z.array(z.string()),
  preview: z.string(),
});

export const aiAuditFindingSchema = z.object({
  code: z.string(),
  severity: z.string(),
  message: z.string(),
  question_ids: z.array(z.string()),
  count: z.number().int(),
});

export const aiAuditSchema = z.object({
  total: z.number().int(),
  missing_answer: z.number().int(),
  missing_explanation: z.number().int(),
  missing_knowledge_points: z.number().int(),
  missing_category: z.number().int(),
  missing_tags: z.number().int(),
  duplicate_count: z.number().int(),
  duplicates: z.array(aiDuplicateGroupSchema),
  difficulty: z.record(z.string(), z.number()),
  category_distribution: z.record(z.string(), z.number()),
  findings: z.array(aiAuditFindingSchema),
  provider: z.string(),
  warnings: z.array(z.string()).default([]),
});

export const aiStatusSchema = z.object({
  provider: aiProviderSchema,
  model: z.string(),
  llm_enabled: z.boolean(),
  models: z.array(z.string()).default([]),
  default_model: z.string().default(""),
  source: z.string().default("local"),
  warning: z.string().optional(),
  fallback_policy: z.string().default("error"),
  models_health: z.array(z.object({
    model: z.string(),
    status: z.string(),
    failures: z.number().int().default(0),
    retry_after_seconds: z.number().int().default(0),
  })).default([]),
  limits: z.object({
    max_analyze_chars: z.number().int(),
    max_generate: z.number().int(),
    max_batch: z.number().int(),
  }),
  capabilities: z.array(z.string()),
  safety: z.array(z.string()),
});

export const aiApplyResultSchema = z.object({
  analysis: aiAnalysisSchema,
  applied: z.boolean(),
  question: questionSchema.optional(),
});

export const aiStepSchema = z.object({
  key: z.string(),
  title: z.string(),
  status: z.string(),
  summary: z.string(),
  affected: z.number().int(),
  question_ids: z.array(z.string()).optional(),
  warning: z.string().optional(),
});

export const aiRunSchema = z.object({
  id: z.string().uuid(),
  goal: z.string(),
  intent: z.string(),
  status: z.string(),
  provider: z.string(),
  model: z.string(),
  plan: z.array(aiStepSchema),
  steps: z.array(aiStepSchema),
  result: z.record(z.string(), z.unknown()),
  error: z.string().optional(),
  created_at: z.string(),
});

export const aiRunListSchema = z.object({ items: z.array(aiRunSchema) });

export const aiAuditLogSchema = z.object({
  id: z.string().uuid(),
  actor_id: z.string().uuid(),
  run_id: z.string().uuid().optional(),
  action: z.string(),
  target_type: z.string(),
  target_id: z.string().uuid().optional(),
  detail: z.record(z.string(), z.unknown()),
  created_at: z.string(),
});

export const aiAuditLogListSchema = z.object({ items: z.array(aiAuditLogSchema) });

export interface AIAnalyzeInput {
  subject?: string;
  grade?: string;
  question_type?: z.infer<typeof questionTypeSchema>;
  difficulty?: number;
  tags?: string[];
  category?: string;
  knowledge_points?: string[];
  content: string;
  options?: Array<{ key: string; content: string }>;
  answer?: string;
  explanation?: string;
  model?: string;
}

export interface AIGenerateInput {
  subject: string;
  grade: string;
  question_type?: z.infer<typeof questionTypeSchema>;
  difficulty?: number;
  category?: string;
  knowledge_points?: string[];
  tags?: string[];
  count: number;
  reference?: string;
  persist?: boolean;
  model?: string;
}

export interface AIVariantInput {
  count: number;
  strategy?: "numbers" | "context" | "reverse" | "difficulty";
  difficulty_delta?: number;
  persist?: boolean;
  model?: string;
}

export interface AIAgentRequest {
  goal: string;
  subject?: string;
  grade?: string;
  category?: string;
  knowledge_point?: string;
  tag?: string;
  question_id?: string;
  count?: number;
  model?: string;
}

export type AIAnalysis = z.infer<typeof aiAnalysisSchema>;
export type AIGeneratedQuestion = z.infer<typeof aiGeneratedQuestionSchema>;
export type AIGenerateResult = z.infer<typeof aiGenerateResultSchema>;
export type AIVariantResult = z.infer<typeof aiVariantResultSchema>;
export type AIAudit = z.infer<typeof aiAuditSchema>;
export type AIStatus = z.infer<typeof aiStatusSchema>;
export type AIApplyResult = z.infer<typeof aiApplyResultSchema>;
export type AIRun = z.infer<typeof aiRunSchema>;
export type AIRunStep = z.infer<typeof aiStepSchema>;
export type AIAuditLog = z.infer<typeof aiAuditLogSchema>;

export function getAIStatus() {
  return apiRequest("/api/v1/ai/status", aiStatusSchema);
}

export function analyzeDraft(input: AIAnalyzeInput) {
  return apiRequest("/api/v1/ai/analyze", aiAnalysisSchema, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function analyzeQuestion(id: string, apply: boolean) {
  return apiRequest(`/api/v1/ai/questions/${encodeURIComponent(id)}/analyze`, aiApplyResultSchema, {
    method: "POST",
    body: JSON.stringify({ apply }),
  });
}

export function generateVariants(id: string, input: AIVariantInput) {
  return apiRequest(`/api/v1/ai/questions/${encodeURIComponent(id)}/variants`, aiVariantResultSchema, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function generateQuestions(input: AIGenerateInput) {
  return apiRequest("/api/v1/ai/generate", aiGenerateResultSchema, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function auditQuestionBank(limit = 500) {
  return apiRequest(`/api/v1/ai/bank/audit?limit=${limit}`, aiAuditSchema);
}

export function createAgentRun(input: AIAgentRequest) {
  return apiRequest("/api/v1/ai/agent/runs", aiRunSchema, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function listAgentRuns(limit = 20) {
  return apiRequest(`/api/v1/ai/agent/runs?limit=${limit}`, aiRunListSchema);
}

export function getAgentRun(id: string) {
  return apiRequest(`/api/v1/ai/agent/runs/${encodeURIComponent(id)}`, aiRunSchema);
}

export function listAIAuditLogs(limit = 50) {
  return apiRequest(`/api/v1/ai/audit-logs?limit=${limit}`, aiAuditLogListSchema);
}

export const aiQueryKeys = {
  status: ["ai", "status"] as const,
  audit: (limit: number) => ["ai", "audit", limit] as const,
  runs: (limit: number) => ["ai", "runs", limit] as const,
  run: (id: string) => ["ai", "run", id] as const,
  logs: (limit: number) => ["ai", "logs", limit] as const,
};
