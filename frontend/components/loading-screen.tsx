import { Brand } from "@/components/brand";

export function LoadingScreen() {
  return (
    <div className="min-h-dvh bg-background px-6 py-6">
      <Brand />
      <div className="mx-auto mt-28 max-w-5xl animate-pulse" aria-label="正在读取当前用户">
        <div className="h-4 w-28 rounded bg-neutral-4" />
        <div className="mt-5 h-10 w-72 max-w-full rounded bg-neutral-4" />
        <div className="mt-5 h-5 w-full max-w-lg rounded bg-neutral-3" />
        <div className="mt-12 grid gap-6 border-t border-border pt-8 sm:grid-cols-2">
          <div className="h-20 rounded bg-neutral-3" />
          <div className="h-20 rounded bg-neutral-3" />
        </div>
      </div>
    </div>
  );
}
