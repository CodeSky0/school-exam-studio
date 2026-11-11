import { Brand } from "@/components/brand";

export function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-dvh bg-white lg:grid lg:grid-cols-[minmax(340px,0.85fr)_minmax(0,1.15fr)]">
      <aside className="hidden min-h-dvh flex-col justify-between bg-slate-950 px-10 py-10 text-white lg:flex xl:px-16 xl:py-12">
        <Brand inverse />
        <div className="max-w-md py-16">
          <p className="mb-5 text-xs font-medium tracking-[0.2em] text-blue-300">面向学校管理员与学科教师</p>
          <h1 className="text-4xl font-semibold leading-[1.15] tracking-tight xl:text-5xl">同一所学校，<br />同一套可靠题目。</h1>
          <p className="mt-6 max-w-sm text-base leading-8 text-slate-300">从学校初始化到教师登录，所有身份资料均由当前部署服务实时提供。</p>
        </div>
        <dl className="grid grid-cols-2 gap-x-8 gap-y-6 border-t border-white/15 pt-7 text-sm">
          <div>
            <dt className="text-slate-400">部署方式</dt>
            <dd className="mt-1.5 font-medium text-slate-100">单学校独立使用</dd>
          </div>
          <div>
            <dt className="text-slate-400">会话保护</dt>
            <dd className="mt-1.5 font-medium text-slate-100">HttpOnly Cookie</dd>
          </div>
        </dl>
      </aside>
      <main className="flex min-h-dvh flex-col px-5 py-6 sm:px-10 lg:px-16 xl:px-24">
        <div className="lg:hidden"><Brand /></div>
        <div className="mx-auto flex w-full max-w-md flex-1 items-center py-10 lg:py-16">
          <div className="w-full">{children}</div>
        </div>
        <p className="text-xs leading-5 text-slate-400">账户信息仅用于当前学校的身份认证与权限识别。</p>
      </main>
    </div>
  );
}
