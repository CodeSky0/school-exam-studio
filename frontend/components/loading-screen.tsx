import { Brand } from "@/components/brand";

export function LoadingScreen() {
  return (
    <div className="min-h-dvh bg-paper">
      <header className="border-b border-border px-5 py-5 sm:px-8 lg:px-12">
        <div className="mx-auto max-w-[1440px]"><Brand /></div>
      </header>
      <main className="mx-auto grid w-full max-w-[1440px] gap-12 px-5 py-16 sm:px-8 lg:grid-cols-[0.92fr_1.08fr] lg:items-center lg:px-12 lg:py-24" role="status" aria-live="polite" aria-label="正在读取当前用户">
        <div className="motion-fade">
          <div className="skeleton h-3 w-36 rounded bg-neutral-4" />
          <div className="skeleton mt-6 h-11 w-full max-w-md rounded bg-neutral-4" />
          <div className="skeleton mt-3 h-11 w-4/5 max-w-sm rounded bg-neutral-4" />
          <div className="skeleton mt-7 h-4 w-full max-w-lg rounded bg-neutral-3" />
          <div className="skeleton mt-3 h-4 w-3/4 max-w-md rounded bg-neutral-3" />
          <div className="mt-10 flex flex-col gap-3 sm:flex-row">
            <div className="skeleton h-11 w-full rounded-md bg-accent/25 sm:w-44" />
            <div className="skeleton h-11 w-full rounded-md bg-neutral-3 sm:w-44" />
          </div>
        </div>
        <div className="motion-fade mx-auto w-full max-w-lg px-6 py-8 [--motion-delay:150ms]">
          <div className="aspect-[4/5] rotate-2 rounded-md border border-border bg-neutral-2 p-8">
            <div className="skeleton h-8 w-40 rounded bg-neutral-4" />
            <div className="mt-8 h-px w-full bg-border" />
            <div className="mt-10 space-y-8">
              <div className="skeleton h-4 w-11/12 rounded bg-neutral-3" />
              <div className="skeleton h-4 w-9/12 rounded bg-neutral-3" />
              <div className="skeleton h-4 w-10/12 rounded bg-neutral-3" />
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
