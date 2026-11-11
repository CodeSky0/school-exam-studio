"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowRightIcon } from "@radix-ui/react-icons";
import { ApiError } from "@/lib/api";
import type { CurrentUser } from "@/lib/schemas";
import { useCurrentUser } from "@/lib/use-current-user";
import { Brand } from "@/components/brand";
import { FormError } from "@/components/auth-form";
import { LoadingScreen } from "@/components/loading-screen";
import { Button, buttonVariants } from "@/components/ui/button";

function GuestHome({ error, retry }: { error: unknown; retry: () => void }) {
  return (
    <div className="min-h-dvh bg-white">
      <header className="flex h-16 items-center justify-between border-b border-border px-5 sm:px-8 lg:px-12">
        <Brand />
        <Link href="/login" className={buttonVariants({ variant: "outline", size: "sm" })}>登录</Link>
      </header>
      <main className="mx-auto grid w-full max-w-7xl gap-12 px-5 py-16 sm:px-8 lg:grid-cols-[1.35fr_0.65fr] lg:px-12 lg:py-24">
        <section className="max-w-2xl">
          <p className="text-xs font-semibold tracking-[0.18em] text-primary">学校专属题库与组卷系统</p>
          <h1 className="mt-5 text-4xl font-semibold leading-tight tracking-tight text-slate-950 sm:text-5xl">从真实题目开始，<br />完成每一份校本试卷。</h1>
          <p className="mt-6 max-w-xl text-base leading-8 text-slate-600">管理员初始化学校，教师凭学校代码注册。登录后，页面仅展示当前账号与所属学校的真实信息。</p>
          {error ? <div className="mt-8 max-w-lg"><FormError error={error} /><Button variant="outline" size="sm" className="mt-3" onClick={retry}>重新获取会话</Button></div> : null}
        </section>
        <aside className="border-t border-border pt-7 lg:border-l lg:border-t-0 lg:pl-10 lg:pt-0" aria-label="账户入口">
          <h2 className="text-sm font-semibold text-slate-950">选择访问方式</h2>
          <div className="mt-5 grid gap-3">
            <Link href="/login" className={buttonVariants({ size: "lg", className: "w-full justify-between" })}>登录现有账号<ArrowRightIcon /></Link>
            <Link href="/register" className={buttonVariants({ variant: "outline", size: "lg", className: "w-full justify-between" })}>注册教师账号<ArrowRightIcon /></Link>
            <Link href="/init" className={buttonVariants({ variant: "ghost", size: "lg", className: "w-full justify-between" })}>首次使用，初始化学校<ArrowRightIcon /></Link>
          </div>
          <p className="mt-5 text-xs leading-6 text-slate-500">学校代码由管理员初始化后获得。系统不会在浏览器中读取或保存 JWT。</p>
        </aside>
      </main>
    </div>
  );
}

function AuthenticatedHome({ user }: { user: CurrentUser }) {
  const router = useRouter();
  useEffect(() => {
    router.replace(user.role === "admin" ? "/admin/dashboard" : "/teacher/dashboard");
  }, [router, user.role]);
  return <LoadingScreen />;
}

export function Home() {
  const currentUser = useCurrentUser();
  if (currentUser.isPending) return <LoadingScreen />;
  if (currentUser.data) return <AuthenticatedHome user={currentUser.data} />;
  const isUnauthorized = currentUser.error instanceof ApiError && currentUser.error.status === 401;
  return <GuestHome error={isUnauthorized ? null : currentUser.error} retry={() => { void currentUser.refetch(); }} />;
}
