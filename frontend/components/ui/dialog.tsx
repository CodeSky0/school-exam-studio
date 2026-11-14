"use client";

import { useId } from "react";
import { Cross2Icon } from "@radix-ui/react-icons";
import { cn } from "@/lib/utils";
import { AnimatedDialog } from "./animated-dialog";
import { Button } from "./button";

export function Dialog({ open, title, description, children, footer, onClose, isPending = false, className }: {
  open: boolean;
  title: string;
  description?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  onClose: () => void;
  isPending?: boolean;
  className?: string;
}) {
  const titleId = useId();
  const descriptionId = useId();

  return (
    <AnimatedDialog
      open={open}
      labelledBy={titleId}
      describedBy={description ? descriptionId : undefined}
      onClose={onClose}
      isPending={isPending}
      className={cn("m-auto flex max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg flex-col overflow-hidden rounded-xl border border-border bg-paper p-0 text-neutral-9 shadow-xl backdrop:bg-neutral-10/25 backdrop:backdrop-blur-sm", className)}
    >
      <header className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
        <div>
          <h2 id={titleId} className="text-title-20 font-medium tracking-[0.01em] text-neutral-10">{title}</h2>
          {description ? <p id={descriptionId} className="mt-1.5 text-copy-14 leading-7 text-neutral-7">{description}</p> : null}
        </div>
        <Button type="button" variant="ghost" size="icon" className="-mr-2 -mt-2 size-8" aria-label="关闭对话框" disabled={isPending} onClick={onClose}><Cross2Icon /></Button>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto p-5">{children}</div>
      {footer ? <footer className="flex flex-wrap justify-end gap-3 border-t border-border bg-neutral-2/60 px-5 py-4">{footer}</footer> : null}
    </AnimatedDialog>
  );
}
