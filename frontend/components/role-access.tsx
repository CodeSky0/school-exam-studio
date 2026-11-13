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

export function RoleAccess({ role, children }: {
  role?: CurrentUser["role"];
  children: (user: CurrentUser) => React.ReactNode;
}) {
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
        <h1 className="text-title-24 font-medium text-neutral-10">无法验证当前会话</h1>
        <p className="mt-3 text-copy-14 leading-7 text-neutral-7">确认身份前不会请求或展示此页面的业务数据。</p>
        <div className="mt-6"><FormError error={currentUser.error} /></div>
        <div className="mt-6 flex gap-3"><Button variant="outline" onClick={() => { void currentUser.refetch(); }}>重新验证</Button><Link href="/login" className={buttonVariants({ variant: "ghost" })}>返回登录</Link></div>
      </main>
    );
  }

  const user = currentUser.data;
  if (role && user.role !== role) {
    const requiredLabel = role === "admin" ? "学校管理员" : "学科教师";
    return (
      <AppShell user={user}>
        <div role="alert" className="mx-auto max-w-2xl border-l-2 border-warning py-4 pl-5">
          <p className="font-mono text-label-12 font-medium tracking-wide text-warning">403 · 无权限</p>
          <h1 className="mt-2 text-title-24 font-medium text-neutral-10">此页面仅面向{requiredLabel}</h1>
          <p className="mt-3 text-copy-14 leading-7 text-neutral-7">当前角色不会请求或展示此页面的业务数据，请通过导航进入可用工作区。</p>
          <Link href={user.role === "admin" ? "/admin/dashboard" : "/teacher/dashboard"} className={buttonVariants({ variant: "outline", className: "mt-6" })}>返回我的工作区</Link>
        </div>
      </AppShell>
    );
  }

  return children(user);
}
