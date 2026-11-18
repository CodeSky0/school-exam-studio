"use client";

import { useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArchiveIcon, CheckIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { batchUpdateQuestionStatus, listQuestions, questionQueryKeys, updateQuestionStatus, type QuestionFilters } from "@/lib/questions";
import type { Question, QuestionStatus } from "@/lib/question-schemas";
import { statsQueryKeys } from "@/lib/stats";
import { AppShell } from "@/components/app-shell";
import { PageHeader } from "@/components/page-header";
import { RoleAccess } from "@/components/role-access";
import { Button } from "@/components/ui/button";
import { BatchActionBar } from "@/components/ui/batch-action-bar";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { StatusMessage } from "@/components/ui/status-message";
import { QuestionFilterBar } from "@/components/question/filter-bar";
import { QuestionTable } from "@/components/question/question-table";

type ModerationStatus = Exclude<QuestionStatus, "draft">;
interface ModerationTarget { question: Question; status: ModerationStatus }

function AdminQuestions({ user }: { user: CurrentUser }) {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<QuestionFilters>({ page: 1, page_size: 10 });
  const [target, setTarget] = useState<ModerationTarget | null>(null);
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [batchTarget, setBatchTarget] = useState<ModerationStatus | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const listQuery = useQuery({ queryKey: questionQueryKeys.list(filters), queryFn: () => listQuestions(filters), placeholderData: keepPreviousData });
  const batchMutation = useMutation({
    mutationFn: ({ ids, status }: { ids: string[]; status: ModerationStatus }) => batchUpdateQuestionStatus(ids, status),
    onSuccess: async (result, variables) => {
      setBatchTarget(null);
      setSelectedIDs([]);
      setMessage(result.failed.length > 0
        ? `已${variables.status === "published" ? "发布" : "归档"} ${result.succeeded.length} 道，${result.failed.length} 道被跳过`
        : `已${variables.status === "published" ? "发布" : "归档"} ${result.succeeded.length} 道题目`);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: questionQueryKeys.all }),
        queryClient.invalidateQueries({ queryKey: statsQueryKeys.admin }),
      ]);
    },
  });

  function toggleSelected(id: string) {
    setMessage(null);
    setSelectedIDs((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  }

  function toggleSelectAll(ids: string[]) {
    setMessage(null);
    setSelectedIDs((current) => ids.every((id) => current.includes(id)) ? current.filter((id) => !ids.includes(id)) : [...new Set([...current, ...ids])]);
  }

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
      <div className="flex flex-wrap justify-end gap-1">
        {question.status === "draft" ? <Button variant="ghost" size="sm" disabled={mutation.isPending} onClick={() => { setMessage(null); setTarget({ question, status: "published" }); }}><CheckIcon />发布</Button> : null}
        {question.status !== "archived" ? <Button variant="ghost" size="sm" className="text-neutral-8" disabled={mutation.isPending} onClick={() => { setMessage(null); setTarget({ question, status: "archived" }); }}><ArchiveIcon />{pending && mutation.variables.status === "archived" ? "处理中" : "归档"}</Button> : <span className="text-label-12 text-neutral-6">已归档</span>}
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-[1500px]">
      <PageHeader eyebrow="管理员工作区 / 内容审核" title="全校题库" description="查看本校教师的题目与完整预览，并执行发布、归档状态流转。" />
      <StatusMessage message={message} tone="success" className="mb-5" />
      <StatusMessage message={batchMutation.isError ? batchMutation.error.message : null} tone="error" className="mb-5" />
      <section aria-label="全校题目检索与列表" className="overflow-hidden rounded-lg border border-border bg-neutral-1 shadow-sm">
        <QuestionFilterBar user={user} filters={filters} scope="admin" onApply={(next) => { setFilters(next); setSelectedIDs([]); setMessage(null); }} />
        <BatchActionBar count={selectedIDs.length} onClear={() => setSelectedIDs([])}>
          <Button type="button" size="sm" disabled={batchMutation.isPending} onClick={() => setBatchTarget("published")}>
            <CheckIcon />批量发布
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={batchMutation.isPending} onClick={() => setBatchTarget("archived")}>
            <ArchiveIcon />批量归档
          </Button>
        </BatchActionBar>
        <QuestionTable
          user={user}
          data={listQuery.data}
          isPending={listQuery.isPending}
          isFetching={listQuery.isFetching}
          error={listQuery.error}
          onRetry={() => { void listQuery.refetch(); }}
          onPageChange={(page) => { setSelectedIDs([]); setFilters((current) => ({ ...current, page })); }}
          onPageSizeChange={(pageSize) => { setSelectedIDs([]); setFilters((current) => ({ ...current, page: 1, page_size: pageSize })); }}
          renderActions={actions}
          emptyDescription="调整筛选条件后重新查询，或等待教师创建题目。"
          selectedIds={selectedIDs}
          onToggleSelect={toggleSelected}
          onToggleSelectAll={toggleSelectAll}
        />
      </section>
      <ConfirmDialog
        open={batchTarget !== null}
        title={batchTarget === "published" ? `发布选中的 ${selectedIDs.length} 道题目？` : `归档选中的 ${selectedIDs.length} 道题目？`}
        description={batchTarget === "published"
          ? "只有当前为草稿的题目会被发布；已归档或状态不允许的题目会被跳过，并逐条返回原因。"
          : "选中的草稿或已发布题目会被归档；已归档题目会跳过。归档后不再用于新组卷。"}
        confirmLabel={batchTarget === "published" ? "确认批量发布" : "确认批量归档"}
        confirmVariant={batchTarget === "published" ? "default" : "destructive"}
        isPending={batchMutation.isPending}
        error={batchMutation.isError ? batchMutation.error.message : undefined}
        onCancel={() => { if (!batchMutation.isPending) { batchMutation.reset(); setBatchTarget(null); } }}
        onConfirm={() => { if (batchTarget) batchMutation.mutate({ ids: selectedIDs, status: batchTarget }); }}
      />

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
