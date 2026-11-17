"use client";

import { useEffect, useRef, useState } from "react";
import { MagicWandIcon, PaperPlaneIcon, PlusIcon, StopIcon } from "@radix-ui/react-icons";
import {
  ATTACHMENT_ACCEPT,
  MAX_ATTACHMENTS,
  type AttachmentRef,
  type Skill,
} from "@/lib/ai-chat";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { AttachmentTray, type UploadTask } from "./attachment-tray";

interface ComposerProps {
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  onCancel: () => void;
  attachments: AttachmentRef[];
  uploads: UploadTask[];
  isSending: boolean;
  models: string[];
  model: string;
  onModelChange: (model: string) => void;
  onRemoveAttachment: (id: string) => void;
  onDismissUpload: (id: string) => void;
  onUploadFiles: (files: File[]) => void;
  skills: Skill[];
  onUseSkill: (skill: Skill) => void;
  readOnly?: boolean;
}

export function Composer({
  value, onChange, onSubmit, onCancel, attachments, uploads, isSending, models, model, onModelChange,
  onRemoveAttachment, onDismissUpload, onUploadFiles, skills, onUseSkill, readOnly = false,
}: ComposerProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const composingRef = useRef(false);
  const [dragging, setDragging] = useState(false);
  const uploading = uploads.some((item) => item.status === "uploading");
  const full = attachments.length >= MAX_ATTACHMENTS;
  const canSend = (value.trim().length > 0 || attachments.length > 0 || uploads.length > 0) && !isSending && !uploading && !readOnly;

  useEffect(() => {
    const element = textareaRef.current;
    if (!element) return;
    element.style.height = "auto";
    element.style.height = `${Math.min(element.scrollHeight, 168)}px`;
  }, [value]);

  function selectFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    onUploadFiles(Array.from(files));
  }

  function submit() {
    if (!canSend) return;
    onSubmit();
  }

  return (
    <div
      className={cn("relative border-t border-border bg-neutral-1", dragging && "ring-2 ring-inset ring-accent/40")}
      onDragOver={(event) => { event.preventDefault(); setDragging(true); }}
      onDragLeave={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragging(false);
      }}
      onDrop={(event) => {
        event.preventDefault();
        setDragging(false);
        selectFiles(event.dataTransfer.files);
      }}
    >
      {dragging ? (
        <div className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-t-lg bg-accent-soft/80 backdrop-blur-sm">
          <p className="text-copy-13 font-medium text-accent-strong">松开上传</p>
        </div>
      ) : null}
      {skills.length > 0 ? (
        <div className="flex gap-1.5 overflow-x-auto border-b border-border px-3 py-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          {skills.map((skill) => (
            <button
              key={skill.id}
              type="button"
              onClick={() => onUseSkill(skill)}
              className="inline-flex shrink-0 items-center gap-1 rounded-full border border-border bg-neutral-2/70 px-2.5 py-1 text-caption-10 text-neutral-7 transition-colors hover:border-accent/35 hover:text-accent-strong"
            >
              <MagicWandIcon className="size-3" />{skill.title}
            </button>
          ))}
        </div>
      ) : null}
      <AttachmentTray attachments={attachments} uploads={uploads} onRemove={onRemoveAttachment} onDismissUpload={onDismissUpload} />
      {readOnly ? (
        <p className="border-b border-border bg-neutral-2/60 px-3 py-1.5 text-caption-10 text-neutral-7">只读对话</p>
      ) : null}
      <div className="px-3 py-2.5">
        <Textarea
          ref={textareaRef}
          value={value}
          rows={1}
          maxLength={8000}
          placeholder={readOnly ? "只读对话" : isSending ? "正在执行…" : "描述任务，或粘贴、拖入图片与文件…"}
          aria-label="消息内容"
          disabled={isSending || readOnly}
          onChange={(event) => onChange(event.target.value)}
          onCompositionStart={() => { composingRef.current = true; }}
          onCompositionEnd={() => { composingRef.current = false; }}
          onPaste={(event) => {
            const files = Array.from(event.clipboardData.files ?? []);
            if (files.length > 0) {
              event.preventDefault();
              onUploadFiles(files);
            }
          }}
          onKeyDown={(event) => {
            if (event.key !== "Enter" || event.shiftKey || composingRef.current) return;
            event.preventDefault();
            submit();
          }}
          className="min-h-11 max-h-[168px] resize-none border-0 bg-transparent px-1 py-1.5 text-copy-14 leading-6 shadow-none focus-visible:ring-0"
        />
        <div className="mt-1 flex flex-wrap items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-1.5">
            <input
              ref={fileInputRef}
              type="file"
              accept={ATTACHMENT_ACCEPT}
              multiple
              className="hidden"
              aria-label="上传附件"
              onChange={(event) => {
                selectFiles(event.target.files);
                event.target.value = "";
              }}
            />
            <Button type="button" size="icon" variant="ghost" aria-label="添加图片或文件" title="添加图片或文件" disabled={full || isSending || readOnly} onClick={() => fileInputRef.current?.click()}>
              <PlusIcon />
            </Button>
            {full ? <span className="text-caption-10 text-neutral-6">最多 {MAX_ATTACHMENTS} 个附件</span> : null}
          </div>
          <div className="flex items-center gap-2">
            {value.length > 6000 ? <span className={cn("text-caption-10", value.length > 7600 ? "text-error" : "text-warning")}>{value.length}/8000</span> : null}
            {models.length > 0 ? (
              <label className="flex items-center gap-1.5 text-caption-10 text-neutral-6">
                <span className="sr-only">选择模型</span>
                <select
                  value={model}
                  onChange={(event) => onModelChange(event.target.value)}
                  disabled={isSending}
                  className="h-7 max-w-44 rounded border border-border bg-neutral-2 px-1.5 font-mono text-caption-10 text-neutral-7 outline-none focus-visible:ring-2 focus-visible:ring-accent/35"
                >
                  {models.map((item) => <option key={item} value={item}>{item}</option>)}
                </select>
              </label>
            ) : null}
            {isSending ? (
              <Button type="button" size="sm" variant="outline" onClick={onCancel}>
                <StopIcon />
                停止
              </Button>
            ) : (
              <Button type="button" size="sm" onClick={submit} disabled={!canSend}>
                <PaperPlaneIcon />
                发送
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}