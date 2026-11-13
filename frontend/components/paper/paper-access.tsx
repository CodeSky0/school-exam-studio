"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ApiError } from "@/lib/api";
import type { CurrentUser } from "@/lib/schemas";
import { useCurrentUser } from "@/lib/use-current-user";
import { AppShell } from "@/components/app-shell";
import { FormError } from "@/components/auth-form";
import { LoadingScreen } from "@/components/loading-screen";
import { Button, buttonVariants } from "@/components/ui/button";

export function PaperAccess({ children, standalone = false }: {
  children: (user: CurrentUser) => React.ReactNode;
  standalone?: boolean;
}) {
  const router = useRouter();
  const currentUser = useCurrentUser();
  const isUnauthorized = currentUser.error instanceof ApiError && currentUser.error.status === 401;

  useEffect(() => {
    if (isUnauthorized) router.replace("/login");
  }, [isUnauthorized, router]);

  if (currentUser.isPending || isUnauthorized) return <div className="no-print"><LoadingScreen /></div>;
  if (!currentUser.data) {
    return (
      <main className="no-print mx-auto flex min-h-dvh max-w-lg flex-col justify-center px-6 py-16">
        <h1 className="text-title-24 font-medium text-neutral-10">无法验证教师会话</h1>
        <p className="mt-3 text-copy-14 leading-7 text-neutral-7">确认身份前不会请求或展示试卷数据。</p>
        <div className="mt-6"><FormError error={currentUser.error} /></div>
        <div className="mt-6 flex gap-3">
          <Button variant="outline" onClick={() => { void currentUser.refetch(); }}>重新验证</Button>
          <Link href="/login" className={buttonVariants({ variant: "ghost" })}>返回登录</Link>
        </div>
      </main>
    );
  }

  const user = currentUser.data;
  if (user.role !== "teacher") {
    const denied = (
      <div className="no-print mx-auto max-w-2xl border-l-2 border-warning py-4 pl-5">
        <p className="text-label-12 font-medium tracking-wide text-warning">无权限</p>
        <h1 className="mt-2 text-title-24 font-medium text-neutral-10">组卷中心仅面向学科教师</h1>
        <p className="mt-3 text-copy-14 leading-7 text-neutral-7">当前角色不会请求或展示教师试卷数据。</p>
        <Link href="/" className={buttonVariants({ variant: "outline", className: "mt-6" })}>返回工作区</Link>
      </div>
    );
    return standalone ? <main className="px-6 py-16">{denied}</main> : <AppShell user={user}>{denied}</AppShell>;
  }

  return children(user);
}
