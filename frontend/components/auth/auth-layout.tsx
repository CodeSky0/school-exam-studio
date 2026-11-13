import { Brand } from "@/components/brand";

export function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-dvh bg-paper lg:grid lg:grid-cols-[minmax(360px,0.9fr)_minmax(0,1.1fr)]">
      <aside className="hidden min-h-dvh flex-col justify-between border-r border-border bg-neutral-2 px-10 py-10 lg:flex xl:px-16 xl:py-12">
        <Brand />
        <div className="max-w-md py-16">
          <span className="mb-6 block h-px w-16 bg-accent" aria-hidden="true" />
          <p className="mb-5 text-caption-10 font-medium uppercase tracking-[0.22em] text-accent">面向学校管理员与学科教师</p>
          <h1 className="text-display-36 font-medium leading-[1.15] tracking-[0.01em] text-neutral-10 xl:text-display-48">同一所学校，<br />同一套可靠题目。</h1>
          <p className="mt-6 max-w-sm text-copy-15 leading-8 text-neutral-7">从学校初始化到教师登录，所有身份资料均由当前部署服务实时提供。</p>
        </div>
        <dl className="grid grid-cols-2 gap-x-8 gap-y-6 border-t border-border pt-7 text-copy-13">
          <div>
            <dt className="text-label-12 text-neutral-6">部署方式</dt>
            <dd className="mt-1.5 font-medium text-neutral-9">单学校独立使用</dd>
          </div>
          <div>
            <dt className="text-label-12 text-neutral-6">会话保护</dt>
            <dd className="mt-1.5 font-medium text-neutral-9">HttpOnly Cookie</dd>
          </div>
        </dl>
      </aside>
      <main className="flex min-h-dvh flex-col px-5 py-6 sm:px-10 lg:px-16 xl:px-24">
        <div className="lg:hidden"><Brand /></div>
        <div className="mx-auto flex w-full max-w-md flex-1 items-center py-10 lg:py-16">
          <div className="w-full">{children}</div>
        </div>
        <p className="text-label-12 leading-5 text-neutral-6">账户信息仅用于当前学校的身份认证与权限识别。</p>
      </main>
    </div>
  );
}