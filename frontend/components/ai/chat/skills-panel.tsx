"use client";

import { useState } from "react";
import {
  ChevronDownIcon,
  FileTextIcon,
  LightningBoltIcon,
  LockClosedIcon,
  LoopIcon,
  MagicWandIcon,
  ReaderIcon,
  GearIcon,
} from "@radix-ui/react-icons";
import type { AIStatus } from "@/lib/ai";
import type { Skill, ToolDescriptor } from "@/lib/ai-chat";
import { cn } from "@/lib/utils";

function skillIcon(icon: string) {
  switch (icon) {
    case "file": return <FileTextIcon className="size-4" />;
    case "book": return <ReaderIcon className="size-4" />;
    case "split": return <MagicWandIcon className="size-4" />;
    case "shield": return <LockClosedIcon className="size-4" />;
    case "repeat": return <LoopIcon className="size-4" />;
    default: return <MagicWandIcon className="size-4" />;
  }
}

function prettyParameters(value: unknown) {
  try {
    const text = JSON.stringify(value ?? {}, null, 2);
    return text.length > 4000 ? `${text.slice(0, 4000)}\n…` : text;
  } catch {
    return "{}";
  }
}

export function SkillsPanel({ status, isStatusPending, statusError, skills, tools, onUseSkill }: {
  status?: AIStatus;
  isStatusPending: boolean;
  statusError: boolean;
  skills: Skill[];
  tools: ToolDescriptor[];
  onUseSkill: (skill: Skill) => void;
}) {
  const [tab, setTab] = useState<"skills" | "tools">("skills");
  const provider = status?.provider ?? "local";
  const providerLabel = provider === "llm" ? "大模型" : provider === "local_fallback" ? "本地降级" : "本地引擎";

  return (
    <div className="flex h-full min-h-0 flex-col bg-neutral-1">
      <div className="border-b border-border p-3">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className={cn("inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-caption-10 font-medium", provider === "llm" ? "border-accent/30 bg-accent-soft text-accent-strong" : "border-info/25 bg-info-soft text-info")}>
            <LightningBoltIcon className="size-3" />{isStatusPending ? "读取中" : providerLabel}
          </span>
          {status?.model ? <span className="max-w-full truncate rounded-full bg-neutral-2 px-2 py-0.5 font-mono text-caption-10 text-neutral-7">{status.model}</span> : null}
        </div>
        {status?.warning ? <p className="mt-2 text-caption-10 leading-4 text-warning">{status.warning}</p> : null}
        {statusError ? <p className="mt-2 text-caption-10 leading-4 text-error">无法读取引擎状态</p> : null}
      </div>

      <div className="grid grid-cols-2 gap-1 border-b border-border p-2" role="tablist" aria-label="技能与工具">
        <button
          type="button"
          role="tab"
          aria-selected={tab === "skills"}
          onClick={() => setTab("skills")}
          className={cn("rounded-md px-2 py-1.5 text-label-12 font-medium transition-colors", tab === "skills" ? "bg-accent-soft text-accent-strong" : "text-neutral-7 hover:bg-neutral-2")}
        >
          技能 · {skills.length}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === "tools"}
          onClick={() => setTab("tools")}
          className={cn("rounded-md px-2 py-1.5 text-label-12 font-medium transition-colors", tab === "tools" ? "bg-accent-soft text-accent-strong" : "text-neutral-7 hover:bg-neutral-2")}
        >
          工具 · {tools.length}
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-2">
        {tab === "skills" ? (
          <ul className="grid gap-1.5">
            {skills.map((skill) => (
              <li key={skill.id}>
                <button
                  type="button"
                  onClick={() => onUseSkill(skill)}
                  className="w-full rounded-md border border-border bg-neutral-1 p-2.5 text-left transition-colors hover:border-accent/35 hover:bg-accent-soft/25 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/35"
                >
                  <span className="flex items-center gap-2 text-label-12 font-medium text-neutral-9">
                    <span className="text-accent-strong">{skillIcon(skill.icon)}</span>
                    <span className="min-w-0 flex-1 truncate">{skill.title}</span>
                    {skill.accepts_attachments ? <span className="rounded bg-neutral-3 px-1 py-0.5 text-caption-10 text-neutral-7">附件</span> : null}
                  </span>
                  <span className="mt-1 block text-caption-10 leading-4 text-neutral-6">{skill.description}</span>
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <ul className="grid gap-1.5">
            {tools.map((tool) => (
              <li key={tool.name}>
                <details className="group rounded-md border border-border bg-neutral-1">
                  <summary className="flex cursor-pointer list-none items-start gap-2 p-2.5">
                    <GearIcon className="mt-0.5 size-3.5 shrink-0 text-neutral-6" />
                    <span className="min-w-0 flex-1">
                      <span className="flex flex-wrap items-center gap-1.5">
                        <span className="text-label-12 font-medium text-neutral-9">{tool.label}</span>
                        {tool.writes ? <span className="rounded bg-warning-soft px-1 py-0.5 text-caption-10 text-warning">写入</span> : null}
                      </span>
                      <span className="mt-0.5 block font-mono text-caption-10 text-neutral-5">{tool.name}</span>
                      <span className="mt-1 block text-caption-10 leading-4 text-neutral-6">{tool.description}</span>
                    </span>
                    <ChevronDownIcon className="mt-0.5 size-3 shrink-0 text-neutral-6 transition-transform group-open:rotate-180" />
                  </summary>
                  <pre className="max-h-48 overflow-auto border-t border-border bg-neutral-2 p-2 font-mono text-caption-10 leading-4 text-neutral-7">{prettyParameters(tool.parameters)}</pre>
                </details>
              </li>
            ))}
          </ul>
        )}
      </div>

    </div>
  );
}