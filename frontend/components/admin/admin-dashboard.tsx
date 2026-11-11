"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowRightIcon } from "@radix-ui/react-icons";
import { getAdminStats, statsQueryKeys } from "@/lib/stats";
import { AppShell } from "@/components/app-shell";
import { RoleAccess } from "@/components/role-access";
import { StatsCards, type StatCardItem } from "@/components/stats-cards";
import { buttonVariants } from "@/components/ui/button";

function AdminStats() {
  const query = useQuery({ queryKey: statsQueryKeys.admin, queryFn: getAdminStats, staleTime: 0 });
  const data = query.data;
  const questionTotal = data ? data.questions.draft + data.questions.published + data.questions.archived : 0;
  const teacherTotal = data ? data.teachers.active + data.teachers.disabled : 0;
  const items: StatCardItem[] | undefined = data ? [
    { label: "正常教师", value: data.teachers.active, detail: `全校共 ${teacherTotal} 位教师` },
    { label: "已禁用教师", value: data.teachers.disabled, detail: "禁用后不能继续使用会话" },
    { label: "全校题目", value: questionTotal, detail: `草稿 ${data.questions.draft} · 已发布 ${data.questions.published} · 已归档 ${data.questions.archived}` },
    { label: "全校试卷", value: data.papers, detail: "当前学校的全部试卷" },
  ] : undefined;

  return (
    <div className="mx-auto max-w-[1400px]">
      <p className="text-xs font-semibold tracking-[0.16em] text-primary">管理员工作区 / 实时数据</p>
      <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">学校总览</h1>
      <p className="mt-2 text-sm leading-6 text-slate-600">统计直接来自管理接口，覆盖教师、题目状态和试卷数量。</p>
      <div className="mt-7"><StatsCards items={items} isPending={query.isPending} error={query.error} onRetry={() => { void query.refetch(); }} /></div>
      <nav aria-label="管理快捷入口" className="mt-7 grid gap-3 border-t border-border pt-5 sm:grid-cols-3">
        <Link href="/admin/teachers" className={buttonVariants({ variant: "outline", className: "justify-between" })}>管理教师<ArrowRightIcon /></Link>
        <Link href="/admin/questions" className={buttonVariants({ variant: "outline", className: "justify-between" })}>审核全校题目<ArrowRightIcon /></Link>
        <Link href="/admin/settings" className={buttonVariants({ variant: "outline", className: "justify-between" })}>学校设置<ArrowRightIcon /></Link>
      </nav>
    </div>
  );
}

export function AdminDashboard() {
  return <RoleAccess role="admin">{(user) => <AppShell user={user}><AdminStats /></AppShell>}</RoleAccess>;
}
