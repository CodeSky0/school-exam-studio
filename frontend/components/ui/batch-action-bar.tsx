"use client";

import { Button } from "./button";

export function BatchActionBar({ count, onClear, children }: {
  count: number;
  onClear: () => void;
  children?: React.ReactNode;
}) {
  if (count === 0) return null;
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-b border-accent/25 bg-accent-soft/40 px-4 py-2.5" role="region" aria-label="批量操作">
      <span className="text-label-12 font-medium text-accent-strong">已选择 {count} 项</span>
      <div className="flex flex-wrap items-center gap-2">
        {children}
        <Button type="button" variant="ghost" size="sm" onClick={onClear}>取消选择</Button>
      </div>
    </div>
  );
}