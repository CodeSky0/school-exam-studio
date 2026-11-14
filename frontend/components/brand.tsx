import Link from "next/link";
import { cn } from "@/lib/utils";

export function Brand({ inverse = false }: { inverse?: boolean }) {
  return (
    <Link href="/" className={cn("inline-flex items-center gap-3 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-accent/35 focus-visible:ring-offset-2", inverse ? "focus-visible:ring-accent-foreground/60 focus-visible:ring-offset-accent" : "focus-visible:ring-offset-paper")}>
      <span className={cn("grid size-9 place-items-center rounded-md border font-serif text-copy-15 font-medium", inverse ? "border-accent-foreground/35 bg-accent-foreground/10 text-accent-foreground" : "border-accent bg-accent text-accent-foreground")}>题</span>
      <span className="grid gap-0.5">
        <span className={cn("font-serif text-copy-16 font-medium tracking-[0.06em]", inverse ? "text-accent-foreground" : "text-neutral-10")}>校本题库</span>
        <span className={cn("hidden text-caption-10 tracking-[0.12em] sm:block", inverse ? "text-accent-foreground/65" : "text-neutral-6")}>学校专属工作区</span>
      </span>
    </Link>
  );
}
