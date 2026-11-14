"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowRightIcon } from "@radix-ui/react-icons";
import { getAdminStats, statsQueryKeys } from "@/lib/stats";
import { AppShell } from "@/components/app-shell";
import { PageHeader } from "@/components/page-header";
import { RoleAccess } from "@/components/role-access";
import { StatsCards, type StatCardItem } from "@/components/stats-cards";

const managementLinks = [
  { href: "/admin/teachers", label: "管理教师", description: "查看教师状态与账号权限" },
  { href: "/admin/questions", label: "审核全校题目", description: "检查题目状态与内容" },
  { href: "/admin/settings", label: "学校设置", description: "维护学校名称与标识" },
];

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
      <PageHeader eyebrow="管理员工作区 / 实时数据" title="学校总览" description="统计直接来自管理接口，覆盖教师、题目状态和试卷数量。" />
      <StatsCards items={items} isPending={query.isPending} error={query.error} onRetry={() => { void query.refetch(); }} />
      <section className="mt-10" aria-labelledby="admin-shortcuts">
        <h2 id="admin-shortcuts" className="text-copy-14 font-medium text-neutral-9">管理快捷入口</h2>
        <nav aria-label="管理快捷入口" className="mt-3 divide-y divide-border border-y border-border">
          {managementLinks.map((link) => (
            <Link key={link.href} href={link.href} className="group flex items-center gap-5 py-5 outline-none transition-colors hover:text-accent-strong focus-visible:bg-neutral-2/60">
              <span className="min-w-0 flex-1 sm:flex sm:items-baseline sm:gap-6">
                <span className="block shrink-0 text-copy-15 font-medium text-neutral-10 transition-colors group-hover:text-accent-strong sm:w-32">{link.label}</span>
                <span className="mt-1 block text-label-12 leading-5 text-neutral-6 sm:mt-0">{link.description}</span>
              </span>
              <ArrowRightIcon className="size-4 shrink-0 text-accent transition-transform group-hover:translate-x-1" />
            </Link>
          ))}
        </nav>
      </section>
    </div>
  );
}

export function AdminDashboard() {
  return <RoleAccess role="admin">{(user) => <AppShell user={user}><AdminStats /></AppShell>}</RoleAccess>;
}
