import type { EditorDocument } from './evaluate.ts';

export type PlanStoreState = { dirty: boolean; saving: boolean; lastError: string | null; locked: boolean };

export type PlanStore = {
  update(doc: EditorDocument): void;
  flush(): Promise<void>;
  lock(): void;
  unlock(): void;
  getState(): PlanStoreState;
};

const DEFAULT_DEBOUNCE_MS = 400;

function failureMessage(err: unknown): string {
  if (err instanceof Error && err.message !== '') return err.message;
  if (typeof err === 'string' && err !== '') return err;
  return 'No se pudo completar la operación.';
}

export function createPlanStore(opts: {
  putPlan: (doc: EditorDocument) => Promise<EditorDocument>;
  debounceMs?: number;
  schedule?: (fn: () => void, ms: number) => number;
  cancel?: (id: number) => void;
}): PlanStore {
  const debounceMs = opts.debounceMs ?? DEFAULT_DEBOUNCE_MS;
  const nativeTimers = new Map<number, ReturnType<typeof setTimeout>>();
  let nativeSeq = 1;
  const schedule = opts.schedule ?? ((fn, ms) => {
    const id = nativeSeq;
    nativeSeq += 1;
    nativeTimers.set(id, setTimeout(fn, ms));
    return id;
  });
  const cancel = opts.cancel ?? ((id) => {
    const handle = nativeTimers.get(id);
    if (handle === undefined) return;
    nativeTimers.delete(id);
    clearTimeout(handle);
  });

  const state: PlanStoreState = { dirty: false, saving: false, lastError: null, locked: false };
  let latest: EditorDocument | undefined;
  let timerId: number | undefined;
  let tail: Promise<void> = Promise.resolve();

  function armTimer(): void {
    if (timerId !== undefined) cancel(timerId);
    timerId = schedule(() => {
      timerId = undefined;
      void saveOnce(false);
    }, debounceMs);
  }

  function cancelTimer(): void {
    if (timerId === undefined) return;
    cancel(timerId);
    timerId = undefined;
  }

  async function doSave(retry: boolean): Promise<void> {
    if (!state.dirty && state.lastError === null) return;
    if (latest === undefined) return;
    state.saving = true;
    try {
      const attempts = retry ? 2 : 1;
      for (let attempt = 0; attempt < attempts; attempt += 1) {
        const snapshot: EditorDocument | undefined = latest;
        if (snapshot === undefined) return;
        try {
          await opts.putPlan(snapshot);
          if (latest === snapshot) state.dirty = false;
          state.lastError = null;
          return;
        } catch (err) {
          state.lastError = failureMessage(err);
        }
      }
    } finally {
      state.saving = false;
    }
  }

  function saveOnce(retry: boolean): Promise<void> {
    const run = tail.then(() => doSave(retry));
    tail = run.then(
      () => undefined,
      () => undefined,
    );
    return run;
  }

  return {
    update(doc: EditorDocument): void {
      if (state.locked) return;
      latest = doc;
      state.dirty = true;
      armTimer();
    },
    async flush(): Promise<void> {
      cancelTimer();
      await tail;
      if (!state.dirty && state.lastError === null) return;
      await saveOnce(true);
      if (state.dirty && state.lastError === null) await saveOnce(true);
    },
    lock(): void {
      state.locked = true;
    },
    unlock(): void {
      state.locked = false;
      if (state.dirty) armTimer();
    },
    getState(): PlanStoreState {
      return { ...state };
    },
  };
}
