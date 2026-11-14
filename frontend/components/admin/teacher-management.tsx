"use client";

import { useState } from "react";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckIcon, ChevronLeftIcon, ChevronRightIcon, CopyIcon, MagnifyingGlassIcon, ResetIcon } from "@radix-ui/react-icons";
import { adminQueryKeys, listTeachers, resetTeacherPassword, updateTeacherStatus, type TeacherFilters } from "@/lib/admin";
import type { CurrentUser } from "@/lib/schemas";
import { statsQueryKeys } from "@/lib/stats";
import { cn } from "@/lib/utils";
import { AppShell } from "@/components/app-shell";
import { FormError } from "@/components/auth-form";
import { PageHeader } from "@/components/page-header";
import { RoleAccess } from "@/components/role-access";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { StatusMessage } from "@/components/ui/status-message";

const listVariants: Variants = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { staggerChildren: 0.04 } },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  visible: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -6 },
};

function TeacherFilterBar({ filters, onApply }: { filters: TeacherFilters; onApply: (filters: TeacherFilters) => void }) {
  const [draftState, setDraftState] = useState({ source: filters, value: filters });
  const draft = draftState.source === filters ? draftState.value : filters;

  function setDraft(value: TeacherFilters) {
    setDraftState({ source: filters, value });
  }

  function reset() {
    const next = { page: 1, page_size: filters.page_size };
    setDraft(next);
    onApply(next);
  }

  return (
    <form className="grid gap-3 border-b border-border px-4 py-4 sm:grid-cols-[minmax(0,1fr)_12rem_auto] sm:items-end" onSubmit={(event) => { event.preventDefault(); onApply({ ...draft, page: 1 }); }}>
      <div className="grid gap-1.5"><Label htmlFor="teacher-subject" className="text-label-12">学科</Label><Input id="teacher-subject" className="h-9" value={draft.subject ?? ""} placeholder="输入学科，例如数学" onChange={(event) => setDraft({ ...draft, subject: event.target.value })} /></div>
      <div className="grid gap-1.5"><Label htmlFor="teacher-status" className="text-label-12">账号状态</Label><Select id="teacher-status" className="h-9" value={draft.status ?? ""} onChange={(event) => setDraft({ ...draft, status: (event.target.value || undefined) as TeacherFilters["status"] })}><option value="">全部状态</option><option value="active">正常</option><option value="disabled">已禁用</option></Select></div>
      <div className="flex gap-2"><Button type="button" variant="ghost" size="sm" onClick={reset}><ResetIcon />重置</Button><Button type="submit" size="sm"><MagnifyingGlassIcon />筛选</Button></div>
    </form>
  );
}

function formatDate(value?: string) {
  if (!value) return "未提供";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", timeZone: "Asia/Shanghai" }).format(date);
}

function TeacherStatus({ teacher }: { teacher: CurrentUser }) {
  const isDisabled = teacher.status === "disabled";
  return <span className={cn("inline-flex rounded-full border px-2 py-0.5 text-label-12", isDisabled ? "border-neutral-4 bg-neutral-3 text-neutral-7" : "border-success/25 bg-success-soft text-success")}>{isDisabled ? "已禁用" : "正常"}</span>;
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

  function teacherActions(teacher: CurrentUser) {
    const isDisabled = teacher.status === "disabled";
    return (
      <div className="flex flex-wrap justify-end gap-1">
        <Button variant="ghost" size="sm" disabled={statusMutation.isPending || resetMutation.isPending} onClick={() => { setMessage(null); setStatusTarget(teacher); }}>{isDisabled ? "启用" : "禁用"}</Button>
        <Button variant="ghost" size="sm" disabled={statusMutation.isPending || resetMutation.isPending} onClick={() => { setMessage(null); setResetTarget(teacher); }}>重置密码</Button>
      </div>
    );
  }

  const data = listQuery.data;
  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.page_size)) : 1;

  return (
    <div className="mx-auto max-w-[1400px]">
      <PageHeader eyebrow="管理员工作区 / 账号管理" title="教师管理" description="按学科与状态检索本校教师。禁用和重置密码会立即使该教师的旧会话失效。" />
      <StatusMessage message={message} tone="success" className="mb-5" />
      <section aria-label="教师列表" className="overflow-hidden rounded-lg border border-border bg-neutral-1 shadow-sm">
        <TeacherFilterBar filters={filters} onApply={(next) => { setFilters(next); setMessage(null); }} />
        {listQuery.isPending ? <div className="grid animate-pulse gap-px bg-border" aria-label="正在读取教师列表">{Array.from({ length: 5 }, (_, index) => <div key={index} className="h-16 bg-neutral-1" />)}</div> : null}
        {listQuery.error ? <div className="p-5"><FormError error={listQuery.error} /><Button variant="outline" size="sm" className="mt-3" onClick={() => { void listQuery.refetch(); }}>重新读取</Button></div> : null}
        {data?.items.length === 0 ? <div className="px-5 py-14 text-center"><h2 className="text-copy-13 font-medium text-neutral-10">当前筛选下没有教师</h2><p className="mt-2 text-copy-13 text-neutral-7">调整学科或账号状态后重新查询。</p></div> : null}
        {data && data.items.length > 0 ? <>
          <div className="flex min-h-9 items-center justify-between px-4 text-label-12 text-neutral-7"><span>共 {data.total} 位教师{listQuery.isFetching ? " · 正在更新…" : ""}</span><span>重置密码仅展示一次</span></div>
          <div className="hidden overflow-x-auto border-y border-border md:block" tabIndex={0} role="region" aria-label="教师列表，可横向滚动">
            <table className="w-full min-w-[1080px] border-collapse text-left text-copy-13">
              <thead className="bg-neutral-2 text-label-12 text-neutral-7"><tr><th scope="col" className="px-4 py-3">教师</th><th scope="col" className="px-3 py-3">邮箱</th><th scope="col" className="px-3 py-3">学科</th><th scope="col" className="px-3 py-3">状态</th><th scope="col" className="px-3 py-3">创建时间</th><th scope="col" className="px-4 py-3 text-right">操作</th></tr></thead>
              <motion.tbody className="divide-y divide-border" variants={listVariants} initial="hidden" animate="visible">{data.items.map((teacher) => (
                <motion.tr layout="position" variants={itemVariants} key={teacher.id}>
                  <td className="px-4 py-3"><p className="font-medium text-neutral-10">{teacher.name}</p><p className="mt-1 font-mono text-label-12 text-neutral-6">{teacher.id}</p></td>
                  <td className="px-3 py-3 text-copy-13 text-neutral-8">{teacher.email}</td>
                  <td className="px-3 py-3 text-copy-13">{teacher.subject || "未设置"}</td>
                  <td className="px-3 py-3"><TeacherStatus teacher={teacher} /></td>
                  <td className="px-3 py-3 text-copy-13 text-neutral-7">{formatDate(teacher.created_at)}</td>
                  <td className="px-4 py-3">{teacherActions(teacher)}</td>
                </motion.tr>
              ))}</motion.tbody>
            </table>
          </div>
          <motion.div className="border-y border-border md:hidden" variants={listVariants} initial="hidden" animate="visible" aria-label="教师摘要列表">
            <AnimatePresence initial={false} mode="popLayout">
              {data.items.map((teacher) => (
                <motion.article layout="position" variants={itemVariants} exit="exit" key={teacher.id} className="border-b border-border p-4 last:border-b-0">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0"><h2 className="text-copy-14 font-medium text-neutral-10">{teacher.name}</h2><p className="mt-1 break-all font-mono text-label-12 text-neutral-6">{teacher.id}</p></div>
                    <TeacherStatus teacher={teacher} />
                  </div>
                  <dl className="mt-4 grid gap-3 text-label-12">
                    <div><dt className="text-neutral-6">邮箱</dt><dd className="mt-1 break-all text-copy-13 text-neutral-9">{teacher.email}</dd></div>
                    <div className="grid grid-cols-2 gap-4"><div><dt className="text-neutral-6">学科</dt><dd className="mt-1 text-copy-13 text-neutral-9">{teacher.subject || "未设置"}</dd></div><div><dt className="text-neutral-6">创建时间</dt><dd className="mt-1 text-copy-13 text-neutral-9">{formatDate(teacher.created_at)}</dd></div></div>
                  </dl>
                  <div className="mt-4 border-t border-border pt-3">{teacherActions(teacher)}</div>
                </motion.article>
              ))}
            </AnimatePresence>
          </motion.div>
          <div className="flex flex-wrap items-center justify-between gap-3 px-4 py-4"><div className="flex items-center gap-2"><Label htmlFor="teacher-page-size" className="text-label-12">每页</Label><Select id="teacher-page-size" className="h-8 w-20" value={data.page_size} onChange={(event) => setFilters((current) => ({ ...current, page: 1, page_size: Number(event.target.value) }))}><option value={10}>10</option><option value={20}>20</option><option value={50}>50</option></Select></div><div className="flex items-center gap-3"><span className="font-mono text-label-12 text-neutral-7">第 {data.page} / {totalPages} 页</span><Button variant="outline" size="sm" disabled={data.page <= 1} onClick={() => setFilters((current) => ({ ...current, page: data.page - 1 }))}><ChevronLeftIcon />上一页</Button><Button variant="outline" size="sm" disabled={data.page >= totalPages} onClick={() => setFilters((current) => ({ ...current, page: data.page + 1 }))}>下一页<ChevronRightIcon /></Button></div></div>
        </> : null}
      </section>

      <ConfirmDialog open={Boolean(statusTarget)} title={statusTarget?.status === "disabled" ? "启用这个教师账号？" : "禁用这个教师账号？"} description={statusTarget ? `${statusTarget.name}（${statusTarget.email}）将${statusTarget.status === "disabled" ? "恢复登录和使用教师功能" : "无法继续使用现有会话或登录"}。` : ""} confirmLabel={statusTarget?.status === "disabled" ? "确认启用" : "确认禁用"} confirmVariant={statusTarget?.status === "disabled" ? "default" : "destructive"} isPending={statusMutation.isPending} error={statusMutation.isError ? statusMutation.error.message : undefined} onCancel={() => { if (!statusMutation.isPending) { statusMutation.reset(); setStatusTarget(null); } }} onConfirm={() => { if (statusTarget) statusMutation.mutate({ id: statusTarget.id, status: statusTarget.status === "disabled" ? "active" : "disabled" }); }} />
      <ConfirmDialog open={Boolean(resetTarget)} title="重置这个教师的密码？" description={resetTarget ? `将为${resetTarget.name}生成一次性临时密码，并使其现有会话全部失效。临时密码关闭后无法再次查看。` : ""} confirmLabel="确认重置" isPending={resetMutation.isPending} error={resetMutation.isError ? resetMutation.error.message : undefined} onCancel={() => { if (!resetMutation.isPending) { resetMutation.reset(); setResetTarget(null); } }} onConfirm={() => { if (resetTarget) resetMutation.mutate(resetTarget.id); }} />
      <Dialog open={Boolean(temporary)} title="一次性临时密码" description={temporary ? `请将密码安全地交给${temporary.teacherName}。关闭此对话框后无法再次查看。` : undefined} onClose={closeTemporary} footer={<><Button variant="outline" onClick={closeTemporary}>我已保存，关闭</Button><Button onClick={() => { void copyPassword(); }}>{copied ? <CheckIcon /> : <CopyIcon />}{copied ? "已复制" : "复制密码"}</Button></>}>
        {temporary ? <div className="grid gap-3"><Label htmlFor="temporary-password">临时密码</Label><Input id="temporary-password" readOnly value={temporary.password} onFocus={(event) => event.target.select()} className="font-mono text-copy-14" /><p className="text-label-12 leading-5 text-warning">教师首次登录后应立即在个人中心修改密码。</p><StatusMessage message={copyError} tone="error" /></div> : null}
      </Dialog>
    </div>
  );
}

export function TeacherManagement() {
  return <RoleAccess role="admin">{(user) => <AppShell user={user}><TeacherManagementContent /></AppShell>}</RoleAccess>;
}
