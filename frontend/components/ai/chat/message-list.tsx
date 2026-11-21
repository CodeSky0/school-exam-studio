"use client";

import { useEffect, useMemo, useRef } from "react";
import {
  BarChartIcon,
  ChatBubbleIcon,
  EyeOpenIcon,
  FileTextIcon,
  LayersIcon,
  TableIcon,
  UploadIcon,
} from "@radix-ui/react-icons";
import type { ChatMessage, LiveTurn, Skill } from "@/lib/ai-chat";
import { cn } from "@/lib/utils";
import { MessageBubble } from "./message-bubble";
import { SkillIcon } from "./skill-icon";

interface SkillGroup {
  category: string;
  label: string;
  description: string;
  icon: React.ReactNode;
}

const SKILL_GROUPS: SkillGroup[] = [
  { category: "document", label: "文档与资料", description: "直接阅读文档、录题与云盘检索", icon: <FileTextIcon className="size-3.5" /> },
  { category: "question", label: "题库整理", description: "体检、补全、出题与变式", icon: <LayersIcon className="size-3.5" /> },
  { category: "paper", label: "试卷生成", description: "组卷、质检与双向细目表", icon: <TableIcon className="size-3.5" /> },
  { category: "analytics", label: "数据分析", description: "覆盖度、重复题与相似题", icon: <BarChartIcon className="size-3.5" /> },
  { category: "review", label: "产出复核", description: "AI 草稿逐条审核", icon: <EyeOpenIcon className="size-3.5" /> },
];

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
  const messages: ChatMessage[] = [];
  // 用户消息一旦由后端落库回传，就不再渲染本地实时气泡，防止同一条消息显示两次。
  if (!turn.userMessagePersisted) {
    messages.push({
      id: "live-user",
      conversation_id: "",
      role: "user",
      content: turn.content,
      status: "succeeded",
      provider: "user",
      model: "",
      tool_calls: [],
      timeline: [],
      attachments: turn.attachments,
      created_at: turn.startedAt,
    });
  }
  const timeline = turn.reasoning.trim()
    ? [...turn.timeline, { type: "thought" as const, content: turn.reasoning.trim(), created_at: createdAt }]
    : turn.timeline;
  messages.push({
    id: "live-assistant",
    conversation_id: "",
    role: "assistant",
    content: turn.assistantContent,
    status: turn.status,
    provider: "llm",
    model: "",
    tool_calls: turn.toolCalls,
    timeline,
    attachments: [],
    reasoning: turn.reasoning,
    error: turn.error,
    created_at: turn.startedAt,
  });
  return messages;
}

function EmptyWorkspace({ skills, onUseSkill }: { skills: Skill[]; onUseSkill: (skill: Skill) => void }) {
  const grouped = SKILL_GROUPS.map((group) => ({
    ...group,
    items: skills.filter((skill) => skill.category === group.category),
  })).filter((group) => group.items.length > 0);

  return (
    <div className="mx-auto flex max-w-3xl flex-col items-center py-8 text-center">
      <span className="motion-pop flex size-14 items-center justify-center rounded-2xl border border-accent/25 bg-accent-soft text-accent-strong shadow-sm">
        <ChatBubbleIcon className="size-7" />
      </span>
      <h2 className="mt-4 text-title-20 font-medium text-neutral-10">AI 题库工作台</h2>
      <p className="mt-1.5 max-w-md text-label-12 leading-5 text-neutral-7">
        用自然语言完成文档录题、题库体检、智能组卷与校内资料检索。选择下方能力开始，或直接在输入框描述任务。
      </p>
      <div className="mt-8 grid w-full gap-6 text-left">
        {grouped.map((group) => (
          <section key={group.category} aria-label={group.label}>
            <header className="flex items-center gap-2">
              <span className="flex size-6 items-center justify-center rounded-md border border-accent/20 bg-accent-soft/60 text-accent-strong">{group.icon}</span>
              <span className="text-label-12 font-medium text-neutral-9">{group.label}</span>
              <span className="text-caption-10 text-neutral-6">{group.description}</span>
              <span className="ml-auto rounded-full bg-neutral-2 px-1.5 py-0.5 font-mono text-caption-10 text-neutral-6">{group.items.length}</span>
            </header>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              {group.items.map((skill) => (
                <button
                  key={skill.id}
                  type="button"
                  onClick={() => onUseSkill(skill)}
                  className="group flex items-start gap-2.5 rounded-lg border border-border bg-neutral-1 p-3 text-left transition-all hover:-translate-y-0.5 hover:border-accent/40 hover:bg-accent-soft/25 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/35"
                >
                  <span className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-md border border-border bg-neutral-2 text-accent-strong transition-colors group-hover:border-accent/30 group-hover:bg-accent-soft">
                    <SkillIcon icon={skill.icon} className="size-3.5" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="flex items-center gap-1.5 text-copy-13 font-medium text-neutral-9">
                      {skill.title}
                      {skill.accepts_attachments ? <UploadIcon className="size-3 text-neutral-6" aria-label="支持附件" /> : null}
                    </span>
                    <span className="mt-0.5 block text-label-12 leading-5 text-neutral-7">{skill.description}</span>
                  </span>
                </button>
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
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
      // Token 流期间使用即时滚动，避免高频平滑动画堆积。
      const behavior = liveTurn?.status === "running" || messages.length <= 1 ? "auto" : "smooth";
      endRef.current?.scrollIntoView({ behavior, block: "end" });
    }
  }, [messages.length, liveTurn]);

  return (
    <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 py-5 sm:px-6" role="log" aria-live="polite" aria-relevant="additions">
      {isEmpty ? <EmptyWorkspace skills={skills} onUseSkill={onUseSkill} /> : null}

      <div className="mx-auto grid max-w-4xl gap-5">
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