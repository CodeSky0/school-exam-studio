"use client";

import { useState } from "react";
import {
  CheckCircledIcon,
  ChevronDownIcon,
  ClockIcon,
  CopyIcon,
  CrossCircledIcon,
  ExclamationTriangleIcon,
  MinusCircledIcon,
} from "@radix-ui/react-icons";
import { formatToolDuration, toolStatusLabel, type ToolCall } from "@/lib/ai-chat";
import { cn } from "@/lib/utils";

const statusStyles: Record<ToolCall["status"], { icon: React.ReactNode; tone: string }> = {
  running: { icon: <ClockIcon className="size-4 animate-pulse" />, tone: "border-info/25 bg-info-soft text-info" },
  succeeded: { icon: <CheckCircledIcon className="size-4" />, tone: "border-success/25 bg-success-soft text-success" },
  failed: { icon: <CrossCircledIcon className="size-4" />, tone: "border-error-soft-strong bg-error-soft text-error" },
  skipped: { icon: <MinusCircledIcon className="size-4" />, tone: "border-warning/30 bg-warning-soft text-warning" },
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

export function ToolCallTimeline({ calls, scope }: { calls: ToolCall[]; scope: "teacher" | "admin" }) {
  if (calls.length === 0) return null;
  return (
    <ol className="mt-3 grid gap-2" aria-label="工具调用记录">
      {calls.map((call, index) => (
        <ToolCallCard key={call.id} call={call} last={index === calls.length - 1} scope={scope} />
      ))}
    </ol>
  );
}

function ToolCallCard({ call, last, scope }: { call: ToolCall; last: boolean; scope: "teacher" | "admin" }) {
  const style = statusStyles[call.status] ?? statusStyles.skipped;
  const duration = formatToolDuration(call.started_at, call.finished_at);
  const questionIDs = toolQuestionIDs(call);
  const warnings = toolWarnings(call);
  const paperID = toolPaperID(call);
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
    <li className="grid grid-cols-[1.1rem_minmax(0,1fr)] gap-2.5">
      <div className="flex flex-col items-center pt-1">
        <span className={cn("flex size-6 items-center justify-center rounded-full border", style.tone)}>{style.icon}</span>
        {!last ? <span className="mt-1 w-px flex-1 bg-border" aria-hidden="true" /> : null}
      </div>
      <div className="min-w-0 pb-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-copy-13 font-medium text-neutral-9">{call.label}</span>
          <span className={cn("rounded-full border px-2 py-0.5 text-caption-10 font-medium", style.tone)}>{toolStatusLabel(call.status)}</span>
          {duration ? <span className="text-caption-10 text-neutral-6">{duration}</span> : null}
          <span className="font-mono text-caption-10 text-neutral-5">{call.name}</span>
        </div>
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
            <p className="mt-1 text-caption-10 text-neutral-6">
              可在{scope === "admin" ? "全校题库" : "教师题库"}中搜索这些 ID 查看草稿。
            </p>
          </div>
        ) : null}
        {paperID ? (
          <p className="mt-2 rounded-md border border-accent/25 bg-accent-soft px-2.5 py-2 text-label-12 text-accent-strong">
            已生成试卷草稿：<span className="font-mono">{paperID}</span>
          </p>
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