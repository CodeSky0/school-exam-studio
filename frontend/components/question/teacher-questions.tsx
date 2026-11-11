"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ApiError } from "@/lib/api";
import { useCurrentUser } from "@/lib/use-current-user";
import { AppShell } from "@/components/app-shell";
import { FormError } from "@/components/auth-form";
import { LoadingScreen } from "@/components/loading-screen";
import { Button, buttonVariants } from "@/components/ui/button";
import { QuestionBank } from "./question-bank";

export function TeacherQuestions() {
  const router = useRouter();
  const currentUser = useCurrentUser();
  const isUnauthorized = currentUser.error instanceof ApiError && currentUser.error.status === 401;

  useEffect(() => {
    if (isUnauthorized) router.replace("/login");
  }, [isUnauthorized, router]);

  if (currentUser.isPending || isUnauthorized) return <LoadingScreen />;
  if (!currentUser.data) {
    return (
      <main className="mx-auto flex min-h-dvh max-w-lg flex-col justify-center px-6 py-16">
        <h1 className="text-2xl font-semibold text-slate-950">无法验证教师会话</h1>
        <p className="mt-3 text-sm leading-6 text-slate-600">题库不会在未确认身份时展示任何数据。</p>
        <div className="mt-6"><FormError error={currentUser.error} /></div>
        <div className="mt-6 flex gap-3"><Button variant="outline" onClick={() => { void currentUser.refetch(); }}>重新验证</Button><Link href="/login" className={buttonVariants({ variant: "ghost" })}>返回登录</Link></div>
      </main>
    );
  }

  const user = currentUser.data;
  if (user.role !== "teacher") {
    return (
      <AppShell user={user}>
        <div className="mx-auto max-w-2xl border-l-2 border-amber-400 py-4 pl-5">
          <p className="text-xs font-semibold tracking-wide text-amber-800">无权限</p>
          <h1 className="mt-2 text-2xl font-semibold text-slate-950">此页面仅面向学科教师</h1>
          <p className="mt-3 text-sm leading-6 text-slate-600">学校管理员请使用管理员工作区。当前页面不会请求或展示教师题库数据。</p>
          <Link href="/" className={buttonVariants({ variant: "outline", className: "mt-6" })}>返回工作区</Link>
        </div>
      </AppShell>
    );
  }

  return <AppShell user={user}><QuestionBank user={user} /></AppShell>;
}
