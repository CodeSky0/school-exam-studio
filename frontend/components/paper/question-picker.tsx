"use client";

import { useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { CheckIcon, ChevronLeftIcon, ChevronRightIcon, MagnifyingGlassIcon, PlusIcon, ResetIcon } from "@radix-ui/react-icons";
import { motion } from "framer-motion";
import { listQuestions, questionQueryKeys, type QuestionFilters } from "@/lib/questions";
import { questionTypeLabels, type QuestionType } from "@/lib/question-schemas";
import { FormError } from "@/components/auth-form";
import { listContainerVariants, listItemVariants } from "@/components/motion-presets";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { StatusMessage } from "@/components/ui/status-message";
import { RichTextView } from "@/components/question/rich-text-view";
import { usePaperStore } from "./paper-store-provider";

const emptyFilters = (pageSize = 6): QuestionFilters => ({
  status: "published",
  page: 1,
  page_size: pageSize,
});

export function QuestionPicker({ defaultSubject }: { defaultSubject: string }) {
  const [filters, setFilters] = useState<QuestionFilters>(() => ({ ...emptyFilters(), subject: defaultSubject }));
  const [draft, setDraft] = useState(filters);
  const [message, setMessage] = useState("");
  const items = usePaperStore((state) => state.items);
  const addQuestion = usePaperStore((state) => state.addQuestion);
  const listQuery = useQuery({
    queryKey: questionQueryKeys.list(filters),
    queryFn: () => listQuestions(filters),
    placeholderData: keepPreviousData,
  });

  function applyFilters(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage("");
    setFilters({ ...draft, status: "published", page: 1 });
  }

  function resetFilters() {
    const next = emptyFilters(filters.page_size);
    setDraft(next);
    setFilters(next);
    setMessage("");
  }

  const data = listQuery.data;
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / (data?.page_size ?? filters.page_size)));

  return (
    <section aria-labelledby="question-picker-title" className="overflow-hidden rounded-lg border border-border bg-neutral-1 shadow-sm">
      <div className="border-b border-border px-5 py-4">
        <h2 id="question-picker-title" className="text-title-20 font-medium text-neutral-10">选择题库题目</h2>
        <p className="mt-1 text-label-12 leading-5 text-neutral-7">仅检索本校已发布题目，加入后可调整顺序与分值。</p>
      </div>
      <form onSubmit={applyFilters} className="border-b border-border bg-neutral-2/60 p-4" aria-label="组卷题目筛选">
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <div className="grid gap-1.5">
            <Label htmlFor="picker-subject" className="text-label-12">学科</Label>
            <Input id="picker-subject" className="h-9 bg-neutral-1" value={draft.subject ?? ""} placeholder="全部学科" onChange={(event) => setDraft({ ...draft, subject: event.target.value })} />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="picker-grade" className="text-label-12">年级</Label>
            <Input id="picker-grade" className="h-9 bg-neutral-1" value={draft.grade ?? ""} placeholder="全部年级" onChange={(event) => setDraft({ ...draft, grade: event.target.value })} />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="picker-type" className="text-label-12">题型</Label>
            <Select id="picker-type" className="h-9 bg-neutral-1" value={draft.question_type ?? ""} onChange={(event) => setDraft({ ...draft, question_type: (event.target.value || undefined) as QuestionType | undefined })}>
              <option value="">全部题型</option>
              {Object.entries(questionTypeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </Select>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="picker-difficulty" className="text-label-12">难度</Label>
            <Select id="picker-difficulty" className="h-9 bg-neutral-1" value={draft.difficulty ?? ""} onChange={(event) => setDraft({ ...draft, difficulty: event.target.value ? Number(event.target.value) : undefined })}>
              <option value="">全部难度</option>
              {[1, 2, 3, 4, 5].map((value) => <option key={value} value={value}>{value} 级</option>)}
            </Select>
          </div>
          <div className="grid gap-1.5 sm:col-span-2 xl:col-span-1">
            <Label htmlFor="picker-tag" className="text-label-12">知识点标签</Label>
            <Input id="picker-tag" className="h-9 bg-neutral-1" value={draft.tag ?? ""} placeholder="精确标签" onChange={(event) => setDraft({ ...draft, tag: event.target.value })} />
          </div>
        </div>
        <div className="mt-3 flex justify-end gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={resetFilters}><ResetIcon />重置</Button>
          <Button type="submit" size="sm"><MagnifyingGlassIcon />应用筛选</Button>
        </div>
      </form>

      <StatusMessage message={message} tone={message.startsWith("已添加") ? "success" : "info"} className="mx-5 mt-4" />
      {listQuery.isPending ? (
        <div className="animate-pulse" role="status" aria-label="正在读取已发布题目">
          <div className="divide-y divide-border border-y border-border" aria-hidden="true">
            {Array.from({ length: filters.page_size ?? 6 }, (_, item) => (
              <div key={item} className="px-5 py-4">
                <div className="flex items-center gap-2">
                  <div className="h-5 w-16 rounded-full bg-neutral-3" />
                  <div className="h-4 w-28 rounded bg-neutral-3" />
                  <div className="h-4 w-16 rounded bg-neutral-3" />
                </div>
                <div className="mt-4 h-4 w-full rounded bg-neutral-3" />
                <div className="mt-2 h-4 w-4/5 rounded bg-neutral-3" />
                <div className="mt-4 flex items-center justify-between gap-4">
                  <div className="h-5 w-24 rounded bg-neutral-3" />
                  <div className="h-8 w-24 rounded-md bg-neutral-3" />
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : listQuery.error ? (
        <div className="p-5"><FormError error={listQuery.error} /><Button variant="outline" size="sm" className="mt-3" onClick={() => { void listQuery.refetch(); }}>重新读取</Button></div>
      ) : !data || data.items.length === 0 ? (
        <div className="px-5 py-14 text-center">
          <h3 className="text-copy-13 font-medium text-neutral-10">没有符合条件的已发布题目</h3>
          <p className="mt-2 text-copy-13 text-neutral-7">调整学科、年级或其他筛选条件后再试。</p>
        </div>
      ) : (
        <div aria-busy={listQuery.isFetching}>
          <div className="flex items-center justify-between px-5 py-3 text-label-12 text-neutral-7">
            <span>共 {data.total} 道已发布题目</span>
            {listQuery.isFetching ? <span>正在更新…</span> : null}
          </div>
          <motion.ul
            key={data.items.map((question) => question.id).join(",")}
            className="divide-y divide-border border-y border-border"
            variants={listContainerVariants}
            initial="hidden"
            animate="visible"
          >
            {data.items.map((question) => {
              const isAdded = items.some((item) => item.question.id === question.id);
              return (
                <motion.li key={question.id} variants={listItemVariants} className="px-5 py-4">
                  <div className="flex flex-wrap items-center gap-2 text-label-12 text-neutral-7">
                    <span className="rounded-full bg-accent-soft px-2 py-0.5 font-medium text-accent-strong">{questionTypeLabels[question.question_type]}</span>
                    <span>{question.subject} · {question.grade}</span>
                    <span className="font-mono">难度 {question.difficulty}/5</span>
                  </div>
                  <RichTextView content={question.content} className="mt-3 text-copy-13 leading-6 text-neutral-9" />
                  {question.options.length > 0 ? (
                    <div className="mt-3 grid gap-1.5 text-label-12 text-neutral-7 sm:grid-cols-2">
                      {question.options.map((option) => (
                        <div key={option.key} className="flex items-start gap-2">
                          <span className="shrink-0 font-mono font-medium text-neutral-8">{option.key}.</span>
                          <RichTextView content={option.content} className="text-label-12 leading-5" />
                        </div>
                      ))}
                    </div>
                  ) : null}
                  <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
                    <div className="flex flex-wrap gap-1">
                      {question.tags.map((tag) => <span key={tag} className="rounded bg-neutral-3 px-1.5 py-0.5 text-label-12 text-neutral-7">{tag}</span>)}
                    </div>
                    <Button
                      type="button"
                      variant={isAdded ? "ghost" : "outline"}
                      size="sm"
                      disabled={isAdded}
                      onClick={() => {
                        const added = addQuestion(question);
                        setMessage(added ? `已添加一道${questionTypeLabels[question.question_type]}` : "这道题已在购物车中");
                      }}
                    >
                      {isAdded ? <CheckIcon /> : <PlusIcon />}{isAdded ? "已添加" : "加入试卷"}
                    </Button>
                  </div>
                </motion.li>
              );
            })}
          </motion.ul>
          <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-4">
            <div className="flex items-center gap-2">
              <Label htmlFor="picker-page-size" className="text-label-12">每页</Label>
              <Select id="picker-page-size" className="h-8 w-20" value={data.page_size} onChange={(event) => {
                const next = { ...filters, page: 1, page_size: Number(event.target.value) };
                setFilters(next);
                setDraft(next);
              }}>
                {[6, 12, 24].map((value) => <option key={value} value={value}>{value}</option>)}
              </Select>
            </div>
            <div className="flex items-center gap-2">
              <span className="font-mono text-label-12 text-neutral-7">第 {data.page} / {totalPages} 页</span>
              <Button variant="outline" size="sm" disabled={data.page <= 1} onClick={() => setFilters({ ...filters, page: data.page - 1 })}><ChevronLeftIcon />上一页</Button>
              <Button variant="outline" size="sm" disabled={data.page >= totalPages} onClick={() => setFilters({ ...filters, page: data.page + 1 })}>下一页<ChevronRightIcon /></Button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
