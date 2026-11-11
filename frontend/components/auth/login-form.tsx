"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { login } from "@/lib/auth";
import { loginSchema, type LoginFormValues } from "@/lib/schemas";
import { currentUserQueryKey } from "@/lib/use-current-user";
import { AuthHeading, FormError, FormField } from "@/components/auth-form";
import { Button } from "@/components/ui/button";

export function LoginForm() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: login,
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: currentUserQueryKey });
      router.replace("/");
      router.refresh();
    },
  });
  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });

  return (
    <div>
      <AuthHeading eyebrow="账户登录" title="返回学校工作区" description="使用管理员或教师账号登录。会话由服务端通过安全 Cookie 维护。" />
      <form className="grid gap-5" noValidate onSubmit={form.handleSubmit((values) => mutation.mutate(values))}>
        <FormError error={mutation.error} />
        <FormField label="邮箱" type="email" autoComplete="email" placeholder="name@school.edu.cn" error={form.formState.errors.email?.message} {...form.register("email")} />
        <FormField label="密码" type="password" autoComplete="current-password" placeholder="请输入登录密码" hint="密码长度为 10 至 72 个字符" error={form.formState.errors.password?.message} {...form.register("password")} />
        <Button type="submit" size="lg" className="mt-2 w-full" disabled={mutation.isPending}>{mutation.isPending ? "正在登录…" : "登录"}</Button>
      </form>
      <div className="mt-6 flex flex-wrap items-center justify-between gap-3 text-sm">
        <Link href="/register" className="font-medium text-primary hover:underline">注册教师账号</Link>
        <Link href="/init" className="text-slate-500 hover:text-slate-800">首次使用，初始化学校</Link>
      </div>
    </div>
  );
}
