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
        {error ? <p id={messageId} className="text-sm text-red-600">{error}</p> : null}
        {hint && !error ? <p id={messageId} className="text-xs leading-5 text-slate-500">{hint}</p> : null}
      </div>
    );
  },
);

export function FormError({ error }: { error: unknown }) {
  if (!error) return null;
  const message = error instanceof ApiError ? error.message : "操作未完成，请稍后重试";
  return (
    <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3.5 py-3 text-sm leading-6 text-red-700">
      {message}
    </div>
  );
}

export function AuthHeading({ eyebrow, title, description }: { eyebrow: string; title: string; description: string }) {
  return (
    <div className="mb-8">
      <p className="mb-3 text-xs font-semibold tracking-[0.18em] text-primary">{eyebrow}</p>
      <h2 className="text-3xl font-semibold tracking-tight text-slate-950">{title}</h2>
      <p className="mt-3 text-sm leading-6 text-slate-600">{description}</p>
    </div>
  );
}

export function FormSkeleton() {
  return (
    <div aria-label="正在读取页面" className="animate-pulse">
      <div className="h-3 w-20 rounded bg-slate-200" />
      <div className="mt-5 h-9 w-56 rounded bg-slate-200" />
      <div className="mt-4 h-4 w-full max-w-sm rounded bg-slate-100" />
      <div className="mt-10 grid gap-6">
        {[0, 1, 2, 3].map((item) => (
          <div key={item}>
            <div className="h-4 w-24 rounded bg-slate-200" />
            <div className="mt-2 h-11 rounded-lg bg-slate-100" />
          </div>
        ))}
      </div>
    </div>
  );
}
