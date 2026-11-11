import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

export const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 rounded-lg text-sm font-medium transition-[background-color,border-color,color,transform,box-shadow] duration-200 outline-none focus-visible:ring-2 focus-visible:ring-primary/35 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 active:translate-y-px [&_svg]:size-4 [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground shadow-sm hover:bg-primary-strong",
        secondary: "bg-primary-soft text-primary-strong hover:bg-primary-soft-strong",
        outline: "border border-border-strong bg-white text-foreground shadow-sm hover:border-primary/40 hover:bg-slate-50",
        ghost: "text-muted-foreground hover:bg-slate-100 hover:text-foreground",
        destructive: "bg-red-600 text-white shadow-sm hover:bg-red-700",
      },
      size: {
        default: "h-10 px-4 py-2",
        sm: "h-8 rounded-md px-3 text-xs",
        lg: "h-12 rounded-xl px-5",
        icon: "size-10 p-0",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

export type ButtonProps = React.ComponentProps<"button"> & VariantProps<typeof buttonVariants> & {
  asChild?: boolean;
};

export function Button({ className, variant, size, asChild = false, ...props }: ButtonProps) {
  const Component = asChild ? Slot : "button";
  return <Component className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
