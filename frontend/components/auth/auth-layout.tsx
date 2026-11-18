import { Brand } from "@/components/brand";

export function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-dvh bg-paper lg:grid lg:grid-cols-[minmax(24rem,0.88fr)_minmax(0,1.12fr)]">
      <aside className="relative hidden min-h-dvh flex-col justify-between overflow-hidden border-r border-border bg-neutral-2 px-10 py-10 lg:flex xl:px-16 xl:py-12">
        <div className="pointer-events-none absolute -right-44 -top-44 size-[30rem] rounded-full border border-border" aria-hidden="true" />
        <div className="pointer-events-none absolute -right-24 top-1/3 size-80 rounded-full border border-border/70" aria-hidden="true" />
        <div className="animate-drift pointer-events-none absolute -bottom-48 -left-24 size-[26rem] rounded-full border border-border/60" aria-hidden="true" />
        <div className="motion-fade relative"><Brand /></div>
        <div className="relative max-w-md py-16">
          <span className="motion-line mb-6 block h-px w-16 bg-accent" aria-hidden="true" />
          <p className="motion-rise mb-5 text-caption-10 font-medium uppercase tracking-[0.22em] text-neutral-7 [--motion-delay:120ms]">面向学校管理员与学科教师</p>
          <p className="motion-rise font-serif text-title-28 font-medium leading-[1.15] tracking-[0.01em] text-neutral-10 [--motion-delay:220ms] xl:text-display-36">同一所学校，<br />同一套可靠题目。</p>
          <p className="motion-rise mt-6 max-w-sm text-copy-15 leading-8 text-neutral-7 [--motion-delay:330ms]">从学校初始化到教师登录，所有身份资料均由当前部署服务实时提供。</p>
        </div>
        <dl className="motion-fade relative grid grid-cols-2 gap-x-8 gap-y-6 border-t border-border pt-7 text-copy-13 [--motion-delay:460ms]">
          <div>
            <dt className="text-label-12 text-neutral-7">部署方式</dt>
            <dd className="mt-1.5 font-medium text-neutral-9">单学校独立使用</dd>
          </div>
          <div>
            <dt className="text-label-12 text-neutral-7">会话保护</dt>
            <dd className="mt-1.5 font-medium text-neutral-9">HttpOnly Cookie</dd>
          </div>
        </dl>
      </aside>
      <main className="paper-texture flex min-h-dvh flex-col px-5 py-6 sm:px-10 lg:px-16 xl:px-24">
        <div className="motion-fade lg:hidden"><Brand /></div>
        <div className="mx-auto flex w-full max-w-md flex-1 items-center py-10 lg:py-16">
          <div className="motion-rise w-full [--motion-delay:160ms]">{children}</div>
        </div>
        <p className="motion-fade border-t border-border pt-5 text-label-12 leading-5 text-neutral-6 [--motion-delay:380ms]">账户信息仅用于当前学校的身份认证与权限识别。</p>
      </main>
    </div>
  );
}
