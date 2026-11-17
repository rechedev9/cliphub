import type { BrandKit } from './brand-kit';

export type BrandKitState = { kit: BrandKit | null; error: string | null };
export type BrandKitStore = {
  subscribe: (listener: () => void) => () => void;
  snapshot: () => BrandKitState;
  setKit: (kit: BrandKit) => void;
};

/** One kit shared by every editor and preview on screen; the first subscriber reads it again, a save reaches them all. */
export function createBrandKitStore(load: (signal: AbortSignal) => Promise<BrandKit>): BrandKitStore {
  let state: BrandKitState = { kit: null, error: null };
  let controller: AbortController | null = null;
  const listeners = new Set<() => void>();
  const publish = (next: BrandKitState): void => {
    state = next;
    for (const listener of listeners) listener();
  };
  return {
    subscribe(listener) {
      listeners.add(listener);
      if (listeners.size === 1) {
        const current = new AbortController();
        controller = current;
        load(current.signal).then((kit) => {
          if (!current.signal.aborted) publish({ kit, error: null });
        }).catch((failure: unknown) => {
          if (!current.signal.aborted) publish({ ...state, error: failure instanceof Error ? failure.message : 'No se pudo leer el kit de marca.' });
        });
      }
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0) {
          controller?.abort();
          controller = null;
        }
      };
    },
    snapshot: () => state,
    setKit: (kit) => publish({ kit, error: null }),
  };
}
