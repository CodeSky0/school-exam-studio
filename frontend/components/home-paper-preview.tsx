import { cn } from "@/lib/utils";

const previewRows = [
  { id: "first", line: "w-11/12", detail: "w-8/12", delay: "[--motion-delay:460ms]" },
  { id: "second", line: "w-9/12", detail: "w-10/12", delay: "[--motion-delay:580ms]" },
  { id: "third", line: "w-10/12", detail: "w-6/12", delay: "[--motion-delay:700ms]" },
];

const answerCells = ["a", "b", "c", "d"];

export function HomePaperPreview() {
  return (
    <div className="group relative mx-auto w-full max-w-[36rem] select-none px-3 py-8 sm:px-8 sm:py-10 lg:py-12" aria-hidden="true">
      <div className="motion-fade absolute inset-x-8 bottom-2 top-14 rotate-3 rounded-lg border border-accent/15 bg-accent-soft transition-transform duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] [--motion-delay:380ms] group-hover:rotate-[4.5deg] sm:inset-x-14" />
      <div className="motion-fade absolute inset-x-5 bottom-7 top-7 -rotate-3 rounded-lg border border-border bg-neutral-2 transition-transform duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] [--motion-delay:280ms] group-hover:-rotate-[4deg] sm:inset-x-10" />
      <div className="motion-rise [--motion-delay:140ms]">
        <div className="animate-float [--motion-delay:1.4s]">
          <div className="relative ml-auto w-[94%] -rotate-1 rounded-md border border-border-strong/70 bg-[#fffdf8] px-6 py-7 shadow-[0_28px_70px_-42px_rgba(77,47,48,0.45)] transition-transform duration-700 ease-[cubic-bezier(0.16,1,0.3,1)] group-hover:rotate-0 sm:px-9 sm:py-9">
            <div className="flex items-start justify-between gap-5 border-b border-border pb-6">
              <div>
                <p className="text-caption-10 font-medium tracking-[0.24em] text-accent">校本题库</p>
                <p className="mt-3 font-serif text-title-28 font-medium tracking-[0.08em] text-neutral-10">校本试卷</p>
              </div>
              <span className="grid size-10 shrink-0 place-items-center border border-accent/35 font-serif text-copy-14 text-accent">题</span>
            </div>
            <div className="mt-6 flex items-center gap-3">
              <span className="h-1.5 w-20 rounded-full bg-neutral-4" />
              <span className="h-1.5 w-12 rounded-full bg-neutral-3" />
              <span className="ml-auto h-1.5 w-16 rounded-full bg-accent/20" />
            </div>
            <div className="mt-8 space-y-7">
              {previewRows.map((row, index) => (
                <div key={row.id} className={cn("motion-rise", row.delay)}>
                  <div className="flex items-center gap-3">
                    <span className="font-mono text-caption-10 text-accent">0{index + 1}</span>
                    <span className={cn("h-2 rounded-full bg-neutral-4", row.line)} />
                  </div>
                  <div className={cn("ml-8 mt-3 h-1.5 rounded-full bg-neutral-3", row.detail)} />
                  <div className="ml-8 mt-4 grid grid-cols-4 gap-2">
                    {answerCells.map((cell) => (
                      <span key={`${row.id}-${cell}`} className="h-5 rounded-sm border border-border bg-paper" />
                    ))}
                  </div>
                </div>
              ))}
            </div>
            <div className="motion-fade mt-8 flex items-center justify-between border-t border-border pt-5 [--motion-delay:820ms]">
              <span className="text-caption-10 tracking-[0.18em] text-neutral-6">作答区域</span>
              <span className="h-px w-20 bg-accent/45" />
            </div>
          </div>
        </div>
      </div>
      <div className="motion-pop absolute bottom-0 left-0 flex items-center gap-3 rounded-md bg-accent px-4 py-3 text-accent-foreground shadow-[0_16px_35px_-24px_rgba(77,47,48,0.65)] [--motion-delay:960ms] sm:left-2">
        <span className="h-px w-6 bg-accent-foreground/60" />
        <span className="text-label-12 font-medium tracking-[0.12em]">规范编排</span>
      </div>
    </div>
  );
}
