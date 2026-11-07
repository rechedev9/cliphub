import type { FullDemoBumperOptions, FullDemoDocument } from '@/lib/full-demo-plan';

export type FullDemoBumperSlot = keyof FullDemoBumperOptions;
export type FullDemoOrderPart = { kind: 'bumper'; slot: FullDemoBumperSlot } | { kind: 'rounds'; rounds: readonly number[] };

/** Where the planner puts each clip: intro first, sponsor after the second round (or the only one), outro last. */
export function fullDemoOrder(rounds: readonly number[]): readonly FullDemoOrderPart[] {
  return [
    { kind: 'bumper', slot: 'intro' },
    { kind: 'rounds', rounds: rounds.slice(0, 2) },
    { kind: 'bumper', slot: 'sponsor' },
    { kind: 'rounds', rounds: rounds.slice(2) },
    { kind: 'bumper', slot: 'outro' },
  ];
}

/** The source rounds the video covers, from the most specific source the page has. */
export function fullDemoOrderRounds({ chosen, planned, recap }: {
  chosen: readonly number[]; planned: readonly number[] | null; recap: readonly number[];
}): readonly number[] {
  const source = chosen.length > 0 ? chosen : planned ?? recap;
  return [...new Set(source)].sort((a, b) => a - b);
}

/** Output length of a planned timeline in seconds, or null when the plan has none yet. */
export function fullDemoDurationSeconds(document: Pick<FullDemoDocument, 'timeline' | 'clock'>): number | null {
  const end = Math.max(0, ...(document.timeline ?? []).map((entry) => entry.end_frame));
  return end > 0 ? Math.round(end / document.clock.output_fps) : null;
}

/** "41:08", or "1:02:05" past the hour. */
export function formatVideoDuration(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = String(seconds % 60).padStart(2, '0');
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, '0')}:${rest}` : `${minutes}:${rest}`;
}
