"use client";

import { Fragment, memo } from "react";
import { ChevronDownIcon, FileTextIcon, ImageIcon } from "@radix-ui/react-icons";
import {
  attachmentKindLabel,
  formatBytes,
  formatChatTime,
  isImageAttachment,
  type ChatMessage,
  type TimelineEntry,
  type ToolCall,
} from "@/lib/ai-chat";
import { AttachmentImage } from "./attachment-image";
import { MessageMath } from "./message-math";
import { ToolCallCard } from "./tool-call-timeline";

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
  const parts = text.split(/(\*\*[^*]+\*\*|`[^`]+`|\\\([\s\S]+?\\\)|\$[^$\n]+?\$)/g);
  return parts.filter(Boolean).map((part, index) => {
    const key = `${prefix}-${index}`;
    if (part.startsWith("**") && part.endsWith("**") && part.length > 4) {
      return <strong key={key} className="font-semibold text-neutral-10">{part.slice(2, -2)}</strong>;
    }
    if (part.startsWith("`") && part.endsWith("`") && part.length > 2) {
      return <code key={key} className="rounded bg-neutral-3 px-1 py-0.5 font-mono text-[0.92em] text-neutral-9">{part.slice(1, -1)}</code>;
    }
    if (part.startsWith("\\(") && part.endsWith("\\)") && part.length > 4) {
      return <MessageMath key={key} latex={part.slice(2, -2).trim()} />;
    }
    if (part.startsWith("$") && part.endsWith("$") && part.length > 2) {
      return <MessageMath key={key} latex={part.slice(1, -1).trim()} />;
    }
    return <span key={key}>{part}</span>;
  });
}

const blockMathPattern = /\$\$([\s\S]+?)\$\$|\\\[([\s\S]+?)\\\]/g;

interface TextSegment {
  type: "text" | "block";
  value: string;
}

function splitBlockMath(content: string): TextSegment[] {
  const segments: TextSegment[] = [];
  let cursor = 0;
  let match: RegExpExecArray | null;
  blockMathPattern.lastIndex = 0;
  while ((match = blockMathPattern.exec(content)) !== null) {
    if (match.index > cursor) segments.push({ type: "text", value: content.slice(cursor, match.index) });
    const latex = (match[1] ?? match[2] ?? "").trim();
    segments.push(latex ? { type: "block", value: latex } : { type: "text", value: match[0] });
    cursor = match.index + match[0].length;
  }
  if (cursor < content.length) segments.push({ type: "text", value: content.slice(cursor) });
  return segments.length > 0 ? segments : [{ type: "text", value: content }];
}

function renderPlainText(content: string, prefix: string): React.ReactNode[] {
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
    flushList(`${prefix}-list-${index}`);
    if (trimmed === "") {
      blocks.push(<span key={`${prefix}-gap-${index}`} className="block h-2" aria-hidden="true" />);
      return;
    }
    if (trimmed.startsWith("> ")) {
      blocks.push(
        <p key={`${prefix}-quote-${index}`} className="my-1 border-l-2 border-warning/60 pl-2.5 text-label-12 leading-5 text-warning">
          {renderInline(trimmed.slice(2), `${prefix}-quote-${index}`)}
        </p>,
      );
      return;
    }
    blocks.push(<p key={`${prefix}-line-${index}`}>{renderInline(line, `${prefix}-line-${index}`)}</p>);
  });
  flushList(`${prefix}-list-final`);
  return blocks;
}

function MessageText({ content }: { content: string }) {
  const segments = splitBlockMath(content);
  return (
    <div className="grid gap-1 text-copy-13 leading-6 text-neutral-8">
      {segments.map((segment, index) => segment.type === "block" ? (
        <div key={`math-${index}`} className="my-1 overflow-x-auto">
          <MessageMath latex={segment.value} display />
        </div>
      ) : (
        <Fragment key={`text-${index}`}>{renderPlainText(segment.value, `seg-${index}`)}</Fragment>
      ))}
    </div>
  );
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

function timelineForMessage(message: ChatMessage): TimelineEntry[] {
  if (message.timeline.length > 0) return message.timeline;
  const items: TimelineEntry[] = [];
  if (message.reasoning?.trim()) {
    items.push({ type: "thought", content: message.reasoning.trim(), created_at: message.created_at });
  }
  for (const call of message.tool_calls) {
    items.push({ type: "tool", content: "", tool_call_id: call.id, created_at: call.started_at ?? message.created_at });
  }
  return items;
}

function processLabel(calls: ToolCall[], isRunning: boolean) {
  if (isRunning) {
    return calls.length > 0 ? `正在运行，已调用 ${calls.length} 次工具` : "正在思考";
  }
  if (calls.length === 0) return "思考过程";
  const duration = calls.reduce((total, call) => {
    if (!call.started_at || !call.finished_at) return total;
    const start = Date.parse(call.started_at);
    const end = Date.parse(call.finished_at);
    if (!Number.isFinite(start) || !Number.isFinite(end)) return total;
    return total + Math.max(0, end - start);
  }, 0);
  const seconds = Math.max(1, Math.round(duration / 1000));
  return `运行了 ${seconds} 秒，调用 ${calls.length} 次工具`;
}

function AssistantProcess({ message, scope, isRunning }: { message: ChatMessage; scope: "teacher" | "admin"; isRunning: boolean }) {
  const timeline = timelineForMessage(message);
  if (timeline.length === 0) return null;
  const callByID = new Map(message.tool_calls.map((call) => [call.id, call]));
  return (
    <details open={isRunning} className="group mb-2 rounded-md border border-border bg-neutral-2/60">
      <summary className="flex cursor-pointer list-none items-center gap-1.5 px-2.5 py-2 text-caption-10 font-medium text-neutral-7 [&::-webkit-details-marker]:hidden">
        <ChevronDownIcon className="size-3 shrink-0 transition-transform group-open:rotate-180" />
        <span>{processLabel(message.tool_calls, isRunning)}</span>
      </summary>
      <ol className="grid gap-2 border-t border-border px-2.5 py-2">
        {timeline.map((entry, index) => {
          if (entry.type === "thought") {
            return (
              <li key={`thought-${index}`} className="whitespace-pre-wrap border-l-2 border-accent/30 pl-2 text-label-12 leading-5 text-neutral-7">
                {entry.content}
              </li>
            );
          }
          const call = entry.tool_call_id ? callByID.get(entry.tool_call_id) : undefined;
          if (!call) return null;
          return (
            <li key={call.id}>
              <ToolCallCard call={call} last={index === timeline.length - 1} scope={scope} />
            </li>
          );
        })}
      </ol>
    </details>
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
          <AssistantProcess message={message} scope={scope} isRunning={isRunning} />
          {message.content ? (
            <>
              <MessageText content={message.content} />
              {isRunning ? <span aria-hidden="true" className="ml-0.5 inline-block h-3.5 w-1.5 animate-pulse rounded-sm bg-accent align-text-bottom" /> : null}
            </>
          ) : isRunning ? <ThinkingLine /> : null}
          <MessageAttachments message={message} />
          {message.status === "failed" ? (
            <p role="alert" className="mt-2 rounded-md border border-error-soft-strong bg-error-soft px-2.5 py-2 text-label-12 leading-5 text-error">
              {message.error ?? "本轮处理失败，请重试。"}
            </p>
          ) : null}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-2 text-caption-10 text-neutral-6">
          <span>{formatChatTime(message.created_at)}</span>
        </div>
      </div>
    </article>
  );
}

export const MessageBubble = memo(MessageBubbleComponent);