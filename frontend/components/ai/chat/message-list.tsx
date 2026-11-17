"use client";

import { useEffect, useMemo, useRef } from "react";
import {
  ChatBubbleIcon,
  FileTextIcon,
  MagicWandIcon,
  LockClosedIcon,
  LoopIcon,
  UploadIcon,
} from "@radix-ui/react-icons";
import type { ChatMessage, LiveTurn, Skill } from "@/lib/ai-chat";
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
  liveTurn: LiveTurn | null;
  error: string;
  skills: Skill[];
  scope: "teacher" | "admin";
  onUseSkill: (skill: Skill) => void;
}

function liveMessages(turn: LiveTurn | null): ChatMessage[] {
  if (!turn) return [];
  const createdAt = new Date().toISOString();
  return [
    {
      id: "live-user",
      conversation_id: "",
      role: "user",
      content: turn.content,
      status: "succeeded",
      provider: "user",
      model: "",
      tool_calls: [],
      attachments: turn.attachments,
      created_at: createdAt,
    },
    {
      id: "live-assistant",
      conversation_id: "",
      role: "assistant",
      content: turn.assistantContent,
      status: turn.status,
      provider: "llm",
      model: "",
      tool_calls: turn.toolCalls,
      attachments: [],
      error: turn.error,
      created_at: createdAt,
    },
  ];
}

export function MessageList({
  messages, liveTurn, error, skills, scope, onUseSkill,
}: MessageListProps) {
  const endRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const live = useMemo(() => liveMessages(liveTurn), [liveTurn]);
  const isEmpty = messages.length === 0 && !liveTurn;

  useEffect(() => {
    const container = scrollRef.current;
    if (!container) return;
    const nearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 240;
    if (nearBottom || messages.length <= 1) {
      endRef.current?.scrollIntoView({ behavior: messages.length <= 1 ? "auto" : "smooth", block: "end" });
    }
  }, [messages.length, liveTurn]);

  return (
    <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-5 sm:px-6" role="log" aria-live="polite" aria-relevant="additions">
      {isEmpty ? (
        <div className="mx-auto flex max-w-2xl flex-col items-center py-10 text-center">
          <span className="flex size-12 items-center justify-center rounded-full border border-accent/25 bg-accent-soft text-accent-strong">
            <ChatBubbleIcon className="size-6" />
          </span>
          <h2 className="mt-4 text-title-20 font-medium text-neutral-10">今天要处理什么？</h2>
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
                  {skill.accepts_attachments ? <UploadIcon className="size-3 text-neutral-6" aria-label="支持附件" /> : null}
                </span>
                <span className="mt-1 block text-label-12 leading-5 text-neutral-7">{skill.description}</span>
              </button>
            ))}
          </div>
        </div>
      ) : null}

      <div className="mx-auto grid max-w-3xl gap-5">
        {messages.map((message) => <MessageBubble key={message.id} message={message} scope={scope} />)}
        {live.map((message) => <MessageBubble key={message.id} message={message} scope={scope} />)}
        {error ? (
          <p role="alert" className={cn("rounded-lg border border-error-soft-strong bg-error-soft px-3 py-2.5 text-copy-13 leading-6 text-error")}>{error}</p>
        ) : null}
        <div ref={endRef} />
      </div>
    </div>
  );
}
