import { ApiError } from "@/lib/api";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { StatusMessage } from "@/components/ui/status-message";

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
    return (
      <div className="grid border-y border-border sm:grid-cols-2 xl:grid-cols-4" role="status" aria-label="正在读取统计数据">
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className={cn("px-1 py-6 sm:px-6", index > 0 && "border-t border-border sm:border-t-0", index % 2 === 1 && "sm:border-l sm:border-border", index < 2 && "sm:border-b sm:border-border xl:border-b-0", index > 0 && "xl:border-l xl:border-border")}>
            <div className="skeleton h-3 w-20 rounded bg-neutral-4" />
            <div className="skeleton mt-4 h-8 w-16 rounded bg-neutral-3" />
            <div className="skeleton mt-4 h-3 w-full max-w-40 rounded bg-neutral-3" />
          </div>
        ))}
      </div>
    );
  }
  if (error) {
    const message = error instanceof ApiError ? error.message : "统计数据读取失败，请稍后重试";
    return <div className="border-y border-border py-5"><StatusMessage message={message} tone="error" /><Button variant="outline" size="sm" className="mt-3" onClick={onRetry}>重新读取</Button></div>;
  }
  if (!items) return null;

  const stagger = ["[--motion-delay:60ms]", "[--motion-delay:140ms]", "[--motion-delay:220ms]", "[--motion-delay:300ms]"];
  return (
    <dl className="grid border-y border-border sm:grid-cols-2 xl:grid-cols-4">
      {items.map((item, index) => (
        <div key={item.label} className={cn("motion-rise min-w-0 px-1 py-6 sm:px-6", stagger[index % stagger.length], index > 0 && "border-t border-border sm:border-t-0", index % 2 === 1 && "sm:border-l sm:border-border", index < 2 && "sm:border-b sm:border-border xl:border-b-0", index > 0 && "xl:border-l xl:border-border")}>
          <dt className="text-label-12 font-medium uppercase tracking-[0.12em] text-neutral-6">{item.label}</dt>
          <dd className="mt-3 font-mono text-display-36 font-medium leading-none tracking-[0.01em] text-neutral-10">{item.value}</dd>
          <dd className="mt-3 text-label-12 leading-5 text-neutral-7">{item.detail}</dd>
        </div>
      ))}
    </dl>
  );
}
