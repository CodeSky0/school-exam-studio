"use client";

import { useState, type ReactNode } from "react";
import Link from "next/link";
import {
  CheckCircledIcon,
  ChevronDownIcon,
  ClockIcon,
  CopyIcon,
  CrossCircledIcon,
  ExclamationTriangleIcon,
  MinusCircledIcon,
} from "@radix-ui/react-icons";
import {
  formatToolDuration,
  processSummary,
  toolStatusLabel,
  type ChatMessage,
  type TimelineEntry,
  type ToolCall,
} from "@/lib/ai-chat";
import { cn } from "@/lib/utils";

const statusStyles: Record<ToolCall["status"], { icon: ReactNode; dot: string; tone: string }> = {
  running: {
    icon: <ClockIcon className="size-3.5" />,
    dot: "border-info/45 bg-info-soft text-info",
    tone: "border-info/25 bg-info-soft text-info",
  },
  succeeded: {
    icon: <CheckCircledIcon className="size-3.5" />,
    dot: "border-success/45 bg-success-soft text-success",
    tone: "border-success/25 bg-success-soft text-success",
  },
  failed: {
    icon: <CrossCircledIcon className="size-3.5" />,
    dot: "border-error-soft-strong bg-error-soft text-error",
    tone: "border-error-soft-strong bg-error-soft text-error",
  },
  skipped: {
    icon: <MinusCircledIcon className="size-3.5" />,
    dot: "border-warning/40 bg-warning-soft text-warning",
    tone: "border-warning/30 bg-warning-soft text-warning",
  },
};

function stringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is string => typeof item === "string");
}

function toolQuestionIDs(call: ToolCall) {
  const ids = new Set<string>();
  stringArray(call.result?.["question_ids"]).forEach((id) => ids.add(id));
  const data = call.result?.["data"];
  if (data && typeof data === "object") {
    stringArray((data as Record<string, unknown>)["question_ids"]).forEach((id) => ids.add(id));
  }
  return [...ids];
}

function toolWarnings(call: ToolCall) {
  return stringArray(call.result?.["warnings"]);
}

function toolPaperID(call: ToolCall) {
  const data = call.result?.["data"];
  if (!data || typeof data !== "object") return "";
  const value = (data as Record<string, unknown>)["paper_id"];
  return typeof value === "string" ? value : "";
}

function prettyJSON(value: unknown) {
  try {
    const text = JSON.stringify(value ?? {}, null, 2);
    return text.length > 8000 ? `${text.slice(0, 8000)}\n…（已截断）` : text;
  } catch {
    return String(value);
  }
}

// argumentChips 把工具调用的关键参数提炼为最多 3 个可读标签，帮助用户理解每一步在做什么。
function argumentChips(call: ToolCall): { label: string; value: string }[] {
  const args = call.arguments ?? {};
  const chips: { label: string; value: string }[] = [];
  const push = (label: string, raw: unknown, shorten = false) => {
    if (typeof raw === "string" && raw.trim()) {
      const value = shorten && raw.length > 10 ? `${raw.slice(0, 8)}…` : raw.trim();
      chips.push({ label, value });
    } else if (typeof raw === "number") {
      chips.push({ label, value: String(raw) });
    }
  };
  push("学科", args["subject"]);
  push("年级", args["grade"]);
  push("关键词", args["keyword"]);
  push("知识点", args["knowledge_point"]);
  push("检索", args["query"]);
  push("题量", args["count"]);
  push("题目", args["question_id"], true);
  return chips.slice(0, 3);
}

export function timelineForMessage(message: ChatMessage): TimelineEntry[] {
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

function RunningDot() {
  return (
    <span className="relative flex size-4 shrink-0 items-center justify-center" aria-hidden="true">
      <span className="trace-pulse absolute inset-0 rounded-full bg-info/35" />
      <span className="relative size-1.5 rounded-full bg-info" />
    </span>
  );
}

function currentAction(message: ChatMessage, isRunning: boolean) {
  if (!isRunning) return "";
  const running = [...message.tool_calls].reverse().find((call) => call.status === "running");
  if (running) return `正在调用「${running.label}」`;
  return "正在思考…";
}

function TraceThought({ entry }: { entry: TimelineEntry }) {
  return (
    <li className="motion-fade rounded-r-md border-l-2 border-accent/25 bg-accent-soft/15 px-2.5 py-1.5">
      <span className="text-caption-10 font-medium uppercase tracking-[0.14em] text-neutral-6">思路</span>
      <p className="mt-0.5 max-h-36 overflow-y-auto whitespace-pre-wrap text-label-12 leading-5 text-neutral-7">{entry.content}</p>
    </li>
  );
}

function TraceStep({ call, number, last, scope }: { call: ToolCall; number: number; last: boolean; scope: "teacher" | "admin" }) {
  const style = statusStyles[call.status] ?? statusStyles.skipped;
  const duration = formatToolDuration(call.started_at, call.finished_at);
  const questionIDs = toolQuestionIDs(call);
  const warnings = toolWarnings(call);
  const paperID = toolPaperID(call);
  const chips = argumentChips(call);
  const [copied, setCopied] = useState(false);

  async function copyIDs() {
    try {
      await navigator.clipboard.writeText(questionIDs.join("\n"));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  }

  return (
    <li className="grid grid-cols-[1.5rem_minmax(0,1fr)] gap-2.5">
      <div className="flex flex-col items-center pt-0.5">
        <span className={cn("relative flex size-5 items-center justify-center rounded-full border", style.dot)}>
          {call.status === "running" ? <span className="trace-pulse absolute inset-0 rounded-full bg-info/40" aria-hidden="true" /> : null}
          <span className="relative">{style.icon}</span>
        </span>
        {!last ? <span className="mt-1 w-px flex-1 bg-border" aria-hidden="true" /> : null}
      </div>
      <div className="min-w-0 pb-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-caption-10 text-neutral-5">{String(number).padStart(2, "0")}</span>
          <span className="text-copy-13 font-medium text-neutral-9">{call.label}</span>
          <span className={cn("rounded-full border px-2 py-0.5 text-caption-10 font-medium", style.tone)}>{toolStatusLabel(call.status)}</span>
          {duration ? <span className="text-caption-10 text-neutral-6">{duration}</span> : null}
          <span className="font-mono text-caption-10 text-neutral-6">{call.name}</span>
        </div>
        {chips.length > 0 ? (
          <div className="mt-1 flex flex-wrap gap-1">
            {chips.map((chip) => (
              <span key={chip.label} className="inline-flex items-center gap-1 rounded bg-neutral-2 px-1.5 py-0.5 text-caption-10 text-neutral-7">
                <span className="text-neutral-6">{chip.label}</span>
                <span className="max-w-32 truncate font-medium text-neutral-9">{chip.value}</span>
              </span>
            ))}
          </div>
        ) : null}
        {call.summary ? <p className="mt-1 text-copy-13 leading-6 text-neutral-7">{call.summary}</p> : null}
        {call.error ? <p className="mt-1 text-label-12 leading-5 text-error">{call.error}</p> : null}
        {warnings.length > 0 ? (
          <ul className="mt-1.5 grid gap-1">
            {warnings.map((warning) => (
              <li key={warning} className="flex gap-1.5 text-label-12 leading-5 text-warning">
                <ExclamationTriangleIcon className="mt-0.5 size-3.5 shrink-0" />{warning}
              </li>
            ))}
          </ul>
        ) : null}
        {questionIDs.length > 0 ? (
          <div className="mt-2 rounded-md border border-border bg-neutral-2/60 px-2.5 py-2">
            <div className="flex items-center justify-between gap-2">
              <p className="text-caption-10 uppercase tracking-[0.14em] text-neutral-6">涉及题目 · {questionIDs.length}</p>
              <button type="button" onClick={() => { void copyIDs(); }} className="inline-flex items-center gap-1 text-caption-10 text-neutral-6 hover:text-accent-strong">
                <CopyIcon className="size-3" />{copied ? "已复制" : "复制ID"}
              </button>
            </div>
            <p className="mt-1 break-all font-mono text-caption-10 leading-5 text-neutral-7">{questionIDs.slice(0, 8).join(" · ")}{questionIDs.length > 8 ? ` 等 ${questionIDs.length} 道` : ""}</p>
            <Link
              href={scope === "admin" ? "/admin/questions" : "/teacher/questions"}
              className="mt-1.5 inline-block text-caption-10 font-medium text-accent-strong underline-offset-2 hover:underline"
            >
              打开题库
            </Link>
          </div>
        ) : null}
        {paperID ? (
          <div className="mt-2 flex flex-wrap items-center gap-2 rounded-md border border-accent/25 bg-accent-soft px-2.5 py-2 text-label-12 text-accent-strong">
            <span>试卷草稿 <span className="font-mono">{paperID}</span></span>
            {scope === "teacher" ? (
              <Link href={`/teacher/papers/${paperID}`} className="font-medium underline-offset-2 hover:underline">打开试卷</Link>
            ) : null}
          </div>
        ) : null}
        <details className="group mt-2 rounded-md border border-border bg-neutral-1">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-2.5 py-1.5 text-caption-10 text-neutral-6 hover:text-neutral-9">
            <span>查看参数与返回</span>
            <ChevronDownIcon className="size-3 transition-transform group-open:rotate-180" />
          </summary>
          <div className="border-t border-border px-2.5 py-2">
            <p className="text-caption-10 uppercase tracking-[0.14em] text-neutral-6">参数</p>
            <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap break-all rounded bg-neutral-2 p-2 font-mono text-caption-10 leading-5 text-neutral-7">{prettyJSON(call.arguments)}</pre>
            <p className="mt-3 text-caption-10 uppercase tracking-[0.14em] text-neutral-6">返回</p>
            <pre className="mt-1 max-h-56 overflow-auto whitespace-pre-wrap break-all rounded bg-neutral-2 p-2 font-mono text-caption-10 leading-5 text-neutral-7">{prettyJSON(call.result)}</pre>
          </div>
        </details>
      </div>
    </li>
  );
}

// AgentTrace 把一轮对话中的思考与工具调用渲染为可回溯的执行轨迹。
export function AgentTrace({ message, scope, isRunning }: { message: ChatMessage; scope: "teacher" | "admin"; isRunning: boolean }) {
  const timeline = timelineForMessage(message);
  if (timeline.length === 0) return null;
  const callByID = new Map(message.tool_calls.map((call) => [call.id, call]));
  const toolCount = message.tool_calls.length;
  const action = currentAction(message, isRunning);
  // 预先按时间线顺序编号，避免在渲染回调中修改外部变量。
  const toolNumberByID = new Map(
    timeline
      .filter((entry) => entry.type === "tool" && entry.tool_call_id)
      .map((entry, index) => [entry.tool_call_id as string, index + 1]),
  );

  return (
    <details open={isRunning} className="group mb-2 overflow-hidden rounded-xl border border-border bg-neutral-2/40">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2 text-label-12 text-neutral-8 transition-colors hover:bg-neutral-2/60 [&::-webkit-details-marker]:hidden">
        {isRunning ? <RunningDot /> : (
          <span className="motion-pop flex size-4 shrink-0 items-center justify-center rounded-full border border-success/45 bg-success-soft text-success" aria-hidden="true">
            <CheckCircledIcon className="size-3" />
          </span>
        )}
        <span className="min-w-0 truncate font-medium text-neutral-9">{processSummary(message, isRunning)}</span>
        {action ? <span className="shrink-0 rounded-full border border-info/25 bg-info-soft px-2 py-0.5 text-caption-10 text-info">{action}</span> : null}
        {toolCount > 0 ? (
          <span className="ml-auto hidden shrink-0 items-center gap-1 text-caption-10 text-neutral-6 sm:inline-flex">
            {toolCount} 步
          </span>
        ) : null}
        <ChevronDownIcon className="size-3 shrink-0 text-neutral-6 transition-transform group-open:rotate-180" />
      </summary>
      <ol className="grid max-h-96 gap-2 overflow-y-auto border-t border-border/70 px-3 py-2.5">
        {timeline.map((entry, index) => {
          if (entry.type === "thought") {
            return <TraceThought key={`thought-${index}`} entry={entry} />;
          }
          const call = entry.tool_call_id ? callByID.get(entry.tool_call_id) : undefined;
          if (!call) return null;
          return (
            <TraceStep
              key={call.id}
              call={call}
              number={toolNumberByID.get(call.id) ?? 0}
              last={index === timeline.length - 1}
              scope={scope}
            />
          );
        })}
      </ol>
    </details>
  );
}