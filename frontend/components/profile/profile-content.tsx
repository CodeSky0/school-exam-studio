import type { CurrentUser } from "@/lib/schemas";
import { roleLabel } from "@/lib/utils";
import { ChangePasswordForm } from "./change-password-form";

export function ProfileContent({ user }: { user: CurrentUser }) {
  const details = [
    { label: "姓名", value: user.name },
    { label: "邮箱", value: user.email },
    { label: "角色", value: roleLabel(user.role) },
    { label: "学科", value: user.subject || "未设置" },
    { label: "账号状态", value: user.status === "disabled" ? "已禁用" : "正常" },
    { label: "用户 ID", value: user.id, mono: true },
    { label: "学校 ID", value: user.school_id, mono: true },
  ];

  return (
    <div className="mx-auto max-w-6xl">
      <p className="text-xs font-semibold tracking-[0.16em] text-primary">账户与安全</p>
      <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-950">个人中心</h1>
      <p className="mt-2 text-sm leading-6 text-slate-600">资料来自当前会话。修改密码成功后，服务端会清除当前会话。</p>
      <div className="mt-7 grid items-start gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <section aria-label="当前用户资料" className="overflow-hidden rounded-xl border border-border bg-white shadow-sm">
          <h2 className="border-b border-border px-5 py-4 text-sm font-semibold text-slate-900">当前用户</h2>
          <dl>
            {details.map((item) => (
              <div key={item.label} className="grid gap-1.5 border-b border-border px-5 py-3.5 last:border-b-0 sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-4">
                <dt className="text-sm text-slate-500">{item.label}</dt>
                <dd className={item.mono ? "break-all font-mono text-xs leading-5 text-slate-800" : "break-all text-sm text-slate-900"}>{item.value}</dd>
              </div>
            ))}
          </dl>
        </section>
        <section aria-labelledby="password-title" className="rounded-xl border border-border bg-white p-5 shadow-sm">
          <h2 id="password-title" className="text-sm font-semibold text-slate-900">修改密码</h2>
          <p className="mt-1.5 mb-5 text-xs leading-5 text-slate-500">为其他设备保存的会话可能仍然有效；当前会话将立即退出。</p>
          <ChangePasswordForm />
        </section>
      </div>
    </div>
  );
}
