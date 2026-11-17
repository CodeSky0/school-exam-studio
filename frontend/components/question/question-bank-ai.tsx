"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation } from "@tanstack/react-query";
import { MagicWandIcon, MagnifyingGlassIcon } from "@radix-ui/react-icons";
import { runAITool, type AIToolResult } from "@/lib/ai-tools";
import { getQuestion } from "@/lib/questions";
import type { Question } from "@/lib/question-schemas";
import { Button } from "@/components/ui/button";
import { StatusMessage } from "@/components/ui/status-message";

type ReviewItem = {
  question_id: string;
  content_preview?: string;
  subject?: string;
  grade?: string;
  knowledge_points?: string[];
};

function reviewItems(result: AIToolResult | null): ReviewItem[] {
  if (!result) return [];
  const items = result.data.items;
  if (!Array.isArray(items)) return [];
  return items.filter((item): item is ReviewItem => {
    return typeof item === "object" && item !== null && typeof (item as { question_id?: unknown }).question_id === "string";
  });
}

export function QuestionBankAI({ onEdit }: { onEdit: (question: Question) => void }) {
  const [result, setResult] = useState<AIToolResult | null>(null);
  const [activeTool, setActiveTool] = useState<"review_queue" | "bank_stats" | null>(null);

  const runMutation = useMutation({
    mutationFn: (tool: "review_queue" | "bank_stats") => runAITool(tool, tool === "review_queue" ? { limit: 20 } : {}),
    onMutate: (tool) => { setActiveTool(tool); setResult(null); },
    onSuccess: (data) => setResult(data),
  });

  const openMutation = useMutation({
    mutationFn: (id: string) => getQuestion(id),
    onSuccess: (question) => onEdit(question),
  });

  const items = reviewItems(result);
  const error = runMutation.error ?? openMutation.error;

  return (
    <section aria-label="AI 题库助手" className="mb-5 rounded-lg border border-accent/20 bg-accent-soft/40 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="flex items-center gap-2 text-copy-13 font-medium text-neutral-10">
            <MagicWandIcon className="size-4 text-accent" />AI 题库助手
          </h2>
          <p className="mt-1 text-label-12 leading-5 text-neutral-7">直接调用后端工具，不改动已发布题目；写操作只作用于草稿。</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" disabled={runMutation.isPending} onClick={() => runMutation.mutate("review_queue")}>
            <MagnifyingGlassIcon />AI 待复核
          </Button>
          <Button size="sm" variant="outline" disabled={runMutation.isPending} onClick={() => runMutation.mutate("bank_stats")}>
            <MagicWandIcon />题库体检
          </Button>
        </div>
      </div>

      <StatusMessage message={error ? error.message : null} tone="error" className="mt-3" />

      {runMutation.isPending ? <p className="mt-3 text-label-12 text-neutral-7">正在调用 AI 工具…</p> : null}

      {result ? (
        <div className="mt-3 rounded-md border border-border bg-neutral-1 p-3">
          <p className="text-copy-13 text-neutral-9">{result.summary}</p>
          {result.warnings.length > 0 ? (
            <ul className="mt-2 grid gap-1 text-label-12 text-warning">
              {result.warnings.map((warning) => <li key={warning}>{warning}</li>)}
            </ul>
          ) : null}
          {activeTool === "review_queue" && items.length > 0 ? (
            <ul className="mt-3 divide-y divide-border">
              {items.map((item) => (
                <li key={item.question_id} className="flex flex-wrap items-center justify-between gap-2 py-2">
                  <div className="min-w-0">
                    <p className="truncate text-copy-13 text-neutral-9">{item.content_preview || item.question_id}</p>
                    <p className="mt-0.5 font-mono text-label-12 text-neutral-6">{[item.subject, item.grade, ...(item.knowledge_points ?? [])].filter(Boolean).join(" · ")}</p>
                  </div>
                  <Button size="sm" variant="ghost" disabled={openMutation.isPending} onClick={() => openMutation.mutate(item.question_id)}>打开复核</Button>
                </li>
              ))}
            </ul>
          ) : null}
          {activeTool === "review_queue" && items.length > 0 ? (
            <Link
              href={`/ai?prompt=${encodeURIComponent(`帮我复核这 ${items.length} 道待复核题目：${items.slice(0, 10).map((item) => item.question_id).join("、")}。请检查答案解析、知识点与难度，并给出处理建议。`)}`}
              className="mt-2 inline-block text-label-12 font-medium text-accent-strong underline-offset-2 hover:underline"
            >
              在 AI 助手中批量复核
            </Link>
          ) : null}
          {activeTool === "bank_stats" && result.data.total !== undefined ? (
            <dl className="mt-3 grid grid-cols-2 gap-2 font-mono text-label-12 text-neutral-8 sm:grid-cols-4">
              <div><dt className="text-neutral-6">题目总数</dt><dd className="mt-1">{String(result.data.total)}</dd></div>
              <div><dt className="text-neutral-6">AI 生成</dt><dd className="mt-1">{String(result.data.ai_generated_count ?? 0)}</dd></div>
              <div><dt className="text-neutral-6">知识点</dt><dd className="mt-1">{Object.keys((result.data.knowledge_points as Record<string, number>) ?? {}).length}</dd></div>
              <div><dt className="text-neutral-6">覆盖不足</dt><dd className="mt-1">{Array.isArray(result.data.sparse_knowledge) ? result.data.sparse_knowledge.length : 0}</dd></div>
            </dl>
          ) : null}
          {activeTool === "bank_stats" ? (
            <Link
              href={`/ai?prompt=${encodeURIComponent(`请根据题库体检结果继续分析：${result.summary}。给出补齐知识点覆盖和清理待复核题目的具体步骤。`)}`}
              className="mt-2 inline-block text-label-12 font-medium text-accent-strong underline-offset-2 hover:underline"
            >
              在 AI 助手中继续
            </Link>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
