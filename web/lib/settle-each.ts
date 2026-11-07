/** Requests a bulk action keeps in flight at once, so a large selection does not flood the local service. */
export const SETTLE_EACH_LIMIT = 4;

export type SettleOutcome<T, R> = {
  done: { item: T; value: R }[];
  failed: { item: T; error: unknown }[];
};

/** Runs `run` over every item, at most SETTLE_EACH_LIMIT at a time, and keeps each result in item order. */
export async function settleEach<T, R>(items: readonly T[], run: (item: T) => Promise<R>): Promise<SettleOutcome<T, R>> {
  const results: ({ ok: true; value: R } | { ok: false; error: unknown })[] = [];
  let next = 0;
  const worker = async (): Promise<void> => {
    while (next < items.length) {
      const index = next;
      next += 1;
      try {
        results[index] = { ok: true, value: await run(items[index]) };
      } catch (error) {
        results[index] = { ok: false, error };
      }
    }
  };
  await Promise.all(Array.from({ length: Math.min(SETTLE_EACH_LIMIT, items.length) }, worker));
  const outcome: SettleOutcome<T, R> = { done: [], failed: [] };
  items.forEach((item, index) => {
    const result = results[index];
    if (result?.ok) outcome.done.push({ item, value: result.value });
    else outcome.failed.push({ item, error: result?.error });
  });
  return outcome;
}

/** The selection minus what a bulk action finished, keeping failures and anything picked meanwhile. */
export function withoutKeys(current: ReadonlySet<string>, keys: readonly string[]): Set<string> {
  const next = new Set(current);
  for (const key of keys) next.delete(key);
  return next;
}
