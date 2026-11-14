"use client";

import { useQuery } from "@tanstack/react-query";
import { adminQueryKeys, listAllTeachers } from "@/lib/admin";
import type { CurrentUser } from "@/lib/schemas";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";

interface AdminCreatorSelectProps {
  id: string;
  user: CurrentUser;
  value?: string;
  onChange: (value?: string) => void;
}

export function AdminCreatorSelect({ id, user, value, onChange }: AdminCreatorSelectProps) {
  const teachersQuery = useQuery({
    queryKey: adminQueryKeys.teacherOptions,
    queryFn: listAllTeachers,
  });
  const teachers = (teachersQuery.data ?? []).filter((teacher) => teacher.id !== user.id);
  const errorMessage = teachersQuery.error instanceof Error ? teachersQuery.error.message : "请稍后重试。";

  return (
    <div className="grid gap-2">
      <Select
        id={id}
        className="h-9"
        value={value ?? ""}
        disabled={teachersQuery.isPending}
        aria-busy={teachersQuery.isPending}
        aria-describedby={teachersQuery.isError ? `${id}-error` : undefined}
        aria-invalid={teachersQuery.isError || undefined}
        onChange={(event) => onChange(event.target.value || undefined)}
      >
        <option value="">{teachersQuery.isPending ? "正在加载教师…" : "全部创建人"}</option>
        <option value={user.id}>{user.name}（当前管理员）</option>
        {teachers.map((teacher) => (
          <option key={teacher.id} value={teacher.id}>{teacher.name}{teacher.subject ? ` · ${teacher.subject}` : ""}</option>
        ))}
      </Select>
      {teachersQuery.isError ? (
        <div className="flex flex-wrap items-center gap-2">
          <p id={`${id}-error`} role="alert" className="min-w-0 flex-1 text-label-12 leading-5 text-error">教师列表加载失败：{errorMessage}</p>
          <Button type="button" variant="outline" size="sm" disabled={teachersQuery.isFetching} onClick={() => { void teachersQuery.refetch(); }}>
            {teachersQuery.isFetching ? "正在重试" : "重试"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
