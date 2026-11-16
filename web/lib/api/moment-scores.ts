import type { Play } from './types.ts';

/** What GET /api/jobs/{id}/moments (internal/moments) adds to a plan segment. */
export type MomentScore = {
  score: number;
  reasons: string[];
  clutchWon?: boolean;
  winProbSwing?: number;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function finite(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

/** Segment id → server score; plays the document does not name keep no score. */
export function parseMomentScores(body: unknown): Map<string, MomentScore> {
  const out = new Map<string, MomentScore>();
  const moments = isRecord(body) && Array.isArray(body.moments) ? body.moments : [];
  for (const moment of moments) {
    if (!isRecord(moment) || typeof moment.segment_id !== 'string') continue;
    const score = finite(moment.score);
    if (score === undefined) continue;
    const reasons = Array.isArray(moment.reason_codes) ? moment.reason_codes.filter((r): r is string => typeof r === 'string') : [];
    const entry: MomentScore = { score, reasons };
    if (typeof moment.clutch_won === 'boolean') entry.clutchWon = moment.clutch_won;
    const swing = finite(moment.win_prob_swing);
    if (swing !== undefined && swing !== 0) entry.winProbSwing = swing;
    out.set(moment.segment_id, entry);
  }
  return out;
}

/** Reads a moments response; any failure leaves the plays unscored, never fails the list. */
export async function readMomentScores(res: Response): Promise<Map<string, MomentScore> | undefined> {
  if (!res.ok) return undefined;
  return parseMomentScores(await res.json().catch(() => null));
}

export function withMomentScores(plays: Play[], scores: ReadonlyMap<string, MomentScore> | undefined): Play[] {
  if (!scores || scores.size === 0) return plays;
  return plays.map((play) => {
    const found = scores.get(play.id);
    return found ? { ...play, ...found } : play;
  });
}
