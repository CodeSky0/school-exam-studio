import { z } from "zod";
import { apiRequest } from "./api";

export const aiToolResultSchema = z.object({
  summary: z.string().default(""),
  data: z.record(z.string(), z.unknown()).default({}),
  question_ids: z.array(z.string()).default([]),
  warnings: z.array(z.string()).default([]),
});

export const aiSkillRunResultSchema = z.object({
  skill_id: z.string(),
  conversation_id: z.string(),
  assistant_message: z.object({
    id: z.string(),
    role: z.string(),
    content: z.string(),
    provider: z.string().default(""),
    model: z.string().default(""),
    warnings: z.array(z.string()).default([]),
    created_at: z.string().optional(),
  }),
  provider: z.string().default(""),
  model: z.string().default(""),
});

export type AIToolResult = z.infer<typeof aiToolResultSchema>;
export type AISkillRunResult = z.infer<typeof aiSkillRunResultSchema>;

/** 直接执行一个 AI 工具，供题目列表、试卷页等业务入口使用。 */
export function runAITool(name: string, args: Record<string, unknown> = {}) {
  return apiRequest(`/api/v1/ai/tools/${encodeURIComponent(name)}`, aiToolResultSchema, {
    method: "POST",
    body: JSON.stringify(args),
  });
}

/** 一键执行内置技能，返回新建会话与助手消息。 */
export function runAISkill(skillID: string, input: { content?: string; model?: string } = {}) {
  return apiRequest(`/api/v1/ai/skills/${encodeURIComponent(skillID)}/run`, aiSkillRunResultSchema, {
    method: "POST",
    body: JSON.stringify(input),
  });
}
