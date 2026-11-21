import type { KillPlan } from '../api/map.ts';
import { apiFetch } from '../api/edition-fetch.ts';

/** The players a parsed plan films, anchor first; empty when the plan names no target. */
export function planPick(plan: KillPlan): string[] {
  const anchor = plan.target?.steamid64 ?? '';
  if (anchor === '') return [];
  const pick = [anchor];
  for (const segment of plan.segments ?? []) {
    const pov = segment.pov?.steamid64;
    if (pov && !pick.includes(pov)) pick.push(pov);
  }
  return pick;
}

/** Same anchor and the same teammates in any order: the job already holds this pick. */
export function isSamePick(a: readonly string[], b: readonly string[]): boolean {
  if (a.length === 0 || a.length !== b.length || a[0] !== b[0]) return false;
  const rest = new Set(a.slice(1));
  return b.slice(1).every((id) => rest.has(id));
}

/** The pick of a parsed job, or null when its plan cannot be read. */
export async function getOriginPick(jobId: string, signal?: AbortSignal): Promise<string[] | null> {
  try {
    const res = await apiFetch(`/api/demos/${encodeURIComponent(jobId)}/plan`, { cache: 'no-store', signal });
    if (!res.ok) return null;
    const plan: KillPlan = await res.json();
    const pick = planPick(plan);
    return pick.length > 0 ? pick : null;
  } catch {
    return null;
  }
}
