/** localStorage key of the teams opened last; Studio's storage lives in the profile, so each profile keeps its own. */
export const RECENT_TEAMS_KEY = 'cliphub.recent-teams.v1';
/** "Recientes" lists this many teams besides the open one. */
export const RECENT_TEAMS_SHOWN = 4;
/** The open team and the ones shown beside it. */
const RECENT_TEAMS_STORED = RECENT_TEAMS_SHOWN + 1;

/** Team ids from stored JSON; anything unreadable is an empty list. */
export function parseRecentTeams(raw: string | null): string[] {
  if (raw === null) return [];
  try {
    const value: unknown = JSON.parse(raw);
    if (!Array.isArray(value)) return [];
    return [...new Set(value.filter((item): item is string => typeof item === 'string' && item !== ''))];
  } catch {
    return [];
  }
}

export type RecentTeamUpdate = {
  list: readonly string[];
  /** The team just opened. */
  id: string;
  /** Ids of the teams the library has; the others were merged or deleted and drop out first. */
  known: readonly string[];
};

/** The open team first, then the teams still in the library, at most the open one and four more. */
export function withRecentTeam({ list, id, known }: RecentTeamUpdate): string[] {
  const alive = new Set(known);
  return [id, ...list.filter((item) => item !== id && alive.has(item))].slice(0, RECENT_TEAMS_STORED);
}

/** The recent teams to list: not the open one, and only those the picker offers. */
export function recentTeamOptions<T extends { readonly value: string }>(recent: readonly string[], options: readonly T[], openId: string): T[] {
  return recent
    .filter((id) => id !== openId)
    .flatMap((id) => options.filter((option) => option.value === id))
    .slice(0, RECENT_TEAMS_SHOWN);
}

function readRecentTeams(): string[] {
  try {
    return parseRecentTeams(window.localStorage.getItem(RECENT_TEAMS_KEY));
  } catch {
    return [];
  }
}

/** Puts the team first in the stored list, dropping teams that no longer exist, and returns it. */
export function rememberRecentTeam({ id, known }: Pick<RecentTeamUpdate, 'id' | 'known'>): string[] {
  const next = withRecentTeam({ list: readRecentTeams(), id, known });
  try {
    window.localStorage.setItem(RECENT_TEAMS_KEY, JSON.stringify(next));
  } catch {
    // Private windows and blocked storage only lose the shortcut.
  }
  return next;
}
