"use client";

import { useId } from "react";
import { AnimatedDialog } from "@/components/ui/animated-dialog";
import { Button } from "@/components/ui/button";

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  isPending?: boolean;
  error?: string;
  confirmVariant?: "default" | "destructive";
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({ open, title, description, confirmLabel, isPending = false, error, confirmVariant = "destructive", onConfirm, onCancel }: ConfirmDialogProps) {
  const titleId = useId();
  const descriptionId = useId();

  return (
    <AnimatedDialog
      open={open}
      labelledBy={titleId}
      describedBy={descriptionId}
      onClose={onCancel}
      isPending={isPending}
      className="m-auto w-[calc(100%-2rem)] max-w-md rounded-xl border border-border bg-paper p-0 text-neutral-9 shadow-[0_4px_24px_rgba(20,19,18,0.06)] backdrop:bg-neutral-10/25 backdrop:backdrop-blur-sm"
    >
      <div className="p-6">
        <h2 id={titleId} className="text-title-20 font-medium tracking-[0.01em] text-neutral-10">{title}</h2>
        <p id={descriptionId} className="mt-3 text-copy-14 leading-7 text-neutral-7">{description}</p>
        {error ? <p role="alert" className="mt-4 rounded-md border border-error-soft-strong bg-error-soft px-3 py-2 text-copy-13 leading-5 text-error">{error}</p> : null}
        <div className="mt-6 flex justify-end gap-3">
          <Button type="button" variant="outline" disabled={isPending} onClick={onCancel}>取消</Button>
          <Button type="button" variant={confirmVariant} disabled={isPending} onClick={onConfirm}>
            {isPending ? "正在处理…" : confirmLabel}
          </Button>
        </div>
      </div>
    </AnimatedDialog>
  );
}
