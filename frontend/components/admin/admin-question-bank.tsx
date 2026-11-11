"use client";

import { useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArchiveIcon, CheckIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { listQuestions, questionQueryKeys, updateQuestionStatus, type QuestionFilters } from "@/lib/questions";
import type { Question, QuestionStatus } from "@/lib/question-schemas";
import { statsQueryKeys } from "@/lib/stats";
import { AppShell } from "@/components/app-shell";
import { RoleAccess } from "@/components/role-access";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { QuestionFilterBar } from "@/components/question/filter-bar";
import { QuestionTable } from "@/components/question/question-table";

type ModerationStatus = Exclude<QuestionStatus, "draft">;
interface ModerationTarget { question: Question; status: ModerationStatus }

function AdminQuestions({ user }: { user: CurrentUser }) {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<QuestionFilters>({ page: 1, page_size: 10 });
  const [target, setTarget] = useState<ModerationTarget | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const listQuery = useQuery({ queryKey: questionQueryKeys.list(filters), queryFn: () => listQuestions(filters), placeholderData: keepPreviousData });
  const mutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: ModerationStatus }) => updateQuestionStatus(id, status),
    onSuccess: async (_, variables) => {
      if (listQuery.data?.items.length === 1 && filters.page > 1 && filters.status && filters.status !== variables.status) {
        setFilters((current) => ({ ...current, page: current.page - 1 }));
      }
      setTarget(null);
      setMessage(variables.status === "published" ? "题目已发布" : "题目已归档");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: questionQueryKeys.all }),
        queryClient.invalidateQueries({ queryKey: statsQueryKeys.admin }),
      ]);
    },
  });

  function actions(question: Question) {
    const pending = mutation.isPending && mutation.variables.id === question.id;
    return (
      <div className="flex justify-end gap-1">
        {question.status === "draft" ? <Button variant="ghost" size="sm" disabled={mutation.isPending} onClick={() => { setMessage(null); setTarget({ question, status: "published" }); }}><CheckIcon />发布</Button> : null}
        {question.status !== "archived" ? <Button variant="ghost" size="sm" className="text-slate-700" disabled={mutation.isPending} onClick={() => { setMessage(null); setTarget({ question, status: "archived" }); }}><ArchiveIcon />{pending && mutation.variables.status === "archived" ? "处理中" : "归档"}</Button> : <span className="text-xs text-slate-400">已归档</span>}
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-[1500px]">
      <header className="pb-6">
        <p className="text-xs font-semibold tracking-[0.16em] text-primary">管理员工作区 / 内容审核</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">全校题库</h1>
        <p className="mt-2 text-sm leading-6 text-slate-600">查看本校教师的题目与完整预览，并执行发布、归档状态流转。</p>
      </header>
      {message ? <p role="status" className="mb-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">{message}</p> : null}
      <section aria-label="全校题目检索与列表" className="overflow-hidden rounded-xl border border-border bg-white shadow-sm">
        <QuestionFilterBar key={JSON.stringify(filters)} user={user} filters={filters} scope="admin" onApply={(next) => { setFilters(next); setMessage(null); }} />
        <QuestionTable
          user={user}
          data={listQuery.data}
          isPending={listQuery.isPending}
          isFetching={listQuery.isFetching}
          error={listQuery.error}
          onRetry={() => { void listQuery.refetch(); }}
          onPageChange={(page) => setFilters((current) => ({ ...current, page }))}
          onPageSizeChange={(pageSize) => setFilters((current) => ({ ...current, page: 1, page_size: pageSize }))}
          renderActions={actions}
          emptyDescription="调整筛选条件后重新查询，或等待教师创建题目。"
        />
      </section>
      <ConfirmDialog
        open={Boolean(target)}
        title={target?.status === "published" ? "发布这道题目？" : "归档这道题目？"}
        description={target ? `${target.question.creator_name}创建的“${target.question.subject} · ${target.question.grade}”题目将${target.status === "published" ? "对校内教师开放并可用于组卷" : "从可用题目中归档"}。` : ""}
        confirmLabel={target?.status === "published" ? "确认发布" : "确认归档"}
        confirmVariant={target?.status === "published" ? "default" : "destructive"}
        isPending={mutation.isPending}
        error={mutation.isError ? mutation.error.message : undefined}
        onCancel={() => { if (!mutation.isPending) { mutation.reset(); setTarget(null); } }}
        onConfirm={() => { if (target) mutation.mutate({ id: target.question.id, status: target.status }); }}
      />
    </div>
  );
}

export function AdminQuestionBank() {
  return <RoleAccess role="admin">{(user) => <AppShell user={user}><AdminQuestions user={user} /></AppShell>}</RoleAccess>;
}
