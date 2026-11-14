import type { CurrentUser } from "@/lib/schemas";
import { roleLabel } from "@/lib/utils";
import { PageHeader } from "@/components/page-header";
import { StatusMessage } from "@/components/ui/status-message";
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
      <PageHeader eyebrow="账户与安全" title="个人中心" description="资料来自当前会话。修改密码成功后，服务端会清除当前会话。" className="pb-7" />
      <StatusMessage message={user.status === "disabled" ? "当前账号已禁用，请联系学校管理员确认账号状态。" : "当前账号状态正常，资料已与服务端会话同步。"} tone={user.status === "disabled" ? "warning" : "success"} className="mb-7" />
      <div className="grid items-start gap-10 xl:grid-cols-[1.15fr_0.85fr] xl:gap-12">
        <section aria-label="当前用户资料">
          <h2 className="text-copy-16 font-medium text-neutral-10">当前用户</h2>
          <dl className="mt-4 divide-y divide-border border-y border-border">
            {details.map((item) => (
              <div key={item.label} className="grid gap-1.5 py-4 sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-4">
                <dt className="text-copy-13 text-neutral-7">{item.label}</dt>
                <dd className={item.mono ? "break-all font-mono text-label-12 leading-5 text-neutral-9" : "break-all text-copy-13 text-neutral-10"}>{item.value}</dd>
              </div>
            ))}
          </dl>
        </section>
        <section aria-labelledby="password-title" className="border-t border-border pt-7 xl:border-l xl:border-t-0 xl:pl-10 xl:pt-0">
          <h2 id="password-title" className="text-copy-16 font-medium text-neutral-10">修改密码</h2>
          <p className="mb-5 mt-1.5 text-label-12 leading-5 text-neutral-7">为其他设备保存的会话可能仍然有效；当前会话将立即退出。</p>
          <ChangePasswordForm />
        </section>
      </div>
    </div>
  );
}
