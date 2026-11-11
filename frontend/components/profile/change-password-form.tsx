"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { changePassword } from "@/lib/auth";
import { changePasswordSchema } from "@/lib/schemas";
import { FormError, FormField } from "@/components/auth-form";
import { Button } from "@/components/ui/button";

const initialValues = { old_password: "", new_password: "", confirm_password: "" };

export function ChangePasswordForm() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [values, setValues] = useState(initialValues);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const mutation = useMutation({
    mutationFn: changePassword,
    onSuccess: () => {
      queryClient.clear();
      router.replace("/login");
      router.refresh();
    },
  });

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const result = changePasswordSchema.safeParse(values);
    if (!result.success) {
      const nextErrors: Record<string, string> = {};
      result.error.issues.forEach((issue) => {
        const field = String(issue.path[0] ?? "form");
        if (!nextErrors[field]) nextErrors[field] = issue.message;
      });
      setErrors(nextErrors);
      return;
    }
    setErrors({});
    mutation.mutate({ old_password: result.data.old_password, new_password: result.data.new_password });
  }

  return (
    <form onSubmit={submit} noValidate className="grid gap-4">
      <FormField label="原密码" type="password" autoComplete="current-password" value={values.old_password} error={errors.old_password} onChange={(event) => setValues({ ...values, old_password: event.target.value })} />
      <FormField label="新密码" type="password" autoComplete="new-password" value={values.new_password} error={errors.new_password} hint="10 至 72 个字符，修改后需要重新登录。" onChange={(event) => setValues({ ...values, new_password: event.target.value })} />
      <FormField label="确认新密码" type="password" autoComplete="new-password" value={values.confirm_password} error={errors.confirm_password} onChange={(event) => setValues({ ...values, confirm_password: event.target.value })} />
      {mutation.isError ? <FormError error={mutation.error} /> : null}
      <Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? "正在修改…" : "修改密码并重新登录"}</Button>
    </form>
  );
}
