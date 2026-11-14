"use client";

import { useState } from "react";
import { MagnifyingGlassIcon, ResetIcon } from "@radix-ui/react-icons";
import { questionStatusLabels, questionTypeLabels, type QuestionStatus, type QuestionType } from "@/lib/question-schemas";
import type { QuestionFilters } from "@/lib/questions";
import type { CurrentUser } from "@/lib/schemas";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { AdminCreatorSelect } from "./admin-creator-select";

export function QuestionFilterBar({ user, filters, onApply, scope = "teacher" }: {
  user: CurrentUser;
  filters: QuestionFilters;
  onApply: (filters: QuestionFilters) => void;
  scope?: "teacher" | "admin";
}) {
  const [draftState, setDraftState] = useState({ source: filters, value: filters });
  const draft = draftState.source === filters ? draftState.value : filters;

  function setDraft(value: QuestionFilters) {
    setDraftState({ source: filters, value });
  }

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onApply({ ...draft, page: 1 });
  }

  function reset() {
    const next = { page: 1, page_size: filters.page_size };
    setDraft(next);
    onApply(next);
  }

  return (
    <form onSubmit={submit} className="border-y border-border bg-neutral-1 px-4 py-4" aria-label="题目组合筛选">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-7">
        <div className="grid gap-1.5">
          <Label htmlFor="filter-subject" className="text-label-12">学科</Label>
          <Input id="filter-subject" className="h-9" value={draft.subject ?? ""} placeholder="全部学科" onChange={(event) => setDraft({ ...draft, subject: event.target.value })} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="filter-grade" className="text-label-12">年级</Label>
          <Input id="filter-grade" className="h-9" value={draft.grade ?? ""} placeholder="全部年级" onChange={(event) => setDraft({ ...draft, grade: event.target.value })} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="filter-type" className="text-label-12">题型</Label>
          <Select id="filter-type" className="h-9" value={draft.question_type ?? ""} onChange={(event) => setDraft({ ...draft, question_type: (event.target.value || undefined) as QuestionType | undefined })}>
            <option value="">全部题型</option>
            {Object.entries(questionTypeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </Select>
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="filter-difficulty" className="text-label-12">难度</Label>
          <Select id="filter-difficulty" className="h-9" value={draft.difficulty ?? ""} onChange={(event) => setDraft({ ...draft, difficulty: event.target.value ? Number(event.target.value) : undefined })}>
            <option value="">全部难度</option>
            {[1, 2, 3, 4, 5].map((value) => <option key={value} value={value}>{value} 级</option>)}
          </Select>
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="filter-tag" className="text-label-12">知识点标签</Label>
          <Input id="filter-tag" className="h-9" value={draft.tag ?? ""} placeholder="精确标签" onChange={(event) => setDraft({ ...draft, tag: event.target.value })} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="filter-creator" className="text-label-12">创建范围</Label>
          {scope === "admin" ? <AdminCreatorSelect id="filter-creator" user={user} value={draft.creator_id} onChange={(creatorId) => setDraft({ ...draft, creator_id: creatorId })} /> : <Select id="filter-creator" className="h-9" value={draft.creator_id ?? ""} onChange={(event) => setDraft({ ...draft, creator_id: event.target.value || undefined })}>
            <option value="">全部可见题目</option>
            <option value={user.id}>仅我创建</option>
          </Select>}
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="filter-status" className="text-label-12">状态</Label>
          <Select id="filter-status" className="h-9" value={draft.status ?? ""} onChange={(event) => setDraft({ ...draft, status: (event.target.value || undefined) as QuestionStatus | undefined })}>
            <option value="">全部状态</option>
            {Object.entries(questionStatusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </Select>
        </div>
      </div>
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <p className="text-label-12 leading-5 text-neutral-7">{scope === "admin" ? "管理员可查看本校全部状态题目，并可按教师姓名筛选创建人。" : "可查看自己的全部题目，以及其他教师已发布的题目。"}</p>
        <div className="flex gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={reset}><ResetIcon />重置</Button>
          <Button type="submit" size="sm"><MagnifyingGlassIcon />应用筛选</Button>
        </div>
      </div>
    </form>
  );
}
