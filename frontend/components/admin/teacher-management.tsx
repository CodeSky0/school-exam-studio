"use client";

import { useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckIcon, ChevronLeftIcon, ChevronRightIcon, CopyIcon, MagnifyingGlassIcon, ResetIcon } from "@radix-ui/react-icons";
import { adminQueryKeys, listTeachers, resetTeacherPassword, updateTeacherStatus, type TeacherFilters } from "@/lib/admin";
import type { CurrentUser } from "@/lib/schemas";
import { statsQueryKeys } from "@/lib/stats";
import { cn } from "@/lib/utils";
import { AppShell } from "@/components/app-shell";
import { FormError } from "@/components/auth-form";
import { RoleAccess } from "@/components/role-access";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";

function TeacherFilterBar({ filters, onApply }: { filters: TeacherFilters; onApply: (filters: TeacherFilters) => void }) {
  const [draft, setDraft] = useState(filters);
  return (
    <form className="grid gap-3 border-b border-border px-4 py-4 sm:grid-cols-[minmax(0,1fr)_12rem_auto] sm:items-end" onSubmit={(event) => { event.preventDefault(); onApply({ ...draft, page: 1 }); }}>
      <div className="grid gap-1.5"><Label htmlFor="teacher-subject" className="text-xs">学科</Label><Input id="teacher-subject" className="h-9" value={draft.subject ?? ""} placeholder="输入学科，例如数学" onChange={(event) => setDraft({ ...draft, subject: event.target.value })} /></div>
      <div className="grid gap-1.5"><Label htmlFor="teacher-status" className="text-xs">账号状态</Label><Select id="teacher-status" className="h-9" value={draft.status ?? ""} onChange={(event) => setDraft({ ...draft, status: (event.target.value || undefined) as TeacherFilters["status"] })}><option value="">全部状态</option><option value="active">正常</option><option value="disabled">已禁用</option></Select></div>
      <div className="flex gap-2"><Button type="button" variant="ghost" size="sm" onClick={() => onApply({ page: 1, page_size: filters.page_size })}><ResetIcon />重置</Button><Button type="submit" size="sm"><MagnifyingGlassIcon />筛选</Button></div>
    </form>
  );
}

function formatDate(value?: string) {
  if (!value) return "未提供";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", timeZone: "Asia/Shanghai" }).format(date);
}

function TeacherManagementContent() {
  const queryClient = useQueryClient();
  const [filters, setFilters] = useState<TeacherFilters>({ page: 1, page_size: 10 });
  const [statusTarget, setStatusTarget] = useState<CurrentUser | null>(null);
  const [resetTarget, setResetTarget] = useState<CurrentUser | null>(null);
  const [temporary, setTemporary] = useState<{ teacherName: string; password: string } | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [copyError, setCopyError] = useState<string | null>(null);
  const listQuery = useQuery({ queryKey: adminQueryKeys.teacherList(filters), queryFn: () => listTeachers(filters), placeholderData: keepPreviousData });

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: "active" | "disabled" }) => updateTeacherStatus(id, status),
    onSuccess: async (_, variables) => {
      if (listQuery.data?.items.length === 1 && filters.page > 1 && filters.status && filters.status !== variables.status) setFilters((current) => ({ ...current, page: current.page - 1 }));
      setStatusTarget(null);
      setMessage(variables.status === "active" ? "教师账号已启用" : "教师账号已禁用，旧会话将失效");
      await Promise.all([queryClient.invalidateQueries({ queryKey: adminQueryKeys.teachers }), queryClient.invalidateQueries({ queryKey: statsQueryKeys.admin })]);
    },
  });
  const resetMutation = useMutation({
    mutationFn: resetTeacherPassword,
    onSuccess: (result) => {
      if (resetTarget) setTemporary({ teacherName: resetTarget.name, password: result.temporary_password });
      setResetTarget(null);
    },
  });

  function closeTemporary() {
    setTemporary(null);
    setCopied(false);
    setCopyError(null);
  }

  async function copyPassword() {
    if (!temporary) return;
    setCopied(false);
    setCopyError(null);
    try {
      if (!navigator.clipboard) throw new Error("clipboard unavailable");
      await navigator.clipboard.writeText(temporary.password);
      setCopied(true);
    } catch {
      setCopyError("自动复制失败，请选中密码后手动复制。");
    }
  }

  const data = listQuery.data;
  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.page_size)) : 1;

  return (
    <div className="mx-auto max-w-[1400px]">
      <header className="pb-6"><p className="text-xs font-semibold tracking-[0.16em] text-primary">管理员工作区 / 账号管理</p><h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">教师管理</h1><p className="mt-2 text-sm leading-6 text-slate-600">按学科与状态检索本校教师。禁用和重置密码会立即使该教师的旧会话失效。</p></header>
      {message ? <p role="status" className="mb-5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">{message}</p> : null}
      <section aria-label="教师列表" className="overflow-hidden rounded-xl border border-border bg-white shadow-sm">
        <TeacherFilterBar key={JSON.stringify(filters)} filters={filters} onApply={(next) => { setFilters(next); setMessage(null); }} />
        {listQuery.isPending ? <div className="grid animate-pulse gap-px bg-border" aria-label="正在读取教师列表">{Array.from({ length: 5 }, (_, index) => <div key={index} className="h-16 bg-white" />)}</div> : null}
        {listQuery.error ? <div className="p-5"><FormError error={listQuery.error} /><Button variant="outline" size="sm" className="mt-3" onClick={() => { void listQuery.refetch(); }}>重新读取</Button></div> : null}
        {data?.items.length === 0 ? <div className="px-5 py-14 text-center"><h2 className="text-sm font-semibold text-slate-900">当前筛选下没有教师</h2><p className="mt-2 text-sm text-slate-500">调整学科或账号状态后重新查询。</p></div> : null}
        {data && data.items.length > 0 ? <>
          <div className="flex min-h-9 items-center justify-between px-4 text-xs text-slate-500"><span>共 {data.total} 位教师{listQuery.isFetching ? " · 正在更新…" : ""}</span><span>重置密码仅展示一次</span></div>
          <div className="overflow-x-auto border-y border-border" tabIndex={0} role="region" aria-label="教师列表，可横向滚动">
            <table className="w-full min-w-[1080px] border-collapse text-left text-sm">
              <thead className="bg-slate-50 text-xs text-slate-500"><tr><th className="px-4 py-3">教师</th><th className="px-3 py-3">邮箱</th><th className="px-3 py-3">学科</th><th className="px-3 py-3">状态</th><th className="px-3 py-3">创建时间</th><th className="px-4 py-3 text-right">操作</th></tr></thead>
              <tbody className="divide-y divide-border">{data.items.map((teacher) => {
                const isDisabled = teacher.status === "disabled";
                return <tr key={teacher.id}><td className="px-4 py-3"><p className="font-medium text-slate-900">{teacher.name}</p><p className="mt-1 font-mono text-[11px] text-slate-400">{teacher.id}</p></td><td className="px-3 py-3 text-xs text-slate-700">{teacher.email}</td><td className="px-3 py-3 text-xs">{teacher.subject || "未设置"}</td><td className="px-3 py-3"><span className={cn("inline-flex rounded-full border px-2 py-0.5 text-xs", isDisabled ? "border-slate-200 bg-slate-100 text-slate-600" : "border-emerald-200 bg-emerald-50 text-emerald-800")}>{isDisabled ? "已禁用" : "正常"}</span></td><td className="px-3 py-3 text-xs text-slate-500">{formatDate(teacher.created_at)}</td><td className="px-4 py-3"><div className="flex justify-end gap-1"><Button variant="ghost" size="sm" disabled={statusMutation.isPending || resetMutation.isPending} onClick={() => { setMessage(null); setStatusTarget(teacher); }}>{isDisabled ? "启用" : "禁用"}</Button><Button variant="ghost" size="sm" disabled={statusMutation.isPending || resetMutation.isPending} onClick={() => { setMessage(null); setResetTarget(teacher); }}>重置密码</Button></div></td></tr>;
              })}</tbody>
            </table>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-4"><div className="flex items-center gap-2"><Label htmlFor="teacher-page-size" className="text-xs">每页</Label><Select id="teacher-page-size" className="h-8 w-20" value={data.page_size} onChange={(event) => setFilters((current) => ({ ...current, page: 1, page_size: Number(event.target.value) }))}><option value={10}>10</option><option value={20}>20</option><option value={50}>50</option></Select></div><div className="flex items-center gap-3"><span className="font-mono text-xs text-slate-600">第 {data.page} / {totalPages} 页</span><Button variant="outline" size="sm" disabled={data.page <= 1} onClick={() => setFilters((current) => ({ ...current, page: data.page - 1 }))}><ChevronLeftIcon />上一页</Button><Button variant="outline" size="sm" disabled={data.page >= totalPages} onClick={() => setFilters((current) => ({ ...current, page: data.page + 1 }))}>下一页<ChevronRightIcon /></Button></div></div>
        </> : null}
      </section>

      <ConfirmDialog open={Boolean(statusTarget)} title={statusTarget?.status === "disabled" ? "启用这个教师账号？" : "禁用这个教师账号？"} description={statusTarget ? `${statusTarget.name}（${statusTarget.email}）将${statusTarget.status === "disabled" ? "恢复登录和使用教师功能" : "无法继续使用现有会话或登录"}。` : ""} confirmLabel={statusTarget?.status === "disabled" ? "确认启用" : "确认禁用"} confirmVariant={statusTarget?.status === "disabled" ? "default" : "destructive"} isPending={statusMutation.isPending} error={statusMutation.isError ? statusMutation.error.message : undefined} onCancel={() => { if (!statusMutation.isPending) { statusMutation.reset(); setStatusTarget(null); } }} onConfirm={() => { if (statusTarget) statusMutation.mutate({ id: statusTarget.id, status: statusTarget.status === "disabled" ? "active" : "disabled" }); }} />
      <ConfirmDialog open={Boolean(resetTarget)} title="重置这个教师的密码？" description={resetTarget ? `将为${resetTarget.name}生成一次性临时密码，并使其现有会话全部失效。临时密码关闭后无法再次查看。` : ""} confirmLabel="确认重置" isPending={resetMutation.isPending} error={resetMutation.isError ? resetMutation.error.message : undefined} onCancel={() => { if (!resetMutation.isPending) { resetMutation.reset(); setResetTarget(null); } }} onConfirm={() => { if (resetTarget) resetMutation.mutate(resetTarget.id); }} />
      <Dialog open={Boolean(temporary)} title="一次性临时密码" description={temporary ? `请将密码安全地交给${temporary.teacherName}。关闭此对话框后无法再次查看。` : undefined} onClose={closeTemporary} footer={<><Button variant="outline" onClick={closeTemporary}>我已保存，关闭</Button><Button onClick={() => { void copyPassword(); }}>{copied ? <CheckIcon /> : <CopyIcon />}{copied ? "已复制" : "复制密码"}</Button></>}>
        {temporary ? <div className="grid gap-3"><Label htmlFor="temporary-password">临时密码</Label><Input id="temporary-password" readOnly value={temporary.password} onFocus={(event) => event.target.select()} className="font-mono text-base" /><p className="text-xs leading-5 text-amber-800">教师首次登录后应立即在个人中心修改密码。</p>{copyError ? <p role="alert" className="text-sm text-red-700">{copyError}</p> : null}</div> : null}
      </Dialog>
    </div>
  );
}

export function TeacherManagement() {
  return <RoleAccess role="admin">{(user) => <AppShell user={user}><TeacherManagementContent /></AppShell>}</RoleAccess>;
}
