"use client";

import Link from "next/link";
import { useMutation } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { registerTeacher } from "@/lib/auth";
import { registerSchema, type RegisterFormValues } from "@/lib/schemas";
import { AuthHeading, FormError, FormField } from "@/components/auth-form";
import { Button, buttonVariants } from "@/components/ui/button";

export function RegisterForm() {
  const mutation = useMutation({ mutationFn: registerTeacher });
  const form = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: { schoolCode: "", name: "", subject: "", email: "", password: "" },
  });

  if (mutation.isSuccess) {
    return (
      <div aria-live="polite">
        <p className="mb-3 text-caption-10 font-medium uppercase tracking-[0.2em] text-accent">注册完成</p>
        <h2 className="text-title-28 font-medium tracking-[0.01em] text-neutral-10">教师账号已创建</h2>
        <p className="mt-4 text-copy-13 leading-7 text-neutral-7">现在可以使用注册邮箱和密码登录，进入当前学校的工作区。</p>
        <Link href="/login" className={buttonVariants({ size: "lg", className: "mt-8 w-full" })}>前往登录</Link>
      </div>
    );
  }

  return (
    <div>
      <AuthHeading eyebrow="教师注册" title="加入所在学校" description="向学校管理员获取 8 位学校代码。注册成功后，账号将直接启用。" />
      <form className="grid gap-5" noValidate onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
        <FormError error={mutation.error} />
        <FormField label="学校代码" inputMode="numeric" autoComplete="off" maxLength={8} placeholder="8 位数字" error={form.formState.errors.schoolCode?.message} {...form.register("schoolCode")} />
        <div className="grid gap-5 sm:grid-cols-2">
          <FormField label="姓名" autoComplete="name" placeholder="真实姓名" error={form.formState.errors.name?.message} {...form.register("name")} />
          <FormField label="学科" autoComplete="off" placeholder="例如：数学" error={form.formState.errors.subject?.message} {...form.register("subject")} />
        </div>
        <FormField label="邮箱" type="email" autoComplete="email" placeholder="teacher@school.edu.cn" error={form.formState.errors.email?.message} {...form.register("email")} />
        <FormField label="密码" type="password" autoComplete="new-password" placeholder="至少 10 个字符" hint="10 至 72 个字符" error={form.formState.errors.password?.message} {...form.register("password")} />
        <Button type="submit" size="lg" className="mt-2 w-full" disabled={mutation.isPending}>{mutation.isPending ? "正在注册…" : "创建教师账号"}</Button>
      </form>
      <p className="mt-6 text-copy-13 text-neutral-7">已有账号？<Link href="/login" className="ml-1 font-medium text-accent-strong hover:text-accent hover:underline">直接登录</Link></p>
    </div>
  );
}
