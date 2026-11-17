"use client";

import { memo } from "react";
import { FileTextIcon, ImageIcon } from "@radix-ui/react-icons";
import {
  attachmentKindLabel,
  formatBytes,
  formatChatTime,
  isImageAttachment,
  type ChatMessage,
} from "@/lib/ai-chat";
import { AttachmentImage } from "./attachment-image";
import { ToolCallTimeline } from "./tool-call-timeline";

function ThinkingLine() {
  return (
    <span className="inline-flex items-center gap-2 text-label-12 text-neutral-7" aria-label="AI 正在处理">
      <span className="flex gap-1" aria-hidden="true">
        <span className="size-1.5 animate-bounce rounded-full bg-accent [animation-delay:-200ms]" />
        <span className="size-1.5 animate-bounce rounded-full bg-accent [animation-delay:-100ms]" />
        <span className="size-1.5 animate-bounce rounded-full bg-accent" />
      </span>
      正在思考…
    </span>
  );
}

function renderInline(text: string, prefix: string) {
  const parts = text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g);
  return parts.filter(Boolean).map((part, index) => {
    const key = `${prefix}-${index}`;
    if (part.startsWith("**") && part.endsWith("**") && part.length > 4) {
      return <strong key={key} className="font-semibold text-neutral-10">{part.slice(2, -2)}</strong>;
    }
    if (part.startsWith("`") && part.endsWith("`") && part.length > 2) {
      return <code key={key} className="rounded bg-neutral-3 px-1 py-0.5 font-mono text-[0.92em] text-neutral-9">{part.slice(1, -1)}</code>;
    }
    return <span key={key}>{part}</span>;
  });
}

function MessageText({ content }: { content: string }) {
  const lines = content.split("\n");
  const blocks: React.ReactNode[] = [];
  let list: string[] = [];
  const flushList = (key: string) => {
    if (list.length === 0) return;
    blocks.push(
      <ul key={key} className="my-1 grid gap-1">
        {list.map((item, index) => (
          <li key={`${key}-${index}`} className="flex gap-2">
            <span className="mt-2.5 size-1 shrink-0 rounded-full bg-accent" aria-hidden="true" />
            <span>{renderInline(item, `${key}-${index}`)}</span>
          </li>
        ))}
      </ul>,
    );
    list = [];
  };
  lines.forEach((line, index) => {
    const trimmed = line.trim();
    if (trimmed.startsWith("- ") || trimmed.startsWith("• ")) {
      list.push(trimmed.slice(2));
      return;
    }
    flushList(`list-${index}`);
    if (trimmed === "") {
      blocks.push(<span key={`gap-${index}`} className="block h-2" aria-hidden="true" />);
      return;
    }
    if (trimmed.startsWith("> ")) {
      blocks.push(
        <p key={`quote-${index}`} className="my-1 border-l-2 border-warning/60 pl-2.5 text-label-12 leading-5 text-warning">
          {renderInline(trimmed.slice(2), `quote-${index}`)}
        </p>,
      );
      return;
    }
    blocks.push(<p key={`line-${index}`}>{renderInline(line, `line-${index}`)}</p>);
  });
  flushList("list-final");
  return <div className="grid gap-1 text-copy-13 leading-6 text-neutral-8">{blocks}</div>;
}

function MessageAttachments({ message }: { message: ChatMessage }) {
  if (message.attachments.length === 0) return null;
  const images = message.attachments.filter(isImageAttachment);
  const files = message.attachments.filter((item) => !isImageAttachment(item));
  return (
    <div className="mt-2 grid gap-2">
      {images.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {images.map((attachment) => (
            <AttachmentImage key={attachment.id} attachment={attachment} className="max-w-56" />
          ))}
        </div>
      ) : null}
      {files.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {files.map((attachment) => (
            <span key={attachment.id} className="inline-flex max-w-full items-center gap-2 rounded-md border border-border bg-neutral-1 px-2.5 py-1.5">
              {isImageAttachment(attachment) ? <ImageIcon className="size-4 text-neutral-6" /> : <FileTextIcon className="size-4 text-neutral-6" />}
              <span className="min-w-0">
                <span className="block max-w-52 truncate text-label-12 font-medium text-neutral-9">{attachment.name}</span>
                <span className="block text-caption-10 text-neutral-6">
                  {attachmentKindLabel(attachment)} · {formatBytes(attachment.size_bytes)}
                  {attachment.status === "failed" ? ` · ${attachment.error ?? "解析失败"}` : attachment.text_chars > 0 ? ` · 已解析 ${attachment.text_chars} 字` : ""}
                </span>
              </span>
            </span>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function MessageBubbleComponent({ message, scope }: { message: ChatMessage; scope: "teacher" | "admin" }) {
  const isUser = message.role === "user";
  if (isUser) {
    return (
      <article className="motion-rise flex justify-end gap-3">
        <div className="max-w-[85%]">
          <div className="rounded-xl rounded-tr-sm bg-neutral-2 px-4 py-3 ring-1 ring-border">
            {message.content ? <MessageText content={message.content} /> : null}
            <MessageAttachments message={message} />
          </div>
          <p className="mt-1 text-right text-caption-10 text-neutral-6">{formatChatTime(message.created_at)}</p>
        </div>
      </article>
    );
  }
  const isRunning = message.status === "running";
  return (
    <article className="motion-rise flex gap-3">
      <div className="flex size-8 shrink-0 items-center justify-center rounded-md border border-accent/25 bg-accent-soft text-caption-10 font-semibold text-accent-strong" aria-hidden="true">AI</div>
      <div className="min-w-0 flex-1">
        <div className="rounded-lg border border-border bg-neutral-1 px-4 py-3 shadow-sm">
          {message.reasoning ? (
            <details open={isRunning} className="mb-2 rounded-md border border-border bg-neutral-2/60 px-2.5 py-2">
              <summary className="cursor-pointer select-none text-caption-10 font-medium text-neutral-7">
                {isRunning ? "思考中…" : "思考过程"}
              </summary>
              <p className="mt-1.5 max-h-64 overflow-y-auto whitespace-pre-wrap text-label-12 leading-5 text-neutral-7">
                {message.reasoning}
              </p>
            </details>
          ) : null}
          {message.content ? (
            <>
              <MessageText content={message.content} />
              {isRunning ? <span aria-hidden="true" className="ml-0.5 inline-block h-3.5 w-1.5 animate-pulse rounded-sm bg-accent align-text-bottom" /> : null}
            </>
          ) : isRunning ? <ThinkingLine /> : null}
          <MessageAttachments message={message} />
          <ToolCallTimeline calls={message.tool_calls} scope={scope} />
          {message.status === "failed" ? (
            <p role="alert" className="mt-2 rounded-md border border-error-soft-strong bg-error-soft px-2.5 py-2 text-label-12 leading-5 text-error">
              {message.error ?? "本轮处理失败，请重试。"}
            </p>
          ) : null}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-2 text-caption-10 text-neutral-6">
          <span>{formatChatTime(message.created_at)}</span>
          {message.tool_calls.length > 0 ? <span>· {message.tool_calls.length} 个工具</span> : null}
        </div>
      </div>
    </article>
  );
}

export const MessageBubble = memo(MessageBubbleComponent);