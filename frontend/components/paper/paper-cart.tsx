"use client";

import { useLayoutEffect, useMemo, useRef, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { ArrowDownIcon, ArrowUpIcon, DragHandleDots2Icon, TrashIcon } from "@radix-ui/react-icons";
import { questionTypeLabels } from "@/lib/question-schemas";
import { sumScores, toCents } from "@/lib/score";
import { cn } from "@/lib/utils";
import { cartSpring, listItemVariants } from "@/components/motion-presets";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RichTextView } from "@/components/question/rich-text-view";
import { usePaperStore } from "./paper-store-provider";

type PendingFocus = {
  type: "move";
  questionId: string;
  direction: "up" | "down";
} | {
  type: "delete";
  questionId: string | null;
};

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
  const [sortMessage, setSortMessage] = useState("");
  const titleRef = useRef<HTMLHeadingElement>(null);
  const pendingFocusRef = useRef<PendingFocus | null>(null);
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

  useLayoutEffect(() => {
    const pendingFocus = pendingFocusRef.current;
    if (!pendingFocus) return;
    pendingFocusRef.current = null;
    const target = pendingFocus.type === "move"
      ? document.getElementById(`cart-move-${pendingFocus.direction}-${pendingFocus.questionId}`)
      : pendingFocus.questionId
        ? document.getElementById(`cart-question-${pendingFocus.questionId}`)
        : titleRef.current;
    (target ?? titleRef.current)?.focus({ preventScroll: true });
  }, [items]);

  function handleMove(questionId: string, offset: -1 | 1) {
    const index = items.findIndex((item) => item.question.id === questionId);
    const targetIndex = index + offset;
    if (index < 0 || targetIndex < 0 || targetIndex >= items.length) return;
    const direction = offset === -1 ? "up" : "down";
    pendingFocusRef.current = { type: "move", questionId, direction };
    moveQuestion(questionId, offset);
    setSortMessage(`已将第 ${index + 1} 题${offset === -1 ? "上移" : "下移"}至第 ${targetIndex + 1} 题`);
  }

  function handleDelete(questionId: string) {
    const index = items.findIndex((item) => item.question.id === questionId);
    if (index < 0) return;
    const adjacentId = items[index + 1]?.question.id ?? items[index - 1]?.question.id ?? null;
    pendingFocusRef.current = { type: "delete", questionId: adjacentId };
    removeQuestion(questionId);
  }

  return (
    <section aria-labelledby="paper-cart-title" className="overflow-hidden rounded-lg border border-border bg-neutral-1 shadow-sm">
      <div className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
        <div>
          <h2 ref={titleRef} tabIndex={-1} id="paper-cart-title" className="rounded-sm text-title-20 font-medium text-neutral-10 outline-none focus-visible:ring-2 focus-visible:ring-accent/35">卷面题目</h2>
          <p className="mt-1 text-label-12 leading-5 text-neutral-7">拖动排序，或使用每题的上移、下移按钮。</p>
        </div>
        <div className="shrink-0 text-right">
          <p className="text-label-12 text-neutral-7">{items.length} 道题</p>
          <p className="mt-1 font-mono text-copy-16 font-medium text-neutral-10">{totalScore ?? "—"}<span className="ml-1 font-sans text-label-12 font-normal text-neutral-7">分</span></p>
        </div>
      </div>
      <p className="sr-only" role="status" aria-live="polite" aria-atomic="true">{sortMessage}</p>
      <div aria-live="polite">
        {isResolving ? <p className="border-b border-accent-soft-strong bg-accent-soft px-5 py-2.5 text-label-12 text-info">正在校验恢复的题目…</p> : null}
        {resolutionError ? <p role="alert" className="border-b border-error-soft-strong bg-error-soft px-5 py-2.5 text-label-12 leading-5 text-error">{resolutionError}。当前内容仍保留，可重试刷新页面。</p> : null}
        {removedInvalidCount > 0 ? <p role="status" className="border-b border-warning/30 bg-warning-soft px-5 py-2.5 text-label-12 text-warning">已移除 {removedInvalidCount} 道不再发布或不可用的题目。</p> : null}
      </div>
      {items.length === 0 ? (
        <div className="px-5 py-12 text-center">
          <DragHandleDots2Icon className="mx-auto size-6 text-neutral-5" />
          <h3 className="mt-3 text-copy-13 font-medium text-neutral-10">试卷还是空的</h3>
          <p className="mt-2 text-label-12 leading-5 text-neutral-7">从左侧已发布题目中加入至少一道题，再设置卷面信息。</p>
        </div>
      ) : (
        <ol className="divide-y divide-border" aria-label="已选题目，按试卷顺序排列">
          <AnimatePresence initial={false}>
            {items.map((item, index) => {
              const question = item.question;
              const error = scoreError(item.assignedScore);
              const errorId = `cart-score-error-${question.id}`;
              return (
                <motion.li
                  id={`cart-question-${question.id}`}
                  key={question.id}
                  tabIndex={-1}
                  layout="position"
                  variants={listItemVariants}
                  initial="hidden"
                  animate="visible"
                  exit="exit"
                  transition={cartSpring}
                  className={cn("bg-neutral-1 px-4 py-4 outline-none transition-colors focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent/35", draggingId === question.id && "bg-accent-soft/70")}
                  onDragOver={(event) => {
                    event.preventDefault();
                    event.dataTransfer.dropEffect = "move";
                  }}
                  onDrop={(event) => {
                    event.preventDefault();
                    const sourceId = event.dataTransfer.getData("text/plain") || draggingId;
                    const sourceIndex = items.findIndex((cartItem) => cartItem.question.id === sourceId);
                    if (sourceId && sourceIndex >= 0 && sourceId !== question.id) {
                      reorderQuestion(sourceId, question.id);
                      setSortMessage(`已将第 ${sourceIndex + 1} 题移动至第 ${index + 1} 题`);
                    }
                    setDraggingId(null);
                  }}
                >
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      draggable
                      aria-label={`拖动第 ${index + 1} 题排序`}
                      className="flex size-8 cursor-grab items-center justify-center rounded-md text-neutral-6 outline-none hover:bg-neutral-3 hover:text-neutral-8 focus-visible:ring-2 focus-visible:ring-accent/35 active:cursor-grabbing"
                      onDragStart={(event) => {
                        event.dataTransfer.effectAllowed = "move";
                        event.dataTransfer.setData("text/plain", question.id);
                        setDraggingId(question.id);
                      }}
                      onDragEnd={() => setDraggingId(null)}
                    >
                      <DragHandleDots2Icon />
                    </button>
                    <span className="font-mono text-label-12 font-medium text-neutral-7">{String(index + 1).padStart(2, "0")}</span>
                    <span className="rounded-full bg-neutral-3 px-2 py-0.5 text-label-12 font-medium text-neutral-8">{questionTypeLabels[question.question_type]}</span>
                    <div className="ml-auto flex items-center gap-0.5">
                      <Button id={`cart-move-up-${question.id}`} type="button" variant="ghost" size="icon" className="size-8 aria-disabled:pointer-events-none aria-disabled:opacity-50" aria-label={`上移第 ${index + 1} 题`} aria-disabled={index === 0} onClick={() => handleMove(question.id, -1)}><ArrowUpIcon /></Button>
                      <Button id={`cart-move-down-${question.id}`} type="button" variant="ghost" size="icon" className="size-8 aria-disabled:pointer-events-none aria-disabled:opacity-50" aria-label={`下移第 ${index + 1} 题`} aria-disabled={index === items.length - 1} onClick={() => handleMove(question.id, 1)}><ArrowDownIcon /></Button>
                      <Button type="button" variant="ghost" size="icon" className="size-8 text-error hover:bg-error-soft hover:text-error" aria-label={`删除第 ${index + 1} 题`} onClick={() => handleDelete(question.id)}><TrashIcon /></Button>
                    </div>
                  </div>
                  <RichTextView content={question.content} className="mt-2 max-h-24 overflow-hidden text-copy-13 leading-5" />
                  <div className="mt-3 flex flex-wrap items-end justify-between gap-3">
                    <p className="text-label-12 text-neutral-7">{question.subject} · {question.grade}</p>
                    <div className="grid gap-1">
                      <Label htmlFor={`cart-score-${question.id}`} className="text-label-12">本题分值</Label>
                      <Input
                        id={`cart-score-${question.id}`}
                        inputMode="decimal"
                        autoComplete="off"
                        className="h-8 w-28 px-2 font-mono text-label-12"
                        value={item.assignedScore}
                        aria-invalid={Boolean(error)}
                        aria-describedby={error ? errorId : undefined}
                        onChange={(event) => setScore(question.id, event.target.value)}
                      />
                      {error ? <p id={errorId} className="text-label-12 text-error">{error}</p> : null}
                    </div>
                  </div>
                </motion.li>
              );
            })}
          </AnimatePresence>
        </ol>
      )}
      {hasInvalidScore ? <p role="alert" className="border-t border-error-soft-strong bg-error-soft px-5 py-2.5 text-label-12 text-error">完善所有分值后才会计算总分并允许保存。</p> : null}
      {hasTotalError ? <p role="alert" className="border-t border-error-soft-strong bg-error-soft px-5 py-2.5 text-label-12 text-error">试卷总分不能超过 999999.99 分，请降低题目分值。</p> : null}
    </section>
  );
}
