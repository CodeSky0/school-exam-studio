"use client";

import { useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { PlusIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { deleteQuestion, listQuestions, questionQueryKeys, type QuestionFilters } from "@/lib/questions";
import type { Question } from "@/lib/question-schemas";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { StatusMessage } from "@/components/ui/status-message";
import { CsvImport } from "./csv-import";
import { QuestionBankAI } from "./question-bank-ai";
import { QuestionEditor } from "./editor";
import { QuestionFilterBar } from "./filter-bar";
import { QuestionTable } from "./question-table";

export function QuestionBank({ user }: { user: CurrentUser }) {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<QuestionFilters>({ page: 1, page_size: 10 });
  const [editor, setEditor] = useState<{ mode: "create" } | { mode: "edit"; question: Question } | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Question | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const listQuery = useQuery({
    queryKey: questionQueryKeys.list(filters),
    queryFn: () => listQuestions(filters),
    placeholderData: keepPreviousData,
  });

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
          <QuestionFilterBar user={user} filters={filters} onApply={(nextFilters) => { setFilters(nextFilters); setMessage(null); }} />
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
            onPageChange={(page) => setFilters((current) => ({ ...current, page }))}
            onPageSizeChange={(pageSize) => setFilters((current) => ({ ...current, page: 1, page_size: pageSize }))}
          />
        </section>
      )}

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
