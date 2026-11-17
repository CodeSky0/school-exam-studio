"use client";

import { useEffect, useMemo, useRef } from "react";
import {
  ChatBubbleIcon,
  FileTextIcon,
  MagicWandIcon,
  LockClosedIcon,
  LoopIcon,
} from "@radix-ui/react-icons";
import type { AttachmentRef, ChatMessage, Skill } from "@/lib/ai-chat";
import { cn } from "@/lib/utils";
import { MessageBubble } from "./message-bubble";

function skillIcon(icon: string) {
  switch (icon) {
    case "file": return <FileTextIcon className="size-4" />;
    case "split": return <MagicWandIcon className="size-4" />;
    case "shield": return <LockClosedIcon className="size-4" />;
    case "repeat": return <LoopIcon className="size-4" />;
    default: return <MagicWandIcon className="size-4" />;
  }
}

interface MessageListProps {
  messages: ChatMessage[];
  pendingContent: string;
  pendingAttachments: AttachmentRef[];
  isSending: boolean;
  error: string;
  skills: Skill[];
  scope: "teacher" | "admin";
  onUseSkill: (skill: Skill) => void;
}

export function MessageList({
  messages, pendingContent, pendingAttachments, isSending, error, skills, scope, onUseSkill,
}: MessageListProps) {
  const endRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = scrollRef.current;
    if (!container) return;
    const nearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 240;
    if (nearBottom || messages.length <= 1) {
      endRef.current?.scrollIntoView({ behavior: messages.length <= 1 ? "auto" : "smooth", block: "end" });
    }
  }, [messages.length, isSending, pendingContent]);

  const pendingMessage = useMemo<ChatMessage | null>(() => {
    if (!isSending || (!pendingContent && pendingAttachments.length === 0)) return null;
    return {
      id: "pending-user",
      conversation_id: "",
      role: "user",
      content: pendingContent,
      status: "succeeded",
      provider: "user",
      model: "",
      tool_calls: [],
      attachments: pendingAttachments,
      created_at: new Date().toISOString(),
    };
  }, [isSending, pendingContent, pendingAttachments]);

  return (
    <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-5 sm:px-6" role="log" aria-live="polite" aria-relevant="additions">
      {messages.length === 0 && !isSending ? (
        <div className="mx-auto flex max-w-2xl flex-col items-center py-8 text-center">
          <span className="flex size-12 items-center justify-center rounded-full border border-accent/25 bg-accent-soft text-accent-strong">
            <ChatBubbleIcon className="size-6" />
          </span>
          <h2 className="mt-4 text-title-20 font-medium text-neutral-10">把题库工作交给 AI Agent</h2>
          <p className="mt-2 max-w-lg text-copy-13 leading-6 text-neutral-7">
            描述目标，或上传试卷、课本截图与 Word 文档。Agent 会规划步骤、调用题库工具，并把每一次执行过程展示给你。
          </p>
          <div className="mt-6 grid w-full gap-2 sm:grid-cols-2">
            {skills.map((skill) => (
              <button
                key={skill.id}
                type="button"
                onClick={() => onUseSkill(skill)}
                className="group rounded-lg border border-border bg-neutral-1 p-3 text-left transition-all hover:border-accent/40 hover:bg-accent-soft/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/35"
              >
                <span className="flex items-center gap-2 text-copy-13 font-medium text-neutral-9">
                  <span className="text-accent-strong">{skillIcon(skill.icon)}</span>
                  {skill.title}
                  {skill.accepts_attachments ? <span className="rounded bg-accent-soft px-1.5 py-0.5 text-caption-10 text-accent-strong">附件</span> : null}
                </span>
                <span className="mt-1 block text-label-12 leading-5 text-neutral-7">{skill.description}</span>
              </button>
            ))}
          </div>
          <p className="mt-5 text-caption-10 text-neutral-6">自动修改只作用于草稿，已发布题目只读，所有写入都会记录审计。</p>
        </div>
      ) : null}

      <div className="mx-auto grid max-w-3xl gap-5">
        {messages.map((message) => <MessageBubble key={message.id} message={message} scope={scope} />)}
        {pendingMessage ? <MessageBubble message={pendingMessage} scope={scope} /> : null}
        {isSending ? (
          <div className="flex items-center gap-3" aria-label="AI 正在处理">
            <span className="flex size-8 items-center justify-center rounded-md border border-accent/25 bg-accent-soft text-caption-10 font-semibold text-accent-strong">AI</span>
            <span className="inline-flex items-center gap-2 rounded-lg border border-border bg-neutral-1 px-3 py-2 text-label-12 text-neutral-7">
              <span className="flex gap-1" aria-hidden="true">
                <span className="size-1.5 animate-bounce rounded-full bg-accent [animation-delay:-200ms]" />
                <span className="size-1.5 animate-bounce rounded-full bg-accent [animation-delay:-100ms]" />
                <span className="size-1.5 animate-bounce rounded-full bg-accent" />
              </span>
              正在规划并调用工具…
            </span>
          </div>
        ) : null}
        {error ? (
          <p role="alert" className={cn("rounded-lg border border-error-soft-strong bg-error-soft px-3 py-2.5 text-copy-13 leading-6 text-error")}>{error}</p>
        ) : null}
        <div ref={endRef} />
      </div>
    </div>
  );
}