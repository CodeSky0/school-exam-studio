import { FormError } from "@/components/auth-form";
import { Button } from "@/components/ui/button";

export interface StatCardItem {
  label: string;
  value: number;
  detail: string;
}

export function StatsCards({ items, isPending, error, onRetry }: {
  items?: StatCardItem[];
  isPending: boolean;
  error: unknown;
  onRetry: () => void;
}) {
  if (isPending) {
    return <div className="grid animate-pulse gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="正在读取统计数据">{Array.from({ length: 4 }, (_, index) => <div key={index} className="h-32 rounded-lg border border-border bg-neutral-1" />)}</div>;
  }
  if (error) return <div className="rounded-lg border border-border bg-neutral-1 p-5"><FormError error={error} /><Button variant="outline" size="sm" className="mt-3" onClick={onRetry}>重新读取</Button></div>;
  if (!items) return null;

  return (
    <dl className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {items.map((item) => (
        <div key={item.label} className="rounded-lg border border-border bg-neutral-1 px-5 py-4 transition-colors hover:bg-neutral-2/60">
          <dt className="text-label-12 font-medium uppercase tracking-[0.12em] text-neutral-6">{item.label}</dt>
          <dd className="mt-2 font-mono text-title-28 font-medium tracking-[0.01em] text-neutral-10">{item.value}</dd>
          <dd className="mt-2 text-label-12 leading-5 text-neutral-7">{item.detail}</dd>
        </div>
      ))}
    </dl>
  );
}