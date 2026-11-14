"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowRightIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { getTeacherStats, statsQueryKeys } from "@/lib/stats";
import { AppShell } from "@/components/app-shell";
import { PageHeader } from "@/components/page-header";
import { RoleAccess } from "@/components/role-access";
import { StatsCards, type StatCardItem } from "@/components/stats-cards";

function TeacherStats({ user }: { user: CurrentUser }) {
  const query = useQuery({ queryKey: statsQueryKeys.teacher, queryFn: getTeacherStats, staleTime: 0 });
  const data = query.data;
  const total = data ? data.questions.draft + data.questions.published + data.questions.archived : 0;
  const items: StatCardItem[] | undefined = data ? [
    { label: "我的题目", value: total, detail: `已发布 ${data.questions.published} · 已归档 ${data.questions.archived}` },
    { label: "待完善草稿", value: data.questions.draft, detail: "草稿可继续编辑或删除" },
    { label: "已发布题目", value: data.questions.published, detail: "可用于当前学校组卷" },
    { label: "我的试卷", value: data.papers, detail: `${user.subject || "未设置学科"}教师账号下的试卷` },
  ] : undefined;

  return (
    <div className="mx-auto max-w-[1400px]">
      <PageHeader eyebrow="教师工作区 / 实时数据" title={`${user.name}的个人总览`} description="这里只统计当前教师账号的题目与试卷，不包含其他教师的数据。" />
      <StatsCards items={items} isPending={query.isPending} error={query.error} onRetry={() => { void query.refetch(); }} />
      <section className="mt-10" aria-labelledby="teacher-shortcuts">
        <h2 id="teacher-shortcuts" className="motion-rise text-copy-14 font-medium text-neutral-9 [--motion-delay:240ms]">教师快捷入口</h2>
        <nav aria-label="教师快捷入口" className="motion-rise mt-3 divide-y divide-border border-y border-border [--motion-delay:320ms]">
          <Link href="/teacher/questions" className="group flex items-center gap-5 py-5 outline-none transition-colors hover:text-accent-strong focus-visible:bg-neutral-2/60">
            <span className="min-w-0 flex-1 sm:flex sm:items-baseline sm:gap-6">
              <span className="block shrink-0 text-copy-15 font-medium text-neutral-10 transition-colors group-hover:text-accent-strong sm:w-32">进入教师题库</span>
              <span className="mt-1 block text-label-12 leading-5 text-neutral-6 sm:mt-0">编辑当前账号的题目与草稿</span>
            </span>
            <ArrowRightIcon className="size-4 shrink-0 text-accent transition-transform group-hover:translate-x-1" />
          </Link>
          <Link href="/teacher/papers" className="group flex items-center gap-5 py-5 outline-none transition-colors hover:text-accent-strong focus-visible:bg-neutral-2/60">
            <span className="min-w-0 flex-1 sm:flex sm:items-baseline sm:gap-6">
              <span className="block shrink-0 text-copy-15 font-medium text-neutral-10 transition-colors group-hover:text-accent-strong sm:w-32">进入组卷中心</span>
              <span className="mt-1 block text-label-12 leading-5 text-neutral-6 sm:mt-0">查看并维护当前账号的试卷</span>
            </span>
            <ArrowRightIcon className="size-4 shrink-0 text-accent transition-transform group-hover:translate-x-1" />
          </Link>
        </nav>
      </section>
    </div>
  );
}

export function TeacherDashboard() {
  return <RoleAccess role="teacher">{(user) => <AppShell user={user}><TeacherStats user={user} /></AppShell>}</RoleAccess>;
}
