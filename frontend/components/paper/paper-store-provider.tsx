"use client";

import { createContext, useContext, useEffect, useMemo } from "react";
import { useStore } from "zustand";
import { createPaperStore, rehydratePaperStore, type PaperStoreApi, type PaperStoreState } from "@/stores/paper-store";

const PaperStoreContext = createContext<PaperStoreApi | null>(null);

export function PaperComposerSkeleton({ label }: { label: string }) {
  return (
    <div className="mx-auto max-w-[1500px] animate-pulse" role="status" aria-label={label}>
      <div aria-hidden="true">
        <div className="pb-6">
          <div className="h-4 w-48 rounded bg-neutral-4" />
          <div className="mt-4 h-9 w-72 max-w-full rounded bg-neutral-4" />
          <div className="mt-3 h-5 w-full max-w-2xl rounded bg-neutral-3" />
        </div>
        <div className="grid items-start gap-6 2xl:grid-cols-[minmax(0,1.4fr)_minmax(22rem,0.8fr)]">
          <div className="overflow-hidden rounded-lg border border-border bg-neutral-1">
            <div className="border-b border-border px-5 py-4">
              <div className="h-6 w-36 rounded bg-neutral-4" />
              <div className="mt-2 h-4 w-64 max-w-full rounded bg-neutral-3" />
            </div>
            <div className="border-b border-border bg-neutral-2/60 p-4">
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
                {[0, 1, 2, 3, 4].map((item) => (
                  <div key={item} className="grid gap-1.5">
                    <div className="h-4 w-12 rounded bg-neutral-4" />
                    <div className="h-9 rounded-md bg-neutral-3" />
                  </div>
                ))}
              </div>
              <div className="mt-3 flex justify-end gap-2">
                <div className="h-8 w-16 rounded-md bg-neutral-3" />
                <div className="h-8 w-24 rounded-md bg-neutral-4" />
              </div>
            </div>
            <div className="divide-y divide-border">
              {[0, 1, 2].map((item) => (
                <div key={item} className="px-5 py-4">
                  <div className="flex items-center gap-2">
                    <div className="h-5 w-16 rounded-full bg-neutral-3" />
                    <div className="h-4 w-28 rounded bg-neutral-3" />
                    <div className="h-4 w-16 rounded bg-neutral-3" />
                  </div>
                  <div className="mt-4 h-4 w-full rounded bg-neutral-3" />
                  <div className="mt-2 h-4 w-4/5 rounded bg-neutral-3" />
                  <div className="mt-4 flex items-center justify-between gap-4">
                    <div className="h-5 w-24 rounded bg-neutral-3" />
                    <div className="h-8 w-24 rounded-md bg-neutral-3" />
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className="grid min-w-0 gap-5">
            <div className="overflow-hidden rounded-lg border border-border bg-neutral-1">
              <div className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
                <div>
                  <div className="h-6 w-24 rounded bg-neutral-4" />
                  <div className="mt-2 h-4 w-56 max-w-full rounded bg-neutral-3" />
                </div>
                <div className="h-10 w-16 rounded bg-neutral-3" />
              </div>
              <div className="divide-y divide-border">
                {[0, 1].map((item) => (
                  <div key={item} className="px-4 py-4">
                    <div className="flex items-center gap-2">
                      <div className="size-8 rounded-md bg-neutral-3" />
                      <div className="h-4 w-8 rounded bg-neutral-3" />
                      <div className="h-5 w-16 rounded-full bg-neutral-3" />
                      <div className="ml-auto h-8 w-24 rounded-md bg-neutral-3" />
                    </div>
                    <div className="mt-3 h-4 w-full rounded bg-neutral-3" />
                    <div className="mt-2 h-4 w-3/5 rounded bg-neutral-3" />
                  </div>
                ))}
              </div>
            </div>
            <div className="rounded-lg border border-border bg-neutral-1 p-5">
              <div className="border-b border-border pb-4">
                <div className="h-6 w-24 rounded bg-neutral-4" />
                <div className="mt-2 h-4 w-64 max-w-full rounded bg-neutral-3" />
              </div>
              <div className="mt-4 grid gap-4">
                {[0, 1, 2].map((item) => (
                  <div key={item} className="grid gap-2">
                    <div className="h-4 w-20 rounded bg-neutral-4" />
                    <div className="h-9 rounded-md bg-neutral-3" />
                  </div>
                ))}
              </div>
              <div className="mt-5 border-t border-border pt-4">
                <div className="h-9 w-full rounded-md bg-neutral-4" />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export function PaperStoreProvider({ userId, children }: { userId: string; children: React.ReactNode }) {
  const store = useMemo(() => createPaperStore(userId), [userId]);
  const hasHydrated = useStore(store, (state) => state.hasHydrated);
  useEffect(() => {
    void rehydratePaperStore(store);
  }, [store]);
  return (
    <PaperStoreContext.Provider value={store}>
      {hasHydrated ? children : <PaperComposerSkeleton label="正在恢复当前教师的组卷内容" />}
    </PaperStoreContext.Provider>
  );
}

export function usePaperStore<T>(selector: (state: PaperStoreState) => T): T {
  const store = useContext(PaperStoreContext);
  if (!store) throw new Error("usePaperStore 必须在 PaperStoreProvider 内使用");
  return useStore(store, selector);
}
