// Bounded retry with exponential backoff, for page loads that HLTV cuts mid-run.

export type RetryOptions = {
  attempts: number;
  baseMs: number;
  sleep: (ms: number) => Promise<void>;
  onRetry: (attempt: number, err: unknown) => void;
};

export async function withRetry<T>(fn: () => Promise<T>, opts: RetryOptions): Promise<T> {
  for (let attempt = 1; ; attempt++) {
    try {
      return await fn();
    } catch (err) {
      if (attempt >= opts.attempts) throw err;
      opts.onRetry(attempt, err);
      await opts.sleep(opts.baseMs * 2 ** (attempt - 1));
    }
  }
}
