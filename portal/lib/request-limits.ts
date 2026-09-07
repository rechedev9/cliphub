// Caps how many requests one user can have in flight against the single
// shared rendering resource (the owner's own PC). Enforced at creation time
// in app/api/requests/route.ts.

export const ACTIVE_REQUEST_STATUSES = [
  "awaiting_demo",
  "pending",
  "approved",
  "processing",
] as const;

export interface RequestLimits {
  maxActive: number;
  maxDaily: number;
}

export function loadRequestLimits(
  env: Record<string, string | undefined> = process.env,
): RequestLimits {
  return {
    maxActive: Number(env.MAX_ACTIVE_REQUESTS_PER_USER ?? 3),
    maxDaily: Number(env.MAX_DAILY_REQUESTS_PER_USER ?? 5),
  };
}

export type LimitViolation = "active" | "daily" | null;

export function checkRequestLimits(
  counts: { active: number; daily: number },
  limits: RequestLimits,
): LimitViolation {
  if (counts.active >= limits.maxActive) return "active";
  if (counts.daily >= limits.maxDaily) return "daily";
  return null;
}
