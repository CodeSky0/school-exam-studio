import { create } from "zustand";
import { createJSONStorage, persist, type StateStorage } from "zustand/middleware";
import type { Question } from "@/lib/question-schemas";
import type { Paper } from "@/lib/paper-schemas";
import { resolveQuestions } from "@/lib/questions";

const DEFAULT_SCORE = "5.00";
const DEFAULT_DURATION = "90";
const SCORE_INPUT_PATTERN = /^\d{0,6}(?:\.\d{0,2})?$/;

export interface PaperCartItem {
  question: Question;
  assignedScore: string;
}

export interface PaperMetadata {
  title: string;
  subject: string;
  grade: string;
  durationMinutes: string;
}

export interface PaperStoreState extends PaperMetadata {
  paperId: string | null;
  items: PaperCartItem[];
  hasHydrated: boolean;
  isResolving: boolean;
  resolutionError: string | null;
  removedInvalidCount: number;
  addQuestion: (question: Question) => boolean;
  removeQuestion: (questionId: string) => void;
  setScore: (questionId: string, score: string) => void;
  moveQuestion: (questionId: string, offset: -1 | 1) => void;
  reorderQuestion: (sourceId: string, targetId: string) => void;
  setMetadata: (metadata: Partial<PaperMetadata>) => void;
  ensureNewPaper: (defaultSubject: string) => void;
  loadPaper: (paper: Paper) => void;
  restorePublishedQuestions: () => Promise<void>;
  completeHydration: (error?: unknown) => void;
}

export interface PaperStoreOptions {
  storage?: StateStorage;
  resolveQuestions?: (ids: string[]) => Promise<Question[]>;
}

const browserStorage: StateStorage = {
  getItem: (name) => {
    try {
      return typeof window === "undefined" ? null : window.localStorage.getItem(name);
    } catch {
      return null;
    }
  },
  setItem: (name, value) => {
    try {
      if (typeof window !== "undefined") window.localStorage.setItem(name, value);
    } catch {
      // 浏览器禁用存储时仍保留当前会话内的购物车。
    }
  },
  removeItem: (name) => {
    try {
      if (typeof window !== "undefined") window.localStorage.removeItem(name);
    } catch {
      // 与 setItem 保持一致，存储不可用时静默降级。
    }
  },
};

export function paperCartStorageKey(userId: string): string {
  return `paper-cart:${userId}`;
}

export function createPaperStore(userId: string, options: PaperStoreOptions = {}) {
  const resolve = options.resolveQuestions ?? resolveQuestions;
  let restoreRequest = 0;

  return create<PaperStoreState>()(
    persist(
      (set, get) => ({
        paperId: null,
        title: "",
        subject: "",
        grade: "",
        durationMinutes: DEFAULT_DURATION,
        items: [],
        hasHydrated: false,
        isResolving: false,
        resolutionError: null,
        removedInvalidCount: 0,
        addQuestion: (question) => {
          if (question.status !== "published" || get().items.some((item) => item.question.id === question.id)) {
            return false;
          }
          set((state) => ({
            items: [...state.items, { question, assignedScore: DEFAULT_SCORE }],
            removedInvalidCount: 0,
          }));
          return true;
        },
        removeQuestion: (questionId) => {
          set((state) => ({ items: state.items.filter((item) => item.question.id !== questionId) }));
        },
        setScore: (questionId, score) => {
          if (!SCORE_INPUT_PATTERN.test(score)) return;
          set((state) => ({
            items: state.items.map((item) => item.question.id === questionId ? { ...item, assignedScore: score } : item),
          }));
        },
        moveQuestion: (questionId, offset) => {
          set((state) => {
            const index = state.items.findIndex((item) => item.question.id === questionId);
            const target = index + offset;
            if (index < 0 || target < 0 || target >= state.items.length) return state;
            const items = [...state.items];
            [items[index], items[target]] = [items[target], items[index]];
            return { items };
          });
        },
        reorderQuestion: (sourceId, targetId) => {
          set((state) => {
            const sourceIndex = state.items.findIndex((item) => item.question.id === sourceId);
            const targetIndex = state.items.findIndex((item) => item.question.id === targetId);
            if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) return state;
            const items = [...state.items];
            const [moved] = items.splice(sourceIndex, 1);
            items.splice(targetIndex, 0, moved);
            return { items };
          });
        },
        setMetadata: (metadata) => set(metadata),
        ensureNewPaper: (defaultSubject) => {
          const state = get();
          if (state.paperId !== null) {
            set({
              paperId: null,
              title: "",
              subject: defaultSubject,
              grade: "",
              durationMinutes: DEFAULT_DURATION,
              items: [],
              resolutionError: null,
              removedInvalidCount: 0,
            });
          } else if (!state.subject && defaultSubject) {
            set({ subject: defaultSubject });
          }
        },
        loadPaper: (paper) => {
          restoreRequest += 1;
          set({
            paperId: paper.id,
            title: paper.title,
            subject: paper.subject,
            grade: paper.grade,
            durationMinutes: String(paper.duration_minutes),
            items: [...paper.questions]
              .sort((left, right) => left.order_index - right.order_index)
              .map((item) => ({ question: item.question, assignedScore: item.assigned_score })),
            isResolving: false,
            resolutionError: null,
            removedInvalidCount: 0,
          });
        },
        completeHydration: (error) => {
          set({
            hasHydrated: true,
            resolutionError: error ? "无法恢复本地组卷内容" : null,
          });
        },
        restorePublishedQuestions: async () => {
          const snapshot = get();
          const ids = snapshot.items.map((item) => item.question.id);
          const request = ++restoreRequest;
          if (ids.length === 0) {
            set({ isResolving: false, resolutionError: null });
            return;
          }
          set({ isResolving: true, resolutionError: null, removedInvalidCount: 0 });
          const isCurrentCart = () => {
            const current = get();
            return current.paperId === snapshot.paperId
              && current.items.map((item) => item.question.id).join(",") === ids.join(",");
          };
          try {
            const resolved = await resolve(ids);
            if (request !== restoreRequest || !isCurrentCart()) return;
            const resolvedById = new Map(resolved.filter((question) => question.status === "published").map((question) => [question.id, question]));
            set((state) => {
              const items = state.items.flatMap((item) => {
                const question = resolvedById.get(item.question.id);
                return question ? [{ ...item, question }] : [];
              });
              return { items, removedInvalidCount: state.items.length - items.length };
            });
          } catch (error) {
            if (request === restoreRequest && isCurrentCart()) {
              set({ resolutionError: error instanceof Error ? error.message : "无法校验已恢复的题目" });
            }
          } finally {
            if (request === restoreRequest) set({ isResolving: false });
          }
        },
      }),
      {
        name: paperCartStorageKey(userId),
        version: 1,
        skipHydration: true,
        storage: createJSONStorage(() => options.storage ?? browserStorage),
        partialize: (state) => ({
          paperId: state.paperId,
          title: state.title,
          subject: state.subject,
          grade: state.grade,
          durationMinutes: state.durationMinutes,
          items: state.items,
        }),
        onRehydrateStorage: () => (state, error) => {
          if (!state) return;
          state.completeHydration(error);
          void state.restorePublishedQuestions();
        },
      },
    ),
  );
}

export type PaperStoreApi = ReturnType<typeof createPaperStore>;

export async function rehydratePaperStore(store: PaperStoreApi): Promise<void> {
  try {
    await store.persist.rehydrate();
  } catch (error) {
    store.getState().completeHydration(error);
    return;
  }
  if (!store.getState().hasHydrated) {
    store.getState().completeHydration(new Error("persist rehydration failed"));
  }
}
