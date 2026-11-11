import * as React from "react";
import { cn } from "@/lib/utils";

export const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>(
  function Input({ className, type, ...props }, ref) {
    return (
      <input
        type={type}
        className={cn(
          "flex h-11 w-full rounded-lg border border-border-strong bg-white px-3.5 py-2 text-base text-foreground shadow-sm outline-none transition-[border-color,box-shadow] placeholder:text-slate-400 focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-primary/15 disabled:cursor-not-allowed disabled:bg-slate-50 disabled:opacity-60 aria-invalid:border-red-500 aria-invalid:ring-red-100 md:text-sm",
          className,
        )}
        ref={ref}
        {...props}
      />
    );
  },
);
