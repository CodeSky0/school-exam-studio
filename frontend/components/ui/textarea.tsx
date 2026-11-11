import * as React from "react";
import { cn } from "@/lib/utils";

export const Textarea = React.forwardRef<HTMLTextAreaElement, React.ComponentProps<"textarea">>(
  function Textarea({ className, ...props }, ref) {
    return (
      <textarea
        ref={ref}
        className={cn(
          "flex min-h-24 w-full resize-y rounded-lg border border-border-strong bg-white px-3 py-2.5 text-sm leading-6 text-foreground shadow-sm outline-none transition-[border-color,box-shadow] placeholder:text-slate-400 focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-primary/15 disabled:cursor-not-allowed disabled:bg-slate-50 disabled:opacity-60",
          className,
        )}
        {...props}
      />
    );
  },
);
