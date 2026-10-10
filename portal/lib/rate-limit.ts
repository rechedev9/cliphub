export interface RateLimiterOptions {
  limit: number;
  windowMs: number;
}

export interface RateLimiter {
  // Records one attempt and says whether it is within the limit.
  allow: (key: string, now: number) => boolean;
}

const MAX_KEYS = 10000;

// In-memory sliding window. Enough for one container; it resets on restart.
export function createRateLimiter(options: RateLimiterOptions): RateLimiter {
  const hits = new Map<string, number[]>();

  return {
    allow(key, now) {
      const since = now - options.windowMs;
      if (hits.size > MAX_KEYS) {
        for (const [other, times] of hits) {
          if (times.every((time) => time <= since)) hits.delete(other);
        }
      }
      const recent = (hits.get(key) ?? []).filter((time) => time > since);
      if (recent.length >= options.limit) {
        hits.set(key, recent);
        return false;
      }
      recent.push(now);
      hits.set(key, recent);
      return true;
    },
  };
}
