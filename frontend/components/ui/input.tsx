import * as React from "react";
import { cn } from "@/lib/utils";

export const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>(
  function Input({ className, type, ...props }, ref) {
    return (
      <input
        type={type}
        className={cn(
          "flex h-10 w-full rounded-md border border-border-strong bg-neutral-1 px-3 py-2 text-copy-14 text-neutral-9 outline-none transition-[border-color,box-shadow] placeholder:text-neutral-6 focus-visible:border-accent focus-visible:ring-3 focus-visible:ring-accent/15 disabled:cursor-not-allowed disabled:bg-neutral-2 disabled:opacity-60 aria-invalid:border-error aria-invalid:ring-error-soft",
          className,
        )}
        ref={ref}
        {...props}
      />
    );
  },
);