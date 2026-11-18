"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { AnimatePresence, motion } from "framer-motion";
import { ArrowRightIcon, CheckCircledIcon, CheckIcon, CopyIcon } from "@radix-ui/react-icons";
import { getInitStatus, initializeSchool } from "@/lib/auth";
import { initSchema, type InitFormValues } from "@/lib/schemas";
import { AuthHeading, FormError, FormField, FormSkeleton } from "@/components/auth-form";
import { Button, buttonVariants } from "@/components/ui/button";

export function InitPanel() {
  const router = useRouter();
  const [copyState, setCopyState] = useState<"idle" | "copied" | "error">("idle");
  const status = useQuery({ queryKey: ["init", "status"], queryFn: getInitStatus });
  const mutation = useMutation({ mutationFn: initializeSchool });
  const form = useForm<InitFormValues>({
    resolver: zodResolver(initSchema),
    defaultValues: { schoolName: "", adminName: "", email: "", password: "" },
  });

  useEffect(() => {
    if (status.data?.initialized) router.replace("/login");
  }, [router, status.data]);

  async function copySchoolCode(code: string) {
    try {
      await navigator.clipboard.writeText(code);
      setCopyState("copied");
    } catch {
      setCopyState("error");
    }
  }

  if (status.isPending || status.data?.initialized) return <FormSkeleton />;

  if (status.isError) {
    return (
      <div>
        <AuthHeading eyebrow="学校初始化" title="暂时无法确认状态" description="初始化前需要确认当前部署是否已创建学校。" />
        <FormError error={status.error} />
        <Button className="mt-5" variant="outline" onClick={() => { void status.refetch(); }}>重新检查</Button>
      </div>
    );
  }

  const school = mutation.data;

  return (
    <AnimatePresence mode="wait" initial={false}>
      {school ? (
        <motion.div key="success" aria-live="polite" initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -8 }} transition={{ duration: 0.24 }}>
          <span className="grid size-12 place-items-center rounded-full border border-success/25 bg-success-soft text-success"><CheckCircledIcon className="size-6" /></span>
          <p className="mb-3 mt-7 flex items-center gap-2.5 text-caption-10 font-medium uppercase tracking-[0.2em] text-neutral-7">
            <span className="size-1.5 rounded-full bg-accent" aria-hidden="true" />
            初始化完成
          </p>
          <h1 className="font-serif text-display-36 font-medium tracking-[0.01em] text-neutral-10">请保存学校代码</h1>
          <p className="mt-3 text-copy-14 leading-7 text-neutral-7">教师注册时需要输入此代码。本次结果仅在当前页面展示一次，离开或刷新前请妥善保存。</p>
          <div className="mt-8 border-y border-border bg-neutral-2 px-5 py-6">
            <p className="text-label-12 font-medium text-neutral-7">8 位学校代码</p>
            <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
              <output className="font-mono text-title-28 font-medium tracking-[0.22em] text-neutral-10">{school.school_code}</output>
              <Button variant="outline" size="sm" className="bg-paper" onClick={() => { void copySchoolCode(school.school_code); }}>
                {copyState === "copied" ? <CheckIcon /> : <CopyIcon />}
                {copyState === "copied" ? "已复制" : "复制代码"}
              </Button>
            </div>
            {copyState === "error" ? <p className="mt-3 text-label-12 text-error">复制失败，请手动选择并复制代码。</p> : null}
          </div>
          <Link href="/login" className={buttonVariants({ size: "lg", className: "mt-8 w-full justify-between" })}>使用管理员账号登录<ArrowRightIcon /></Link>
        </motion.div>
      ) : (
        <motion.div key="form" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0, y: -8 }} transition={{ duration: 0.18 }}>
          <AuthHeading eyebrow="首次使用" title="初始化学校" description="创建学校与首位管理员。初始化仅可成功一次，请使用学校管理员的常用邮箱。" />
          <form className="grid gap-5" noValidate onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
            <FormError error={mutation.error} />
            <FormField label="学校名称" autoComplete="organization" placeholder="例如：实验中学" error={form.formState.errors.schoolName?.message} {...form.register("schoolName")} />
            <FormField label="管理员姓名" autoComplete="name" placeholder="请输入真实姓名" hint="2 至 50 个字符" error={form.formState.errors.adminName?.message} {...form.register("adminName")} />
            <FormField label="管理员邮箱" type="email" autoComplete="email" placeholder="admin@school.edu.cn" error={form.formState.errors.email?.message} {...form.register("email")} />
            <FormField label="登录密码" type="password" autoComplete="new-password" placeholder="至少 10 个字符" hint="10 至 72 个字符，请使用独立的高强度密码" error={form.formState.errors.password?.message} {...form.register("password")} />
            <Button type="submit" size="lg" className="mt-2 w-full" disabled={mutation.isPending}>{mutation.isPending ? "正在初始化…" : "创建学校与管理员"}</Button>
          </form>
          <p className="mt-7 border-t border-border pt-6 text-copy-13 text-neutral-7">已完成初始化？<Link href="/login" className="ml-1 font-medium text-accent-strong hover:text-accent hover:underline">前往登录</Link></p>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
