import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  eyebrow: string;
  title: string;
  description?: string;
  actions?: ReactNode;
  back?: ReactNode;
  className?: string;
}

export function PageHeader({ eyebrow, title, description, actions, back, className }: PageHeaderProps) {
  return (
    <header className={cn("flex flex-col gap-5 pb-6 sm:flex-row sm:items-end sm:justify-between", className)}>
      <div className="min-w-0">
        {back ? <div className="mb-3">{back}</div> : null}
        <p className="text-caption-10 font-medium uppercase tracking-[0.2em] text-accent">{eyebrow}</p>
        <h1 className="mt-2 text-title-28 font-medium tracking-[0.01em] text-neutral-10">{title}</h1>
        {description ? <p className="mt-2 max-w-2xl text-copy-14 leading-7 text-neutral-7">{description}</p> : null}
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap items-center gap-3">{actions}</div> : null}
    </header>
  );
}
