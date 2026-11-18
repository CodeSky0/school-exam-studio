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
        {back ? <div className="motion-fade mb-3">{back}</div> : null}
        <p className="motion-rise flex items-center gap-2.5 text-caption-10 font-medium uppercase tracking-[0.2em] text-neutral-7">
          <span className="size-1.5 rounded-full bg-accent" aria-hidden="true" />
          {eyebrow}
        </p>
        <h1 className="motion-rise mt-2 font-serif text-title-28 font-medium tracking-[0.01em] text-neutral-10 [--motion-delay:80ms]">{title}</h1>
        {description ? <p className="motion-rise mt-2 max-w-2xl text-copy-14 leading-7 text-neutral-7 [--motion-delay:160ms]">{description}</p> : null}
      </div>
      {actions ? <div className="motion-fade flex shrink-0 flex-wrap items-center gap-3 [--motion-delay:220ms]">{actions}</div> : null}
    </header>
  );
}
