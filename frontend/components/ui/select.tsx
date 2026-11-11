import * as React from "react";
import { cn } from "@/lib/utils";

export const Select = React.forwardRef<HTMLSelectElement, React.ComponentProps<"select">>(
  function Select({ className, ...props }, ref) {
    return (
      <select
        ref={ref}
        className={cn(
          "flex h-10 w-full rounded-lg border border-border-strong bg-white px-3 text-sm text-foreground shadow-sm outline-none transition-[border-color,box-shadow] focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-primary/15 disabled:cursor-not-allowed disabled:bg-slate-50 disabled:opacity-60 aria-invalid:border-red-500",
          className,
        )}
        {...props}
      />
    );
  },
);
