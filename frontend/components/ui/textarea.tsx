import * as React from "react";
import { cn } from "@/lib/utils";

export const Textarea = React.forwardRef<HTMLTextAreaElement, React.ComponentProps<"textarea">>(
  function Textarea({ className, ...props }, ref) {
    return (
      <textarea
        ref={ref}
        className={cn(
          "flex min-h-24 w-full resize-y rounded-md border border-border-strong bg-neutral-1 px-3 py-2.5 text-copy-14 leading-6 text-neutral-9 outline-none transition-[border-color,box-shadow] placeholder:text-neutral-6 focus-visible:border-accent focus-visible:ring-3 focus-visible:ring-accent/15 disabled:cursor-not-allowed disabled:bg-neutral-2 disabled:opacity-60",
          className,
        )}
        {...props}
      />
    );
  },
);