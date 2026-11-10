// Bounded retry with exponential backoff, for page loads that HLTV cuts mid-run,
// and the failure classes that must stop a run instead of a single match.

export type RetryOptions = {
  attempts: number;
  baseMs: number;
  sleep: (ms: number) => Promise<void>;
  onRetry: (attempt: number, err: unknown) => void;
  /** Only a rejection this accepts is retried; any other is rethrown at once. */
  isTransient: (err: unknown) => boolean;
};

export async function withRetry<T>(fn: () => Promise<T>, opts: RetryOptions): Promise<T> {
  for (let attempt = 1; ; attempt++) {
    try {
      return await fn();
    } catch (err) {
      if (attempt >= opts.attempts || !opts.isTransient(err)) throw err;
      opts.onRetry(attempt, err);
      await opts.sleep(opts.baseMs * 2 ** (attempt - 1));
    }
  }
}

const TRANSIENT_NET_ERRORS = [
  "ERR_CONNECTION_CLOSED", "ERR_CONNECTION_RESET", "ERR_TIMED_OUT", "ERR_ABORTED", "ERR_NETWORK_CHANGED", "ERR_HTTP2_PROTOCOL_ERROR",
];

/** True for the network errors a page load can recover from; a bad URL, DNS failure or destroyed window is not. */
export function isTransient(err: unknown): boolean {
  const text = err instanceof Error ? err.message : String(err);
  return TRANSIENT_NET_ERRORS.some((code) => text.includes(code));
}

/** Cloudflare was not cleared in time: every later page would wait just as long, so the run ends. */
export class ChallengeTimeoutError extends Error {
  constructor() {
    super("Cloudflare challenge was not completed within 30 minutes");
    this.name = "ChallengeTimeoutError";
  }
}

/** Counts consecutive failures; a success resets it and the limit-th in a row trips it. */
export class FailureBreaker {
  private streak = 0;
  private readonly limit: number;

  constructor(limit: number) {
    this.limit = limit;
  }

  success(): void {
    this.streak = 0;
  }

  /** Records a failure and returns true when the run should stop. */
  failure(): boolean {
    this.streak++;
    return this.streak >= this.limit;
  }
}
