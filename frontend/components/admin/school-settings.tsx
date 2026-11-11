"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { adminQueryKeys, getSchool, updateSchoolName, type School } from "@/lib/admin";
import { schoolNameSchema } from "@/lib/schemas";
import { AppShell } from "@/components/app-shell";
import { FormError, FormSkeleton } from "@/components/auth-form";
import { RoleAccess } from "@/components/role-access";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

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

  return (
    <form onSubmit={submit} noValidate className="mt-7 max-w-3xl rounded-xl border border-border bg-white p-5 shadow-sm sm:p-6">
      <div className="grid gap-6">
        <div className="grid gap-2"><Label htmlFor="school-name">学校名称</Label><Input id="school-name" value={name} aria-invalid={Boolean(localError)} aria-describedby="school-name-message" onChange={(event) => { setName(event.target.value); setMessage(null); }} />{localError ? <p id="school-name-message" role="alert" className="text-sm text-red-600">{localError}</p> : <p id="school-name-message" className="text-xs text-slate-500">2 至 200 个字符，保存时仅提交学校名称。</p>}</div>
        <div className="grid gap-2"><Label htmlFor="school-code">学校代码（只读）</Label><Input id="school-code" value={school.school_code} readOnly aria-readonly className="bg-slate-50 font-mono" /><p className="text-xs leading-5 text-slate-500">学校代码用于教师注册，不能在此修改。</p></div>
        <dl className="grid gap-3 border-y border-border py-4 text-xs sm:grid-cols-2"><div><dt className="text-slate-500">学校 ID</dt><dd className="mt-1 break-all font-mono text-slate-700">{school.id}</dd></div><div><dt className="text-slate-500">最近更新</dt><dd className="mt-1 text-slate-700"><time dateTime={school.updated_at}>{school.updated_at}</time></dd></div></dl>
        {mutation.isError ? <FormError error={mutation.error} /> : null}
        {message ? <p role="status" className="rounded-lg border border-emerald-200 bg-emerald-50 px-3.5 py-3 text-sm text-emerald-800">{message}</p> : null}
        <div><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? "正在保存…" : "保存学校名称"}</Button></div>
      </div>
    </form>
  );
}

function SchoolSettingsContent() {
  const query = useQuery({ queryKey: adminQueryKeys.school, queryFn: getSchool, staleTime: 0 });
  return (
    <div className="mx-auto max-w-6xl">
      <p className="text-xs font-semibold tracking-[0.16em] text-primary">管理员工作区 / 学校资料</p><h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">学校设置</h1><p className="mt-2 text-sm leading-6 text-slate-600">查看学校标识并维护学校名称，所有内容来自学校设置接口。</p>
      {query.isPending ? <div className="mt-7 max-w-3xl rounded-xl border border-border bg-white p-6"><FormSkeleton /></div> : null}
      {query.error ? <div className="mt-7 max-w-3xl rounded-xl border border-border bg-white p-5"><FormError error={query.error} /><Button variant="outline" size="sm" className="mt-3" onClick={() => { void query.refetch(); }}>重新读取</Button></div> : null}
      {query.data ? <SchoolForm school={query.data} /> : null}
    </div>
  );
}

export function SchoolSettings() {
  return <RoleAccess role="admin">{(user) => <AppShell user={user}><SchoolSettingsContent /></AppShell>}</RoleAccess>;
}
