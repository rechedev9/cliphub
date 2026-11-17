import type { StreamClipGroup, StreamClipRange, StreamEditPlan } from '../api/streams.ts';
import { clipOutputDuration } from './plan.ts';

/** YouTube Shorts length limit (3 min); a joined Short past it gets a warning. */
export const SHORT_MAX_SECONDS = 180;

/** One exported video: a lone moment or a group of moments played back to back. */
export type StreamShort = {
  id: string;
  title: string;
  clips: StreamClipRange[];
  durationSeconds: number;
  groupId: string | null;
};

/** The videos an export makes, mirroring streamclips.EditPlan.Outputs: a group sits where its first moment is. */
export function streamPlanShorts(plan: StreamEditPlan): StreamShort[] {
  const groupOf = new Map<string, StreamClipGroup>();
  for (const group of plan.groups ?? []) for (const id of group.clip_ids) groupOf.set(id, group);
  const byId = new Map(plan.clips.map((clip, index) => [clip.id, { clip, index }]));
  const title = (id: string): string => {
    const entry = byId.get(id);
    return entry?.clip.title?.trim() || `Momento ${(entry?.index ?? 0) + 1}`;
  };
  const emitted = new Set<string>();
  const shorts: StreamShort[] = [];
  for (const clip of plan.clips) {
    const group = groupOf.get(clip.id);
    if (!group) {
      shorts.push({
        id: clip.id,
        title: title(clip.id),
        clips: [clip],
        durationSeconds: clipOutputDuration(clip),
        groupId: null,
      });
      continue;
    }
    if (emitted.has(group.id)) continue;
    emitted.add(group.id);
    const members = group.clip_ids.flatMap((id) => byId.get(id)?.clip ?? []);
    shorts.push({
      id: group.id,
      title: group.title?.trim() || members.map((c) => title(c.id)).join(' + '),
      clips: members,
      durationSeconds: members.reduce((sum, c) => sum + clipOutputDuration(c), 0),
      groupId: group.id,
    });
  }
  return shorts;
}

export function streamShortCount(plan: StreamEditPlan): number {
  return streamPlanShorts(plan).length;
}

/** "6 momentos", or "6 momentos · 5 Shorts" once joined moments make fewer videos than moments. */
export function streamMomentsLabel(moments: number, shorts: number): string {
  const word = moments === 1 ? '1 momento' : `${moments} momentos`;
  if (shorts === moments) return word;
  return `${word} · ${shorts === 1 ? '1 Short' : `${shorts} Shorts`}`;
}

/** Where the music already is when each moment starts: inside a group it runs on, as the export does. */
export function streamMusicOffsets(shorts: readonly StreamShort[]): Map<string, number> {
  const offsets = new Map<string, number>();
  for (const short of shorts) {
    let offset = 0;
    for (const clip of short.clips) {
      offsets.set(clip.id, offset);
      offset += clipOutputDuration(clip);
    }
  }
  return offsets;
}

/** Drops removed moments from their groups and any group left with one, as the Go plan does. */
export function pruneStreamGroups(plan: StreamEditPlan): StreamEditPlan {
  if (!plan.groups) return plan;
  const ids = new Set(plan.clips.map((c) => c.id));
  const groups = plan.groups
    .map((g) => ({ ...g, clip_ids: g.clip_ids.filter((id) => ids.has(id)) }))
    .filter((g) => g.clip_ids.length >= 2);
  const { groups: _old, ...rest } = plan;
  return groups.length > 0 ? { ...rest, groups } : rest;
}

let groupSeq = 0;

/** Joins the moments into one Short in source order; they leave any group they were in. */
export function joinStreamMoments(plan: StreamEditPlan, clipIds: readonly string[]): StreamEditPlan {
  const chosen = plan.clips.filter((c) => clipIds.includes(c.id)).map((c) => c.id);
  if (chosen.length < 2) return plan;
  groupSeq += 1;
  const group: StreamClipGroup = { id: `group-${Date.now()}-${groupSeq}`, clip_ids: chosen };
  const others = (plan.groups ?? []).map((g) => ({ ...g, clip_ids: g.clip_ids.filter((id) => !chosen.includes(id)) }));
  return pruneStreamGroups({ ...plan, groups: [...others, group] });
}

/** Puts back a group a removed moment left, as Deshacer does; a member since joined elsewhere stays there. */
export function restoreStreamGroup(plan: StreamEditPlan, group: StreamClipGroup): StreamEditPlan {
  const ids = new Set(plan.clips.map((c) => c.id));
  const current = plan.groups ?? [];
  const taken = new Set(current.filter((g) => g.id !== group.id).flatMap((g) => g.clip_ids));
  const restored = { ...group, clip_ids: group.clip_ids.filter((id) => ids.has(id) && !taken.has(id)) };
  const groups = current.some((g) => g.id === group.id)
    ? current.map((g) => (g.id === group.id ? restored : g))
    : [...current, restored];
  return pruneStreamGroups({ ...plan, groups });
}

export function splitStreamGroup(plan: StreamEditPlan, groupId: string): StreamEditPlan {
  return pruneStreamGroups({ ...plan, groups: (plan.groups ?? []).filter((g) => g.id !== groupId) });
}

/** Moves one moment earlier (-1) or later (1) inside its group. */
export function moveStreamGroupMoment(
  plan: StreamEditPlan,
  move: { groupId: string; clipId: string; offset: -1 | 1 },
): StreamEditPlan {
  const groups = (plan.groups ?? []).map((g) => {
    if (g.id !== move.groupId) return g;
    const from = g.clip_ids.indexOf(move.clipId);
    const to = from + move.offset;
    if (from < 0 || to < 0 || to >= g.clip_ids.length) return g;
    const clip_ids = [...g.clip_ids];
    [clip_ids[from], clip_ids[to]] = [clip_ids[to], clip_ids[from]];
    return { ...g, clip_ids };
  });
  return { ...plan, groups };
}
