"use client";

import { Cross2Icon, ExclamationTriangleIcon, FileTextIcon, ImageIcon } from "@radix-ui/react-icons";
import { attachmentKindLabel, formatBytes, isImageAttachment, type AttachmentRef } from "@/lib/ai-chat";
import { AttachmentImage } from "./attachment-image";

export interface UploadTask {
  id: string;
  name: string;
  status: "uploading" | "failed";
  error?: string;
}

interface AttachmentTrayProps {
  attachments: AttachmentRef[];
  uploads: UploadTask[];
  onRemove: (id: string) => void;
  onDismissUpload: (id: string) => void;
}

export function AttachmentTray({ attachments, uploads, onRemove, onDismissUpload }: AttachmentTrayProps) {
  if (attachments.length === 0 && uploads.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-border bg-neutral-2/50 px-3 py-2.5" aria-label="待发送附件">
      {attachments.map((attachment) => (
        <span key={attachment.id} className="group relative flex max-w-full items-center gap-2 rounded-md border border-border bg-neutral-1 py-1 pl-1 pr-7">
          {isImageAttachment(attachment) ? (
            <AttachmentImage attachment={attachment} compact className="size-9 rounded-sm" />
          ) : (
            <span className="flex size-9 items-center justify-center rounded-sm bg-neutral-2 text-neutral-7">
              {attachment.kind === "image" ? <ImageIcon className="size-4" /> : <FileTextIcon className="size-4" />}
            </span>
          )}
          <span className="min-w-0">
            <span className="block max-w-40 truncate text-label-12 font-medium text-neutral-9">{attachment.name}</span>
            <span className="block text-caption-10 text-neutral-6">
              {attachmentKindLabel(attachment)} · {formatBytes(attachment.size_bytes)}
              {attachment.status === "failed" ? " · 解析失败" : attachment.text_chars > 0 ? ` · ${attachment.text_chars} 字` : ""}
            </span>
          </span>
          <button
            type="button"
            aria-label={`移除附件 ${attachment.name}`}
            onClick={() => onRemove(attachment.id)}
            className="absolute right-1 top-1 rounded p-1 text-neutral-6 transition-colors hover:bg-neutral-3 hover:text-error focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/35"
          >
            <Cross2Icon className="size-3" />
          </button>
        </span>
      ))}
      {uploads.map((task) => (
        <span key={task.id} className="flex max-w-full items-center gap-2 rounded-md border border-dashed border-border-strong bg-neutral-1 px-2.5 py-2 text-label-12">
          {task.status === "uploading" ? (
            <span className="size-3.5 animate-spin rounded-full border-2 border-accent/30 border-t-accent" aria-hidden="true" />
          ) : (
            <ExclamationTriangleIcon className="size-4 text-error" aria-hidden="true" />
          )}
          <span className="min-w-0">
            <span className="block max-w-40 truncate text-neutral-8">{task.name}</span>
            <span className={task.status === "failed" ? "block text-caption-10 text-error" : "block text-caption-10 text-neutral-6"}>
              {task.status === "uploading" ? "正在解析…" : task.error ?? "上传失败"}
            </span>
          </span>
          {task.status === "failed" ? (
            <button type="button" aria-label={`移除失败的附件 ${task.name}`} onClick={() => onDismissUpload(task.id)} className="rounded p-1 text-neutral-6 hover:text-error">
              <Cross2Icon className="size-3" />
            </button>
          ) : null}
        </span>
      ))}
    </div>
  );
}