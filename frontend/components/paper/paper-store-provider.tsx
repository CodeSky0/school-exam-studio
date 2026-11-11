"use client";

import { createContext, useContext, useEffect, useMemo } from "react";
import { useStore } from "zustand";
import { createPaperStore, type PaperStoreApi, type PaperStoreState } from "@/stores/paper-store";

const PaperStoreContext = createContext<PaperStoreApi | null>(null);

export function PaperStoreProvider({ userId, children }: { userId: string; children: React.ReactNode }) {
  const store = useMemo(() => createPaperStore(userId), [userId]);
  const hasHydrated = useStore(store, (state) => state.hasHydrated);
  useEffect(() => {
    void store.persist.rehydrate();
  }, [store]);
  return (
    <PaperStoreContext.Provider value={store}>
      {hasHydrated ? children : (
        <div className="mx-auto max-w-[1500px] animate-pulse" aria-label="正在恢复当前教师的组卷内容">
          <div className="h-4 w-32 rounded bg-slate-200" />
          <div className="mt-4 h-9 w-72 max-w-full rounded bg-slate-200" />
          <div className="mt-8 h-80 rounded-xl bg-slate-100" />
        </div>
      )}
    </PaperStoreContext.Provider>
  );
}

export function usePaperStore<T>(selector: (state: PaperStoreState) => T): T {
  const store = useContext(PaperStoreContext);
  if (!store) throw new Error("usePaperStore 必须在 PaperStoreProvider 内使用");
  return useStore(store, selector);
}
