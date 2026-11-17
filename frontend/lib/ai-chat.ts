import { z } from "zod";
import { apiDownload, apiRequest } from "./api";

export const attachmentRefSchema = z.object({
  id: z.string(),
  name: z.string(),
  mime: z.string(),
  kind: z.enum(["image", "pdf", "docx", "text"]),
  status: z.string(),
  text_preview: z.string().default(""),
  size_bytes: z.number().int().default(0),
  text_chars: z.number().int().default(0),
  error: z.string().optional(),
});

export const toolCallSchema = z.object({
  id: z.string(),
  name: z.string(),
  label: z.string(),
  status: z.enum(["running", "succeeded", "failed", "skipped"]),
  summary: z.string().default(""),
  error: z.string().optional(),
  arguments: z.record(z.string(), z.unknown()).default({}),
  result: z.record(z.string(), z.unknown()).optional(),
  started_at: z.string().optional(),
  finished_at: z.string().optional(),
});

export const chatMessageSchema = z.object({
  id: z.string(),
  conversation_id: z.string(),
  role: z.enum(["user", "assistant", "tool", "system"]),
  content: z.string().default(""),
  status: z.string().default("succeeded"),
  provider: z.string().default("local"),
  model: z.string().default(""),
  tool_calls: z.array(toolCallSchema).default([]),
  attachments: z.array(attachmentRefSchema).default([]),
  run_id: z.string().optional(),
  error: z.string().optional(),
  created_at: z.string(),
});

export const conversationSchema = z.object({
  id: z.string(),
  title: z.string(),
  status: z.string(),
  pinned: z.boolean().default(false),
  creator_id: z.string(),
  message_count: z.number().int().default(0),
  last_message_at: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});

export const sendMessageResultSchema = z.object({
  conversation: conversationSchema,
  user_message: chatMessageSchema,
  assistant_message: chatMessageSchema,
  attachments: z.array(attachmentRefSchema).default([]),
});

export const toolDescriptorSchema = z.object({
  name: z.string(),
  label: z.string(),
  description: z.string(),
  parameters: z.unknown(),
  writes: z.boolean().default(false),
});

export const skillSchema = z.object({
  id: z.string(),
  title: z.string(),
  description: z.string(),
  icon: z.string(),
  prompt: z.string(),
  required_tools: z.array(z.string()).default([]),
  accepts_attachments: z.boolean().default(false),
});

export type AttachmentRef = z.infer<typeof attachmentRefSchema>;
export type ToolCall = z.infer<typeof toolCallSchema>;
export type ChatMessage = z.infer<typeof chatMessageSchema>;
export type Conversation = z.infer<typeof conversationSchema>;
export type SendMessageResult = z.infer<typeof sendMessageResultSchema>;
export type ToolDescriptor = z.infer<typeof toolDescriptorSchema>;
export type Skill = z.infer<typeof skillSchema>;

export const ATTACHMENT_ACCEPT = "image/png,image/jpeg,image/webp,image/gif,.pdf,.docx,.txt,.md,.csv";
export const MAX_ATTACHMENTS = 5;

const conversationListSchema = z.object({ items: z.array(conversationSchema) });
const messageListSchema = z.object({ items: z.array(chatMessageSchema) });
const conversationDetailSchema = z.object({ conversation: conversationSchema, messages: z.array(chatMessageSchema) });
const toolListSchema = z.object({ items: z.array(toolDescriptorSchema) });
const skillListSchema = z.object({ items: z.array(skillSchema) });
const emptySchema = z.undefined();

export function listConversations(limit = 30) {
  return apiRequest(`/api/v1/ai/conversations?limit=${limit}`, conversationListSchema);
}

export function createConversation(title = "") {
  return apiRequest("/api/v1/ai/conversations", conversationSchema, {
    method: "POST",
    body: JSON.stringify({ title }),
  });
}

export function updateConversation(id: string, input: { title?: string; pinned?: boolean }) {
  return apiRequest(`/api/v1/ai/conversations/${encodeURIComponent(id)}`, conversationSchema, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteConversation(id: string) {
  return apiRequest(`/api/v1/ai/conversations/${encodeURIComponent(id)}`, emptySchema, {
    method: "DELETE",
  });
}

export function getConversation(id: string) {
  return apiRequest(`/api/v1/ai/conversations/${encodeURIComponent(id)}`, conversationDetailSchema);
}

export function listMessages(id: string, limit = 100) {
  return apiRequest(`/api/v1/ai/conversations/${encodeURIComponent(id)}/messages?limit=${limit}`, messageListSchema);
}

export function sendMessage(id: string, input: { content: string; attachment_ids?: string[]; model?: string }) {
  return apiRequest(`/api/v1/ai/conversations/${encodeURIComponent(id)}/messages`, sendMessageResultSchema, {
    method: "POST",
    body: JSON.stringify({ content: input.content, attachment_ids: input.attachment_ids ?? [], model: input.model ?? "" }),
  });
}

export function uploadAttachment(file: File, conversationID?: string) {
  const form = new FormData();
  form.append("file", file);
  if (conversationID) form.append("conversation_id", conversationID);
  return apiRequest("/api/v1/ai/attachments", attachmentRefSchema, { method: "POST", body: form });
}

export function fetchAttachmentBlob(id: string) {
  return apiDownload(`/api/v1/ai/attachments/${encodeURIComponent(id)}/content`);
}

export function listTools() {
  return apiRequest("/api/v1/ai/tools", toolListSchema);
}

export function listSkills() {
  return apiRequest("/api/v1/ai/skills", skillListSchema);
}

export const aiChatQueryKeys = {
  conversations: ["ai", "chat", "conversations"] as const,
  messages: (id: string) => ["ai", "chat", "messages", id] as const,
  tools: ["ai", "chat", "tools"] as const,
  skills: ["ai", "chat", "skills"] as const,
};

export function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

export function formatChatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const now = new Date();
  const sameDay = date.toDateString() === now.toDateString();
  return new Intl.DateTimeFormat("zh-CN", sameDay
    ? { hour: "2-digit", minute: "2-digit", hour12: false }
    : { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }).format(date);
}

export function formatToolDuration(startedAt?: string, finishedAt?: string) {
  if (!startedAt || !finishedAt) return "";
  const start = new Date(startedAt).getTime();
  const end = new Date(finishedAt).getTime();
  if (Number.isNaN(start) || Number.isNaN(end) || end < start) return "";
  const milliseconds = end - start;
  if (milliseconds < 1000) return `${milliseconds} ms`;
  return `${(milliseconds / 1000).toFixed(1)} s`;
}

export function toolStatusLabel(status: ToolCall["status"]) {
  switch (status) {
    case "running": return "执行中";
    case "succeeded": return "已完成";
    case "failed": return "失败";
    case "skipped": return "已跳过";
    default: return status;
  }
}

export function isImageAttachment(attachment: AttachmentRef) {
  return attachment.kind === "image";
}

export function attachmentKindLabel(attachment: AttachmentRef) {
  switch (attachment.kind) {
    case "image": return "图片";
    case "pdf": return "PDF";
    case "docx": return "Word";
    case "text": return "文本";
    default: return "文件";
  }
}

export function messageQuestionIDs(message: ChatMessage) {
  const ids = new Set<string>();
  for (const call of message.tool_calls) {
    const result = call.result;
    if (!result) continue;
    const data = result["data"];
    if (data && typeof data === "object" && "question_ids" in data) {
      const values = (data as Record<string, unknown>)["question_ids"];
      if (Array.isArray(values)) {
        for (const value of values) {
          if (typeof value === "string") ids.add(value);
        }
      }
    }
    const values = result["question_ids"];
    if (Array.isArray(values)) {
      for (const value of values) {
        if (typeof value === "string") ids.add(value);
      }
    }
  }
  return [...ids];
}