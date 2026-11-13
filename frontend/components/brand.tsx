import Link from "next/link";
import { cn } from "@/lib/utils";

export function Brand({ inverse = false }: { inverse?: boolean }) {
  return (
    <Link href="/" className="inline-flex items-center gap-3 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-accent/35 focus-visible:ring-offset-2 focus-visible:ring-offset-paper">
      <span className="grid size-9 place-items-center rounded-md border border-accent/40 bg-accent-soft font-serif text-copy-15 font-medium text-accent-strong">题</span>
      <span className={cn("font-serif text-copy-16 font-medium tracking-[0.06em]", inverse ? "text-neutral-10" : "text-neutral-10")}>校本题库</span>
    </Link>
  );
}