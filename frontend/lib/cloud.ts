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
  created_at: z.string(),
  updated_at: z.string(),
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
