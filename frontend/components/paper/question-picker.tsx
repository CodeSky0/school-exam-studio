"use client";

import { useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { CheckIcon, ChevronLeftIcon, ChevronRightIcon, MagnifyingGlassIcon, PlusIcon, ResetIcon } from "@radix-ui/react-icons";
import { listQuestions, questionQueryKeys, type QuestionFilters } from "@/lib/questions";
import { questionTypeLabels, type QuestionType } from "@/lib/question-schemas";
import { FormError } from "@/components/auth-form";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
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
    <section aria-labelledby="question-picker-title" className="overflow-hidden rounded-xl border border-border bg-white shadow-sm">
      <div className="border-b border-border px-5 py-4">
        <h2 id="question-picker-title" className="text-base font-semibold text-slate-950">选择题库题目</h2>
        <p className="mt-1 text-xs leading-5 text-slate-500">仅检索本校已发布题目，加入后可调整顺序与分值。</p>
      </div>
      <form onSubmit={applyFilters} className="border-b border-border bg-slate-50/60 p-4" aria-label="组卷题目筛选">
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <div className="grid gap-1.5">
            <Label htmlFor="picker-subject" className="text-xs">学科</Label>
            <Input id="picker-subject" className="h-9 bg-white" value={draft.subject ?? ""} placeholder="全部学科" onChange={(event) => setDraft({ ...draft, subject: event.target.value })} />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="picker-grade" className="text-xs">年级</Label>
            <Input id="picker-grade" className="h-9 bg-white" value={draft.grade ?? ""} placeholder="全部年级" onChange={(event) => setDraft({ ...draft, grade: event.target.value })} />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="picker-type" className="text-xs">题型</Label>
            <Select id="picker-type" className="h-9 bg-white" value={draft.question_type ?? ""} onChange={(event) => setDraft({ ...draft, question_type: (event.target.value || undefined) as QuestionType | undefined })}>
              <option value="">全部题型</option>
              {Object.entries(questionTypeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </Select>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="picker-difficulty" className="text-xs">难度</Label>
            <Select id="picker-difficulty" className="h-9 bg-white" value={draft.difficulty ?? ""} onChange={(event) => setDraft({ ...draft, difficulty: event.target.value ? Number(event.target.value) : undefined })}>
              <option value="">全部难度</option>
              {[1, 2, 3, 4, 5].map((value) => <option key={value} value={value}>{value} 级</option>)}
            </Select>
          </div>
          <div className="grid gap-1.5 sm:col-span-2 xl:col-span-1">
            <Label htmlFor="picker-tag" className="text-xs">知识点标签</Label>
            <Input id="picker-tag" className="h-9 bg-white" value={draft.tag ?? ""} placeholder="精确标签" onChange={(event) => setDraft({ ...draft, tag: event.target.value })} />
          </div>
        </div>
        <div className="mt-3 flex justify-end gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={resetFilters}><ResetIcon />重置</Button>
          <Button type="submit" size="sm"><MagnifyingGlassIcon />应用筛选</Button>
        </div>
      </form>

      <p className="sr-only" role="status" aria-live="polite">{message}</p>
      {listQuery.isPending ? (
        <div className="grid animate-pulse gap-px bg-border" aria-label="正在读取已发布题目">
          {[0, 1, 2].map((item) => <div key={item} className="h-36 bg-white" />)}
        </div>
      ) : listQuery.error ? (
        <div className="p-5"><FormError error={listQuery.error} /><Button variant="outline" size="sm" className="mt-3" onClick={() => { void listQuery.refetch(); }}>重新读取</Button></div>
      ) : !data || data.items.length === 0 ? (
        <div className="px-5 py-14 text-center">
          <h3 className="text-sm font-semibold text-slate-900">没有符合条件的已发布题目</h3>
          <p className="mt-2 text-sm text-slate-500">调整学科、年级或其他筛选条件后再试。</p>
        </div>
      ) : (
        <div aria-busy={listQuery.isFetching}>
          <div className="flex items-center justify-between px-5 py-3 text-xs text-slate-500">
            <span>共 {data.total} 道已发布题目</span>
            {listQuery.isFetching ? <span>正在更新…</span> : null}
          </div>
          <ul className="divide-y divide-border border-y border-border">
            {data.items.map((question) => {
              const isAdded = items.some((item) => item.question.id === question.id);
              return (
                <li key={question.id} className="px-5 py-4">
                  <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
                    <span className="rounded-full bg-primary-soft px-2 py-0.5 font-medium text-primary-strong">{questionTypeLabels[question.question_type]}</span>
                    <span>{question.subject} · {question.grade}</span>
                    <span className="font-mono">难度 {question.difficulty}/5</span>
                  </div>
                  <RichTextView content={question.content} className="mt-3 text-sm leading-6 text-slate-800" />
                  {question.options.length > 0 ? (
                    <div className="mt-3 grid gap-1.5 text-xs text-slate-600 sm:grid-cols-2">
                      {question.options.map((option) => (
                        <div key={option.key} className="flex items-start gap-2">
                          <span className="shrink-0 font-mono font-semibold text-slate-700">{option.key}.</span>
                          <RichTextView content={option.content} className="text-xs leading-5" />
                        </div>
                      ))}
                    </div>
                  ) : null}
                  <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
                    <div className="flex flex-wrap gap-1">
                      {question.tags.map((tag) => <span key={tag} className="rounded bg-slate-100 px-1.5 py-0.5 text-[11px] text-slate-600">{tag}</span>)}
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
                </li>
              );
            })}
          </ul>
          <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-4">
            <div className="flex items-center gap-2">
              <Label htmlFor="picker-page-size" className="text-xs">每页</Label>
              <Select id="picker-page-size" className="h-8 w-20" value={data.page_size} onChange={(event) => {
                const next = { ...filters, page: 1, page_size: Number(event.target.value) };
                setFilters(next);
                setDraft(next);
              }}>
                {[6, 12, 24].map((value) => <option key={value} value={value}>{value}</option>)}
              </Select>
            </div>
            <div className="flex items-center gap-2">
              <span className="font-mono text-xs text-slate-600">第 {data.page} / {totalPages} 页</span>
              <Button variant="outline" size="sm" disabled={data.page <= 1} onClick={() => setFilters({ ...filters, page: data.page - 1 })}><ChevronLeftIcon />上一页</Button>
              <Button variant="outline" size="sm" disabled={data.page >= totalPages} onClick={() => setFilters({ ...filters, page: data.page + 1 })}>下一页<ChevronRightIcon /></Button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
