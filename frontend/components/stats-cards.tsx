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
    return <div className="grid animate-pulse gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="正在读取统计数据">{Array.from({ length: 4 }, (_, index) => <div key={index} className="h-32 rounded-xl border border-border bg-white" />)}</div>;
  }
  if (error) return <div className="rounded-xl border border-border bg-white p-5"><FormError error={error} /><Button variant="outline" size="sm" className="mt-3" onClick={onRetry}>重新读取</Button></div>;
  if (!items) return null;

  return (
    <dl className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {items.map((item) => (
        <div key={item.label} className="rounded-xl border border-border bg-white px-5 py-4 shadow-sm">
          <dt className="text-xs font-medium text-slate-500">{item.label}</dt>
          <dd className="mt-2 font-mono text-3xl font-semibold tracking-tight text-slate-950">{item.value}</dd>
          <dd className="mt-2 text-xs leading-5 text-slate-500">{item.detail}</dd>
        </div>
      ))}
    </dl>
  );
}
