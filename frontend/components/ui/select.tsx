import * as React from "react";
import { cn } from "@/lib/utils";

export const Select = React.forwardRef<HTMLSelectElement, React.ComponentProps<"select">>(
  function Select({ className, ...props }, ref) {
    return (
      <select
        ref={ref}
        className={cn(
          "flex h-10 w-full rounded-md border border-border-strong bg-neutral-1 px-3 text-copy-14 text-neutral-9 outline-none transition-[border-color,box-shadow] focus-visible:border-accent focus-visible:ring-3 focus-visible:ring-accent/15 disabled:cursor-not-allowed disabled:bg-neutral-2 disabled:opacity-60 aria-invalid:border-error",
          className,
        )}
        {...props}
      />
    );
  },
);