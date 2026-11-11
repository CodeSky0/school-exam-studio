"use client";

import { useEffect, useRef } from "react";
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
  const dialogRef = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby="confirm-dialog-title"
      aria-describedby="confirm-dialog-description"
      onCancel={(event) => {
        event.preventDefault();
        if (!isPending) onCancel();
      }}
      className="m-auto w-[calc(100%-2rem)] max-w-md rounded-xl border border-border bg-white p-0 text-foreground shadow-xl backdrop:bg-slate-950/40"
    >
      <div className="p-6">
        <h2 id="confirm-dialog-title" className="text-lg font-semibold tracking-tight text-slate-950">{title}</h2>
        <p id="confirm-dialog-description" className="mt-3 text-sm leading-6 text-slate-600">{description}</p>
        {error ? <p role="alert" className="mt-4 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm leading-5 text-red-700">{error}</p> : null}
        <div className="mt-6 flex justify-end gap-3">
          <Button type="button" variant="outline" disabled={isPending} onClick={onCancel}>取消</Button>
          <Button type="button" variant={confirmVariant} disabled={isPending} onClick={onConfirm}>
            {isPending ? "正在处理…" : confirmLabel}
          </Button>
        </div>
      </div>
    </dialog>
  );
}
