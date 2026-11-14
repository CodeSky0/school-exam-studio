"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { adminQueryKeys, getSchool, updateSchoolName, type School } from "@/lib/admin";
import { ApiError } from "@/lib/api";
import { schoolNameSchema } from "@/lib/schemas";
import { AppShell } from "@/components/app-shell";
import { FormSkeleton } from "@/components/auth-form";
import { PageHeader } from "@/components/page-header";
import { RoleAccess } from "@/components/role-access";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { StatusMessage } from "@/components/ui/status-message";

function SchoolForm({ school }: { school: School }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(school.name);
  const [localError, setLocalError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const mutation = useMutation({
    mutationFn: updateSchoolName,
    onSuccess: (updated) => {
      queryClient.setQueryData(adminQueryKeys.school, updated);
      setName(updated.name);
      setMessage("学校名称已保存");
    },
  });

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const result = schoolNameSchema.safeParse(name);
    if (!result.success) {
      setLocalError(result.error.issues[0]?.message ?? "请输入有效学校名称");
      setMessage(null);
      return;
    }
    setLocalError(null);
    setMessage(null);
    mutation.mutate(result.data);
  }

  const mutationMessage = mutation.error instanceof ApiError ? mutation.error.message : "学校名称保存失败，请稍后重试";

  return (
    <form onSubmit={submit} noValidate className="max-w-4xl border-t border-border pt-7">
      <div className="grid gap-7">
        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label htmlFor="school-name" className="pt-2.5">学校名称</Label>
          <div className="grid gap-2">
            <Input id="school-name" value={name} aria-invalid={Boolean(localError)} aria-describedby="school-name-message" className="h-11 bg-[#fffdf8]" onChange={(event) => { setName(event.target.value); setMessage(null); }} />
            {localError ? <p id="school-name-message" role="alert" className="text-copy-13 text-error">{localError}</p> : <p id="school-name-message" className="text-label-12 text-neutral-7">2 至 200 个字符，保存时仅提交学校名称。</p>}
          </div>
        </div>
        <div className="grid gap-2 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-6">
          <Label htmlFor="school-code" className="pt-2.5">学校代码（只读）</Label>
          <div className="grid gap-2">
            <Input id="school-code" value={school.school_code} readOnly aria-readonly className="h-11 border-accent/20 bg-accent-soft font-mono tracking-[0.16em]" />
            <p className="text-label-12 leading-5 text-neutral-7">学校代码用于教师注册，不能在此修改。</p>
          </div>
        </div>
        <dl className="grid gap-4 border-y border-border py-5 text-label-12 sm:ml-40 sm:grid-cols-2 sm:pl-6">
          <div><dt className="text-neutral-7">学校 ID</dt><dd className="mt-1.5 break-all font-mono text-neutral-8">{school.id}</dd></div>
          <div><dt className="text-neutral-7">最近更新</dt><dd className="mt-1.5 break-all text-neutral-8"><time dateTime={school.updated_at}>{school.updated_at}</time></dd></div>
        </dl>
        <div className="grid gap-3 sm:ml-40 sm:pl-6">
          {mutation.isError ? <StatusMessage message={mutationMessage} tone="error" /> : null}
          <StatusMessage message={message} tone="success" />
          <div><Button type="submit" size="lg" disabled={mutation.isPending}>{mutation.isPending ? "正在保存…" : "保存学校名称"}</Button></div>
        </div>
      </div>
    </form>
  );
}

function SchoolSettingsContent() {
  const query = useQuery({ queryKey: adminQueryKeys.school, queryFn: getSchool, staleTime: 0 });
  const queryMessage = query.error instanceof ApiError ? query.error.message : "学校资料读取失败，请稍后重试";
  return (
    <div className="mx-auto max-w-6xl">
      <PageHeader eyebrow="管理员工作区 / 学校资料" title="学校设置" description="查看学校标识并维护学校名称，所有内容来自学校设置接口。" className="pb-7" />
      {query.isPending ? <div className="max-w-3xl border-t border-border py-8"><FormSkeleton /></div> : null}
      {query.error ? <div className="max-w-3xl border-t border-border py-6"><StatusMessage message={queryMessage} tone="error" /><Button variant="outline" size="sm" className="mt-3" onClick={() => { void query.refetch(); }}>重新读取</Button></div> : null}
      {query.data ? <SchoolForm school={query.data} /> : null}
    </div>
  );
}

export function SchoolSettings() {
  return <RoleAccess role="admin">{(user) => <AppShell user={user}><SchoolSettingsContent /></AppShell>}</RoleAccess>;
}
