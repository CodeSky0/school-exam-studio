import Link from "next/link";
import { cn } from "@/lib/utils";

export function Brand({ inverse = false }: { inverse?: boolean }) {
  return (
    <Link href="/" className="inline-flex items-center gap-3 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-primary/40 focus-visible:ring-offset-2">
      <span className="grid size-9 place-items-center rounded-lg bg-primary text-sm font-semibold text-white shadow-sm">题</span>
      <span className={cn("text-base font-semibold tracking-tight", inverse ? "text-white" : "text-slate-950")}>校本题库</span>
    </Link>
  );
}
