"use client";

import * as React from "react";
import { ApiError } from "@/lib/api";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export type FormFieldProps = React.ComponentProps<"input"> & {
  label: string;
  error?: string;
  hint?: string;
};

export const FormField = React.forwardRef<HTMLInputElement, FormFieldProps>(
  function FormField({ id, label, error, hint, ...props }, ref) {
    const generatedId = React.useId();
    const inputId = id ?? generatedId;
    const messageId = `${inputId}-message`;
    return (
      <div className="grid gap-2">
        <Label htmlFor={inputId}>{label}</Label>
        <Input
          id={inputId}
          ref={ref}
          aria-invalid={Boolean(error)}
          aria-describedby={error || hint ? messageId : undefined}
          {...props}
        />
        {error ? <p id={messageId} className="text-copy-13 text-error">{error}</p> : null}
        {hint && !error ? <p id={messageId} className="text-label-12 leading-5 text-neutral-7">{hint}</p> : null}
      </div>
    );
  },
);

export function FormError({ error }: { error: unknown }) {
  if (!error) return null;
  const message = error instanceof ApiError ? error.message : "操作未完成，请稍后重试";
  return (
    <div role="alert" className="rounded-lg border border-error-soft-strong bg-error-soft px-3.5 py-3 text-copy-13 leading-6 text-error">
      {message}
    </div>
  );
}

export function AuthHeading({ eyebrow, title, description }: { eyebrow: string; title: string; description: string }) {
  return (
    <div className="mb-8">
      <p className="mb-3 text-caption-10 font-medium uppercase tracking-[0.2em] text-accent">{eyebrow}</p>
      <h2 className="text-title-28 font-medium tracking-[0.01em] text-neutral-10">{title}</h2>
      <p className="mt-3 text-copy-14 leading-7 text-neutral-7">{description}</p>
    </div>
  );
}

export function FormSkeleton() {
  return (
    <div aria-label="正在读取页面" className="animate-pulse">
      <div className="h-3 w-20 rounded bg-neutral-4" />
      <div className="mt-5 h-9 w-56 rounded bg-neutral-4" />
      <div className="mt-4 h-4 w-full max-w-sm rounded bg-neutral-3" />
      <div className="mt-10 grid gap-6">
        {[0, 1, 2, 3].map((item) => (
          <div key={item}>
            <div className="h-4 w-24 rounded bg-neutral-4" />
            <div className="mt-2 h-11 rounded-lg bg-neutral-3" />
          </div>
        ))}
      </div>
    </div>
  );
}
