"use client";

import { useState } from "react";
import Link from "next/link";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronLeftIcon, ChevronRightIcon, EyeOpenIcon, MagnifyingGlassIcon, Pencil1Icon, FileTextIcon, ResetIcon, TrashIcon } from "@radix-ui/react-icons";
import { deletePaper, listPapers, paperQueryKeys, type PaperFilters } from "@/lib/papers";
import type { PaperSummary } from "@/lib/paper-schemas";
import { FormError } from "@/components/auth-form";
import { Button, buttonVariants } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";

function formatUpdatedAt(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
    timeZone: "Asia/Shanghai",
  }).format(date);
}

export function PaperList() {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<PaperFilters>({ page: 1, page_size: 10 });
  const [draft, setDraft] = useState<PaperFilters>(filters);
  const [deleteTarget, setDeleteTarget] = useState<PaperSummary | null>(null);
  const [message, setMessage] = useState("");
  const listQuery = useQuery({
    queryKey: paperQueryKeys.list(filters),
    queryFn: () => listPapers(filters),
    placeholderData: keepPreviousData,
  });
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deletePaper(id),
    onSuccess: async () => {
      if (deleteTarget && listQuery.data?.items.length === 1 && filters.page > 1) {
        setFilters((current) => ({ ...current, page: current.page - 1 }));
      }
      setDeleteTarget(null);
      setMessage("试卷已删除");
      await queryClient.invalidateQueries({ queryKey: paperQueryKeys.all });
    },
  });

  const data = listQuery.data;
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / (data?.page_size ?? filters.page_size)));

  return (
    <section aria-labelledby="saved-papers-title" className="mt-10 overflow-hidden rounded-lg border border-border bg-neutral-1 shadow-sm">
      <div className="flex flex-wrap items-end justify-between gap-4 px-5 py-5">
        <div>
          <p className="text-caption-10 font-medium uppercase tracking-[0.2em] text-accent">试卷库</p>
          <h2 id="saved-papers-title" className="mt-1.5 text-title-20 font-medium tracking-[0.01em] text-neutral-10">已保存试卷</h2>
        </div>
        {data ? <p className="text-label-12 text-neutral-7">共 {data.total} 份</p> : null}
      </div>
      <form className="grid gap-3 border-y border-border bg-neutral-2/60 p-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]" onSubmit={(event) => {
        event.preventDefault();
        setMessage("");
        setFilters({ ...draft, page: 1 });
      }} aria-label="试卷列表筛选">
        <div className="grid gap-1.5">
          <Label htmlFor="paper-list-subject" className="text-label-12">学科</Label>
          <Input id="paper-list-subject" className="h-9 bg-neutral-1" value={draft.subject ?? ""} placeholder="全部学科" onChange={(event) => setDraft({ ...draft, subject: event.target.value })} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="paper-list-grade" className="text-label-12">年级</Label>
          <Input id="paper-list-grade" className="h-9 bg-neutral-1" value={draft.grade ?? ""} placeholder="全部年级" onChange={(event) => setDraft({ ...draft, grade: event.target.value })} />
        </div>
        <div className="flex items-end gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={() => {
            const next = { page: 1, page_size: filters.page_size };
            setDraft(next);
            setFilters(next);
          }}><ResetIcon />重置</Button>
          <Button type="submit" size="sm"><MagnifyingGlassIcon />筛选</Button>
        </div>
      </form>
      {message ? <p role="status" className="border-b border-success/25 bg-success-soft px-5 py-3 text-copy-13 text-success">{message}</p> : null}
      {listQuery.isPending ? (
        <div className="grid animate-pulse gap-px bg-border" aria-label="正在读取试卷列表">
          {[0, 1, 2, 3].map((item) => <div key={item} className="h-20 bg-neutral-1" />)}
        </div>
      ) : listQuery.error ? (
        <div className="p-5"><FormError error={listQuery.error} /><Button variant="outline" size="sm" className="mt-3" onClick={() => { void listQuery.refetch(); }}>重新读取</Button></div>
      ) : !data || data.items.length === 0 ? (
        <div className="px-5 py-14 text-center">
          <h3 className="text-copy-13 font-medium text-neutral-10">还没有符合条件的试卷</h3>
          <p className="mt-2 text-copy-13 text-neutral-7">调整筛选条件，或在上方组卷中心创建并保存试卷。</p>
        </div>
      ) : (
        <div aria-busy={listQuery.isFetching}>
          <div className="overflow-x-auto" tabIndex={0} role="region" aria-label="试卷列表，可横向滚动">
            <table className="w-full min-w-[920px] border-collapse text-left text-copy-13">
              <thead className="bg-neutral-2 text-label-12 text-neutral-7">
                <tr className="border-b border-border">
                  <th scope="col" className="px-5 py-3 font-medium">试卷</th>
                  <th scope="col" className="px-3 py-3 font-medium">学科 / 年级</th>
                  <th scope="col" className="px-3 py-3 font-medium">题数</th>
                  <th scope="col" className="px-3 py-3 font-medium">总分</th>
                  <th scope="col" className="px-3 py-3 font-medium">时长</th>
                  <th scope="col" className="px-3 py-3 font-medium">更新时间</th>
                  <th scope="col" className="px-5 py-3 text-right font-medium">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {data.items.map((paper) => (
                  <tr key={paper.id} className="align-middle hover:bg-neutral-2/70">
                    <td className="max-w-sm px-5 py-4"><p className="font-medium text-neutral-10">{paper.title}</p><p className="mt-1 break-all font-mono text-label-12 text-neutral-6">{paper.id}</p></td>
                    <td className="px-3 py-4 text-copy-13 leading-5 text-neutral-8">{paper.subject}<br /><span className="text-neutral-7">{paper.grade}</span></td>
                    <td className="px-3 py-4 font-mono text-copy-13">{paper.question_count}</td>
                    <td className="px-3 py-4 font-mono text-copy-13">{paper.total_score} 分</td>
                    <td className="px-3 py-4 font-mono text-copy-13">{paper.duration_minutes} 分钟</td>
                    <td className="whitespace-nowrap px-3 py-4 text-copy-13 text-neutral-7"><time dateTime={paper.updated_at}>{formatUpdatedAt(paper.updated_at)}</time></td>
                    <td className="px-5 py-4">
                      <div className="flex justify-end gap-1">
                        <Link href={`/teacher/papers/${paper.id}`} className={buttonVariants({ variant: "ghost", size: "sm" })}><Pencil1Icon />编辑</Link>
                        <Link href={`/teacher/papers/${paper.id}/print`} className={buttonVariants({ variant: "ghost", size: "sm" })} aria-label={`预览并打印${paper.title}`}><EyeOpenIcon /><FileTextIcon /><span className="sr-only">打印</span></Link>
                        <Button type="button" variant="ghost" size="sm" className="text-error hover:bg-error-soft hover:text-error" disabled={deleteMutation.isPending && deleteMutation.variables === paper.id} onClick={() => { setMessage(""); deleteMutation.reset(); setDeleteTarget(paper); }}><TrashIcon />删除</Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-4">
            <div className="flex items-center gap-2">
              <Label htmlFor="paper-list-page-size" className="text-label-12">每页</Label>
              <Select id="paper-list-page-size" className="h-8 w-20" value={data.page_size} onChange={(event) => {
                const next = { ...filters, page: 1, page_size: Number(event.target.value) };
                setFilters(next);
                setDraft(next);
              }}><option value={10}>10</option><option value={20}>20</option><option value={50}>50</option></Select>
            </div>
            <div className="flex items-center gap-2">
              <span className="font-mono text-label-12 text-neutral-7">第 {data.page} / {totalPages} 页</span>
              <Button variant="outline" size="sm" disabled={data.page <= 1} onClick={() => setFilters({ ...filters, page: data.page - 1 })}><ChevronLeftIcon />上一页</Button>
              <Button variant="outline" size="sm" disabled={data.page >= totalPages} onClick={() => setFilters({ ...filters, page: data.page + 1 })}>下一页<ChevronRightIcon /></Button>
            </div>
          </div>
        </div>
      )}
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="删除这份试卷？"
        description={deleteTarget ? `将删除“${deleteTarget.title}”及其题目顺序和分值配置，不会删除题库中的原题。` : ""}
        confirmLabel="确认删除"
        isPending={deleteMutation.isPending}
        error={deleteMutation.isError ? deleteMutation.error.message : undefined}
        onCancel={() => { if (!deleteMutation.isPending) { deleteMutation.reset(); setDeleteTarget(null); } }}
        onConfirm={() => { if (deleteTarget) deleteMutation.mutate(deleteTarget.id); }}
      />
    </section>
  );
}
