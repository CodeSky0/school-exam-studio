"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowRightIcon } from "@radix-ui/react-icons";
import type { CurrentUser } from "@/lib/schemas";
import { getTeacherStats, statsQueryKeys } from "@/lib/stats";
import { AppShell } from "@/components/app-shell";
import { RoleAccess } from "@/components/role-access";
import { StatsCards, type StatCardItem } from "@/components/stats-cards";
import { buttonVariants } from "@/components/ui/button";

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
      <p className="text-caption-10 font-medium uppercase tracking-[0.2em] text-accent">教师工作区 / 实时数据</p>
      <h1 className="mt-2 text-title-28 font-medium tracking-[0.01em] text-neutral-10">{user.name}的个人总览</h1>
      <p className="mt-2 text-copy-14 leading-7 text-neutral-7">这里只统计当前教师账号的题目与试卷，不包含其他教师的数据。</p>
      <div className="mt-7"><StatsCards items={items} isPending={query.isPending} error={query.error} onRetry={() => { void query.refetch(); }} /></div>
      <nav aria-label="教师快捷入口" className="mt-7 grid gap-3 border-t border-border pt-5 sm:grid-cols-2">
        <Link href="/teacher/questions" className={buttonVariants({ variant: "outline", className: "justify-between" })}>进入教师题库<ArrowRightIcon /></Link>
        <Link href="/teacher/papers" className={buttonVariants({ variant: "outline", className: "justify-between" })}>进入组卷中心<ArrowRightIcon /></Link>
      </nav>
    </div>
  );
}

export function TeacherDashboard() {
  return <RoleAccess role="teacher">{(user) => <AppShell user={user}><TeacherStats user={user} /></AppShell>}</RoleAccess>;
}
