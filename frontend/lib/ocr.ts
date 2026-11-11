import { apiRequest } from "./api";
import { ocrResponseSchema } from "./question-schemas";

export const OCR_MAX_FILE_SIZE = 5 * 1024 * 1024;
export const OCR_ACCEPTED_TYPES = ["image/jpeg", "image/png", "image/webp"] as const;

export function validateOcrFile(file: File): string | null {
  if (!OCR_ACCEPTED_TYPES.includes(file.type as (typeof OCR_ACCEPTED_TYPES)[number])) {
    return "仅支持 JPEG、PNG 或 WebP 图片";
  }
  if (file.size === 0) return "图片内容为空，请重新选择";
  if (file.size > OCR_MAX_FILE_SIZE) return "图片不能超过 5 MB";
  return null;
}

export function recognizeQuestionImage(file: File) {
  const body = new FormData();
  body.append("image", file, file.name);
  return apiRequest("/api/v1/ocr/recognize", ocrResponseSchema, { method: "POST", body });
}
