"use client";

import { useMemo, useState } from "react";
import { ArrowDownIcon, ArrowUpIcon, DragHandleDots2Icon, TrashIcon } from "@radix-ui/react-icons";
import { questionTypeLabels } from "@/lib/question-schemas";
import { sumScores, toCents } from "@/lib/score";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RichTextView } from "@/components/question/rich-text-view";
import { usePaperStore } from "./paper-store-provider";

function scoreError(value: string): string | null {
  try {
    toCents(value);
    return null;
  } catch {
    return "须为大于 0 的两位小数";
  }
}

export function PaperCart() {
  const items = usePaperStore((state) => state.items);
  const isResolving = usePaperStore((state) => state.isResolving);
  const resolutionError = usePaperStore((state) => state.resolutionError);
  const removedInvalidCount = usePaperStore((state) => state.removedInvalidCount);
  const removeQuestion = usePaperStore((state) => state.removeQuestion);
  const setScore = usePaperStore((state) => state.setScore);
  const moveQuestion = usePaperStore((state) => state.moveQuestion);
  const reorderQuestion = usePaperStore((state) => state.reorderQuestion);
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const hasInvalidScore = items.some((item) => scoreError(item.assignedScore));
  const totalScore = useMemo(() => {
    if (hasInvalidScore) return null;
    try {
      return sumScores(items.map((item) => item.assignedScore));
    } catch {
      return null;
    }
  }, [hasInvalidScore, items]);
  const hasTotalError = !hasInvalidScore && totalScore === null && items.length > 0;

  return (
    <section aria-labelledby="paper-cart-title" className="overflow-hidden rounded-xl border border-border bg-white shadow-sm">
      <div className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
        <div>
          <h2 id="paper-cart-title" className="text-base font-semibold text-slate-950">卷面题目</h2>
          <p className="mt-1 text-xs leading-5 text-slate-500">拖动排序，或使用每题的上移、下移按钮。</p>
        </div>
        <div className="shrink-0 text-right">
          <p className="text-xs text-slate-500">{items.length} 道题</p>
          <p className="mt-1 font-mono text-lg font-semibold text-slate-950">{totalScore ?? "—"}<span className="ml-1 font-sans text-xs font-normal text-slate-500">分</span></p>
        </div>
      </div>
      <div aria-live="polite">
        {isResolving ? <p className="border-b border-blue-100 bg-blue-50 px-5 py-2.5 text-xs text-blue-800">正在校验恢复的题目…</p> : null}
        {resolutionError ? <p role="alert" className="border-b border-red-200 bg-red-50 px-5 py-2.5 text-xs leading-5 text-red-700">{resolutionError}。当前内容仍保留，可重试刷新页面。</p> : null}
        {removedInvalidCount > 0 ? <p role="status" className="border-b border-amber-200 bg-amber-50 px-5 py-2.5 text-xs text-amber-800">已移除 {removedInvalidCount} 道不再发布或不可用的题目。</p> : null}
      </div>
      {items.length === 0 ? (
        <div className="px-5 py-12 text-center">
          <DragHandleDots2Icon className="mx-auto size-6 text-slate-300" />
          <h3 className="mt-3 text-sm font-semibold text-slate-900">试卷还是空的</h3>
          <p className="mt-2 text-xs leading-5 text-slate-500">从左侧已发布题目中加入至少一道题，再设置卷面信息。</p>
        </div>
      ) : (
        <ol className="divide-y divide-border" aria-label="已选题目，按试卷顺序排列">
          {items.map((item, index) => {
            const question = item.question;
            const error = scoreError(item.assignedScore);
            const errorId = `cart-score-error-${question.id}`;
            return (
              <li
                key={question.id}
                className={cn("bg-white px-4 py-4 transition-colors", draggingId === question.id && "bg-blue-50/70")}
                onDragOver={(event) => {
                  event.preventDefault();
                  event.dataTransfer.dropEffect = "move";
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  const sourceId = event.dataTransfer.getData("text/plain") || draggingId;
                  if (sourceId) reorderQuestion(sourceId, question.id);
                  setDraggingId(null);
                }}
              >
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    draggable
                    aria-label={`拖动第 ${index + 1} 题排序`}
                    className="flex size-8 cursor-grab items-center justify-center rounded-md text-slate-400 outline-none hover:bg-slate-100 hover:text-slate-700 focus-visible:ring-2 focus-visible:ring-primary/35 active:cursor-grabbing"
                    onDragStart={(event) => {
                      event.dataTransfer.effectAllowed = "move";
                      event.dataTransfer.setData("text/plain", question.id);
                      setDraggingId(question.id);
                    }}
                    onDragEnd={() => setDraggingId(null)}
                  >
                    <DragHandleDots2Icon />
                  </button>
                  <span className="font-mono text-xs font-semibold text-slate-500">{String(index + 1).padStart(2, "0")}</span>
                  <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-700">{questionTypeLabels[question.question_type]}</span>
                  <div className="ml-auto flex items-center gap-0.5">
                    <Button type="button" variant="ghost" size="icon" className="size-8" aria-label={`上移第 ${index + 1} 题`} disabled={index === 0} onClick={() => moveQuestion(question.id, -1)}><ArrowUpIcon /></Button>
                    <Button type="button" variant="ghost" size="icon" className="size-8" aria-label={`下移第 ${index + 1} 题`} disabled={index === items.length - 1} onClick={() => moveQuestion(question.id, 1)}><ArrowDownIcon /></Button>
                    <Button type="button" variant="ghost" size="icon" className="size-8 text-red-700 hover:bg-red-50 hover:text-red-800" aria-label={`删除第 ${index + 1} 题`} onClick={() => removeQuestion(question.id)}><TrashIcon /></Button>
                  </div>
                </div>
                <RichTextView content={question.content} className="mt-2 max-h-24 overflow-hidden text-[13px] leading-5" />
                <div className="mt-3 flex flex-wrap items-end justify-between gap-3">
                  <p className="text-xs text-slate-500">{question.subject} · {question.grade}</p>
                  <div className="grid gap-1">
                    <Label htmlFor={`cart-score-${question.id}`} className="text-xs">本题分值</Label>
                    <Input
                      id={`cart-score-${question.id}`}
                      inputMode="decimal"
                      autoComplete="off"
                      className="h-8 w-28 px-2 font-mono text-xs"
                      value={item.assignedScore}
                      aria-invalid={Boolean(error)}
                      aria-describedby={error ? errorId : undefined}
                      onChange={(event) => setScore(question.id, event.target.value)}
                    />
                    {error ? <p id={errorId} className="text-[11px] text-red-600">{error}</p> : null}
                  </div>
                </div>
              </li>
            );
          })}
        </ol>
      )}
      {hasInvalidScore ? <p role="alert" className="border-t border-red-200 bg-red-50 px-5 py-2.5 text-xs text-red-700">完善所有分值后才会计算总分并允许保存。</p> : null}
      {hasTotalError ? <p role="alert" className="border-t border-red-200 bg-red-50 px-5 py-2.5 text-xs text-red-700">试卷总分不能超过 999999.99 分，请降低题目分值。</p> : null}
    </section>
  );
}
