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
import { HomePaperPreview } from "@/components/home-paper-preview";
import { LoadingScreen } from "@/components/loading-screen";
import { Button, buttonVariants } from "@/components/ui/button";

function GuestHome({ error, retry }: { error: unknown; retry: () => void }) {
  return (
    <div className="paper-texture min-h-dvh overflow-hidden bg-paper">
      <header className="motion-fade border-b border-border">
        <div className="mx-auto flex h-20 w-full max-w-[1440px] items-center justify-between px-5 sm:px-8 lg:px-12">
          <Brand />
          <Link href="/login" className={buttonVariants({ variant: "outline", size: "sm" })}>登录</Link>
        </div>
      </header>
      <main className="mx-auto grid w-full max-w-[1440px] gap-12 px-5 py-12 sm:px-8 sm:py-16 lg:min-h-[calc(100dvh-5rem)] lg:grid-cols-[minmax(0,0.92fr)_minmax(26rem,1.08fr)] lg:items-center lg:gap-14 lg:px-12 lg:py-14 xl:gap-20">
        <section className="max-w-2xl lg:py-10">
          <div className="motion-fade flex items-center gap-4">
            <span className="motion-line block h-px w-12 bg-accent" aria-hidden="true" />
            <p className="text-caption-10 font-medium uppercase tracking-[0.2em] text-accent">学校专属题库与组卷系统</p>
          </div>
          <h1 className="motion-rise mt-7 max-w-xl text-display-36 font-medium leading-[1.14] tracking-[0.01em] text-neutral-10 [--motion-delay:90ms] sm:text-display-48">从真实题目开始，<br />完成每一份校本试卷。</h1>
          <p className="motion-rise mt-7 max-w-xl text-copy-15 leading-8 text-neutral-7 [--motion-delay:180ms]">管理员初始化学校，教师凭学校代码注册。登录后，页面仅展示当前账号与所属学校的真实信息。</p>
          <nav aria-label="账户入口" className="motion-rise mt-9 [--motion-delay:270ms]">
            <div className="flex flex-col gap-3 sm:flex-row">
              <Link href="/login" className={buttonVariants({ size: "lg", className: "group justify-between sm:min-w-44" })}>登录现有账号<ArrowRightIcon className="transition-transform duration-300 group-hover:translate-x-0.5" /></Link>
              <Link href="/register" className={buttonVariants({ variant: "outline", size: "lg", className: "group justify-between sm:min-w-44" })}>注册教师账号<ArrowRightIcon className="transition-transform duration-300 group-hover:translate-x-0.5" /></Link>
            </div>
            <Link href="/init" className="group mt-5 inline-flex items-center gap-2 rounded-sm text-copy-13 font-medium text-neutral-7 outline-none transition-colors hover:text-accent-strong focus-visible:ring-2 focus-visible:ring-accent/35 focus-visible:ring-offset-4 focus-visible:ring-offset-paper">首次使用，初始化学校<ArrowRightIcon className="size-4 transition-transform group-hover:translate-x-0.5" /></Link>
          </nav>
          <p className="motion-rise mt-7 max-w-lg border-t border-border pt-5 text-label-12 leading-6 text-neutral-6 [--motion-delay:360ms]">学校代码由管理员初始化后获得。系统不会在浏览器中读取或保存 JWT。</p>
          {error ? <div className="motion-rise mt-7 max-w-lg [--motion-delay:430ms]"><FormError error={error} /><Button variant="outline" size="sm" className="mt-3" onClick={retry}>重新获取会话</Button></div> : null}
        </section>
        <div className="motion-fade min-w-0 [--motion-delay:200ms] lg:pl-2">
          <HomePaperPreview />
        </div>
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
