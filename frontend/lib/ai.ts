import { z } from "zod";
import { apiRequest } from "./api";
import { questionTypeSchema } from "./question-schemas";

export const aiProviderSchema = z.enum(["local", "llm", "local_fallback", "none"]);

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

export type AIAnalysis = z.infer<typeof aiAnalysisSchema>;
export type AIStatus = z.infer<typeof aiStatusSchema>;

export function getAIStatus() {
  return apiRequest("/api/v1/ai/status", aiStatusSchema);
}

export function analyzeDraft(input: AIAnalyzeInput) {
  return apiRequest("/api/v1/ai/analyze", aiAnalysisSchema, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export const aiQueryKeys = {
  status: ["ai", "status"] as const,
};
