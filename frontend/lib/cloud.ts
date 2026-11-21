import { z } from "zod";
import { apiDownload, apiRequest } from "./api";

export const cloudItemSchema = z.object({
  id: z.string(),
  parent_id: z.string().default(""),
  name: z.string(),
  is_folder: z.boolean().default(false),
  mime: z.string().default(""),
  kind: z.string().default(""),
  size_bytes: z.number().int().default(0),
  creator_id: z.string().default(""),
  index_status: z.string().default("pending"),
  index_error: z.string().default(""),
  index_provider: z.string().default(""),
  chunk_count: z.number().int().default(0),
  indexed_at: z.string().nullable().default(null),
  created_at: z.string(),
  updated_at: z.string(),
});

// 云盘语义检索结果：AI 与前端搜索共用同一接口。
export const cloudHitSchema = z.object({
  item_id: z.string(),
  name: z.string(),
  kind: z.string().default(""),
  chunk_index: z.number().int().default(0),
  score: z.number().default(0),
  excerpt: z.string().default(""),
});

export const cloudSearchSchema = z.object({
  query: z.string().default(""),
  provider: z.string().default(""),
  hits: z.array(cloudHitSchema).default([]),
  chunks_scanned: z.number().int().default(0),
  index_available: z.boolean().default(false),
});

export const cloudIndexStatsSchema = z.object({
  counts: z.record(z.string(), z.number()).default({}),
});

export const cloudBreadcrumbSchema = z.object({
  id: z.string(),
  name: z.string(),
});

export const cloudListSchema = z.object({
  parent_id: z.string().default(""),
  breadcrumbs: z.array(cloudBreadcrumbSchema).default([]),
  items: z.array(cloudItemSchema).default([]),
});

export type CloudItem = z.infer<typeof cloudItemSchema>;
export type CloudBreadcrumb = z.infer<typeof cloudBreadcrumbSchema>;
export type CloudList = z.infer<typeof cloudListSchema>;
export type CloudHit = z.infer<typeof cloudHitSchema>;
export type CloudSearch = z.infer<typeof cloudSearchSchema>;

export const CLOUD_ACCEPT = ".pdf,.doc,.docx,.ppt,.pptx,.xls,.xlsx,.txt,.md,.csv,image/png,image/jpeg,image/webp,image/gif";
export const CLOUD_MAX_BYTES = 50 * 1024 * 1024;

// AI 工作台只解析这些格式；云盘中的 PPT/Excel 等仍可正常下载与归档。
const AI_COMPATIBLE_EXTENSIONS = [".pdf", ".docx", ".txt", ".md", ".csv", ".png", ".jpg", ".jpeg", ".webp", ".gif"];

export function listCloudItems(parentID = "") {
  const query = parentID ? `?parent_id=${encodeURIComponent(parentID)}` : "";
  return apiRequest(`/api/v1/cloud/items${query}`, cloudListSchema);
}

export function createCloudFolder(name: string, parentID = "") {
  return apiRequest("/api/v1/cloud/folders", cloudItemSchema, {
    method: "POST",
    body: JSON.stringify({ name, parent_id: parentID }),
  });
}

export function uploadCloudFile(file: File, parentID = "") {
  const form = new FormData();
  form.append("file", file);
  if (parentID) form.append("parent_id", parentID);
  return apiRequest("/api/v1/cloud/files", cloudItemSchema, { method: "POST", body: form });
}

export function deleteCloudItem(id: string) {
  return apiRequest(`/api/v1/cloud/items/${encodeURIComponent(id)}`, z.object({ deleted: z.number().int() }), {
    method: "DELETE",
  });
}

export const cloudBatchDeleteSchema = z.object({ deleted: z.number().int() });

// batchDeleteCloudItems 批量删除云盘条目；后端会跳过不存在的条目并返回实际删除数量。
export function batchDeleteCloudItems(ids: string[]) {
  return apiRequest("/api/v1/cloud/items/batch-delete", cloudBatchDeleteSchema, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

// searchCloudFiles 调用云盘语义检索，返回已向量化资料的原文片段。
export function searchCloudFiles(query: string, limit = 8) {
  const params = new URLSearchParams({ q: query, limit: String(limit) });
  return apiRequest(`/api/v1/cloud/search?${params.toString()}`, cloudSearchSchema);
}

export function reindexCloudItem(id: string) {
  return apiRequest(`/api/v1/cloud/items/${encodeURIComponent(id)}/reindex`, cloudItemSchema, { method: "POST" });
}

export function reindexAllCloudFiles() {
  return apiRequest("/api/v1/cloud/reindex", z.object({ queued: z.number().int() }), { method: "POST" });
}

export function fetchCloudIndexStats() {
  return apiRequest("/api/v1/cloud/index/stats", cloudIndexStatsSchema);
}

export function fetchCloudFileBlob(id: string) {
  return apiDownload(`/api/v1/cloud/items/${encodeURIComponent(id)}/download`);
}

export async function downloadCloudFile(id: string, name: string) {
  const blob = await fetchCloudFileBlob(id);
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectURL;
  anchor.download = name;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(objectURL);
}

export const cloudQueryKeys = {
  list: (parentID: string) => ["cloud", "items", parentID] as const,
};

export function isFolder(item: CloudItem) {
  return item.is_folder;
}

export function cloudItemExtension(item: CloudItem) {
  const dot = item.name.lastIndexOf(".");
  return dot >= 0 ? item.name.slice(dot).toLowerCase() : "";
}

export function supportsAIConversation(item: CloudItem) {
  if (item.is_folder) return false;
  return AI_COMPATIBLE_EXTENSIONS.includes(cloudItemExtension(item));
}

// aiCloudFileIssue 返回云盘文件不能被加入 AI 对话的原因；空字符串表示可选。
export function aiCloudFileIssue(item: CloudItem) {
  if (item.is_folder) return "";
  if (!supportsAIConversation(item)) return "AI 暂不支持此格式";
  if (item.kind === "image" && item.size_bytes > 10 * 1024 * 1024) return "图片超过 10 MiB";
  if (item.kind !== "image" && item.size_bytes > 20 * 1024 * 1024) return "文档超过 20 MiB";
  return "";
}

export function cloudKindLabel(item: CloudItem) {
  if (item.is_folder) return "文件夹";
  switch (item.kind) {
    case "pdf": return "PDF";
    case "doc":
    case "docx": return "Word";
    case "ppt":
    case "pptx": return "PPT";
    case "xls":
    case "xlsx": return "Excel";
    case "image": return "图片";
    case "text": return "文本";
    default: return "文件";
  }
}

// cloudIndexLabel 返回云盘文件的向量化状态文案；文件夹不展示。
export function cloudIndexLabel(item: CloudItem) {
  if (item.is_folder) return "";
  switch (item.index_status) {
    case "ready": return `已向量化 · ${item.chunk_count} 段`;
    case "pending":
    case "processing": return "向量化中…";
    case "unsupported": return "不支持向量化";
    case "failed": return "向量化失败";
    default: return "";
  }
}

export function cloudIndexTone(item: CloudItem) {
  if (item.index_status === "ready") return "text-success";
  if (item.index_status === "failed") return "text-error";
  if (item.index_status === "pending" || item.index_status === "processing") return "text-accent-strong";
  return "text-neutral-6";
}

export function cloudIndexHint(item: CloudItem) {
  if (item.is_folder || item.index_status === "ready" || !item.index_error) return "";
  return item.index_error;
}

export function formatCloudScore(score: number) {
  return `${Math.round(score * 100)}%`;
}
