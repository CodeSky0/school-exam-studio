"use client";

import { useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { PlusIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { deleteQuestion, listQuestions, questionQueryKeys, type QuestionFilters } from "@/lib/questions";
import type { Question } from "@/lib/question-schemas";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { CsvImport } from "./csv-import";
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
      <header className="flex flex-wrap items-end justify-between gap-4 pb-6">
        <div>
          <p className="text-xs font-semibold tracking-[0.16em] text-primary">教师工作区 / 内容管理</p>
          <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">题库</h1>
          <p className="mt-2 text-sm leading-6 text-slate-600">管理自己的草稿，查阅校内已发布题目。所有数据均来自当前学校题库。</p>
        </div>
        {!editor ? <div className="flex flex-wrap gap-2"><CsvImport onImported={(count) => setMessage(`已导入 ${count} 道题目草稿，题库与个人统计已刷新`)} /><Button onClick={() => { setMessage(null); setEditor({ mode: "create" }); }}><PlusIcon />新建题目</Button></div> : null}
      </header>

      {message ? <p role="status" className="mb-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">{message}</p> : null}
      {deleteMutation.isError ? <div className="mb-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700" role="alert">{deleteMutation.error.message}</div> : null}

      {editor ? (
        <div className="rounded-xl border border-border bg-white p-4 shadow-sm sm:p-6">
          <QuestionEditor
            key={editor.mode === "edit" ? editor.question.id : "new-question"}
            user={user}
            question={editor.mode === "edit" ? editor.question : undefined}
            onSaved={(successMessage) => { setEditor(null); setMessage(successMessage); }}
            onCancel={() => setEditor(null)}
          />
        </div>
      ) : (
        <section aria-label="题目检索与列表" className="overflow-hidden rounded-xl border border-border bg-white shadow-sm">
          <QuestionFilterBar key={JSON.stringify(filters)} user={user} filters={filters} onApply={(nextFilters) => { setFilters(nextFilters); setMessage(null); }} />
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
