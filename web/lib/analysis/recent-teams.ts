/** localStorage key of the teams opened last; Studio's storage lives in the profile, so each profile keeps its own. */
export const RECENT_TEAMS_KEY = 'cliphub.recent-teams.v1';
export const RECENT_TEAMS_MAX = 4;

/** Team ids from stored JSON; anything unreadable is an empty list. */
export function parseRecentTeams(raw: string | null): string[] {
  if (raw === null) return [];
  try {
    const value: unknown = JSON.parse(raw);
    if (!Array.isArray(value)) return [];
    const ids = value.filter((item): item is string => typeof item === 'string' && item !== '');
    return [...new Set(ids)].slice(0, RECENT_TEAMS_MAX);
  } catch {
    return [];
  }
}

/** The list with `id` first, no repeats, and at most the last four. */
export function withRecentTeam(list: readonly string[], id: string): string[] {
  return [id, ...list.filter((item) => item !== id)].slice(0, RECENT_TEAMS_MAX);
}

/** The recent teams that still exist, newest first. */
export function recentTeamOptions<T extends { readonly value: string }>(recent: readonly string[], options: readonly T[]): T[] {
  return recent.flatMap((id) => options.filter((option) => option.value === id));
}

export function readRecentTeams(): string[] {
  try {
    return parseRecentTeams(window.localStorage.getItem(RECENT_TEAMS_KEY));
  } catch {
    return [];
  }
}

/** Puts the team first in the stored list and returns it; storage that refuses leaves the list as it was. */
export function rememberRecentTeam(id: string): string[] {
  const next = withRecentTeam(readRecentTeams(), id);
  try {
    window.localStorage.setItem(RECENT_TEAMS_KEY, JSON.stringify(next));
  } catch {
    // Private windows and blocked storage only lose the shortcut.
  }
  return next;
}
