"use client";

import { useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { PlusIcon, TrashIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { batchDeleteQuestions, deleteQuestion, listQuestions, questionQueryKeys, type QuestionFilters } from "@/lib/questions";
import type { Question } from "@/lib/question-schemas";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { BatchActionBar } from "@/components/ui/batch-action-bar";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { StatusMessage } from "@/components/ui/status-message";
import dynamic from "next/dynamic";
import { CsvImport } from "./csv-import";
import { QuestionBankAI } from "./question-bank-ai";
import { QuestionFilterBar } from "./filter-bar";
import { QuestionTable } from "./question-table";

// 富文本编辑器依赖 TipTap，仅在进入编辑态时按需加载，缩小题库列表首屏脚本体积。
const QuestionEditor = dynamic(() => import("./editor").then((module) => module.QuestionEditor), {
  ssr: false,
  loading: () => <p className="text-copy-13 text-neutral-7">正在加载编辑器…</p>,
});

export function QuestionBank({ user }: { user: CurrentUser }) {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<QuestionFilters>({ page: 1, page_size: 10 });
  const [editor, setEditor] = useState<{ mode: "create" } | { mode: "edit"; question: Question } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Question | null>(null);
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false);
  const [message, setMessage] = useState<string | null>(null);

  const listQuery = useQuery({
    queryKey: questionQueryKeys.list(filters),
    queryFn: () => listQuestions(filters),
    placeholderData: keepPreviousData,
  });

  const batchDeleteMutation = useMutation({
    mutationFn: (ids: string[]) => batchDeleteQuestions(ids),
    onSuccess: async (result) => {
      setBatchDeleteOpen(false);
      setSelectedIDs([]);
      setMessage(result.failed.length > 0
        ? `已删除 ${result.deleted.length} 道草稿，${result.failed.length} 道未能删除`
        : `已删除 ${result.deleted.length} 道草稿`);
      await queryClient.invalidateQueries({ queryKey: questionQueryKeys.all });
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

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteQuestion(id),
    onSuccess: async () => {
      if (deleteTarget && listQuery.data?.items.length === 1 && filters.page > 1) {
        setFilters((current) => ({ ...current, page: current.page - 1 }));
      }
      setDeleteTarget(null);
      setMessage("草稿已删除");
      await queryClient.invalidateQueries({ queryKey: questionQueryKeys.all });
    },
  });

  return (
    <div className="mx-auto max-w-[1500px]">
      <PageHeader
        eyebrow="教师工作区 / 内容管理"
        title="题库"
        description="管理自己的草稿，查阅校内已发布题目。所有数据均来自当前学校题库。"
        actions={!editor ? <><CsvImport onImported={(count) => setMessage(`已导入 ${count} 道题目草稿，题库与个人统计已刷新`)} /><Button onClick={() => { setMessage(null); setEditor({ mode: "create" }); }}><PlusIcon />新建题目</Button></> : undefined}
      />

      <StatusMessage message={message} tone="success" className="mb-5" />
      <StatusMessage message={deleteMutation.isError ? deleteMutation.error.message : null} tone="error" className="mb-5" />
      <StatusMessage message={batchDeleteMutation.isError ? batchDeleteMutation.error.message : null} tone="error" className="mb-5" />

      {!editor ? <QuestionBankAI onEdit={(question) => { setMessage(null); setEditor({ mode: "edit", question }); }} /> : null}

      {editor ? (
        <div className="rounded-lg border border-border bg-neutral-1 p-4 shadow-sm sm:p-6">
          <QuestionEditor
            key={editor.mode === "edit" ? editor.question.id : "new-question"}
            user={user}
            question={editor.mode === "edit" ? editor.question : undefined}
            onSaved={(successMessage) => { setEditor(null); setMessage(successMessage); }}
            onCancel={() => setEditor(null)}
          />
        </div>
      ) : (
        <section aria-label="题目检索与列表" className="overflow-hidden rounded-lg border border-border bg-neutral-1 shadow-sm">
          <QuestionFilterBar user={user} filters={filters} onApply={(nextFilters) => { setFilters(nextFilters); setSelectedIDs([]); setMessage(null); }} />
          <BatchActionBar count={selectedIDs.length} onClear={() => setSelectedIDs([])}>
            <Button type="button" size="sm" variant="destructive" disabled={batchDeleteMutation.isPending} onClick={() => setBatchDeleteOpen(true)}>
              <TrashIcon />{batchDeleteMutation.isPending ? "删除中…" : "批量删除"}
            </Button>
          </BatchActionBar>
          <QuestionTable
            user={user}
            data={listQuery.data}
            isPending={listQuery.isPending}
            isFetching={listQuery.isFetching}
            error={listQuery.error}
            deletingId={deleteMutation.isPending ? deleteMutation.variables : undefined}
            onRetry={() => { void listQuery.refetch(); }}
            onEdit={(question) => { setMessage(null); setEditor({ mode: "edit", question }); }}
            onDelete={setDeleteTarget}
            onPageChange={(page) => { setSelectedIDs([]); setFilters((current) => ({ ...current, page })); }}
            onPageSizeChange={(pageSize) => { setSelectedIDs([]); setFilters((current) => ({ ...current, page: 1, page_size: pageSize })); }}
            selectedIds={selectedIDs}
            onToggleSelect={toggleSelected}
            onToggleSelectAll={toggleSelectAll}
          />
        </section>
      )}

      <ConfirmDialog
        open={batchDeleteOpen}
        title={`删除选中的 ${selectedIDs.length} 道草稿？`}
        description="只会软删除你本人创建且仍为草稿的题目；已发布、已归档或他人的题目会被跳过。此操作不会影响试卷中的已发布题目快照。"
        confirmLabel={`删除 ${selectedIDs.length} 道`}
        isPending={batchDeleteMutation.isPending}
        error={batchDeleteMutation.isError ? batchDeleteMutation.error.message : undefined}
        onCancel={() => { if (!batchDeleteMutation.isPending) { batchDeleteMutation.reset(); setBatchDeleteOpen(false); } }}
        onConfirm={() => batchDeleteMutation.mutate(selectedIDs)}
      />

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="删除这道草稿？"
        description={deleteTarget ? `将从题库中软删除“${deleteTarget.subject} · ${deleteTarget.grade} · ${deleteTarget.creator_name}”创建的这道草稿。此操作不会发布或修改其他题目。` : ""}
        confirmLabel="确认删除"
        isPending={deleteMutation.isPending}
        error={deleteMutation.isError ? deleteMutation.error.message : undefined}
        onCancel={() => { if (!deleteMutation.isPending) { deleteMutation.reset(); setDeleteTarget(null); } }}
        onConfirm={() => { if (deleteTarget) deleteMutation.mutate(deleteTarget.id); }}
      />
    </div>
  );
}
