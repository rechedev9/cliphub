import { getLibraryMatch, getLibraryPlayerRole, type LibraryMatch, type LibraryPlayerRole } from '@/lib/api/library';

export type TeamRequests = {
  readonly roles: Map<string, Promise<LibraryPlayerRole>>;
  readonly matches: Map<string, Promise<LibraryMatch>>;
};

export const TREND_MATCHES = 12;

export function newTeamRequests(): TeamRequests {
  return { roles: new Map(), matches: new Map() };
}

function shared<T>(cache: Map<string, Promise<T>>, id: string, load: (id: string) => Promise<T>): Promise<T> {
  const known = cache.get(id);
  if (known !== undefined) return known;
  const next = load(id);
  cache.set(id, next);
  next.catch(() => cache.delete(id));
  return next;
}

export function playerRole(requests: TeamRequests, steamId: string): Promise<LibraryPlayerRole> {
  return shared(requests.roles, steamId, getLibraryPlayerRole);
}

export function matchDetail(requests: TeamRequests, matchId: string): Promise<LibraryMatch> {
  return shared(requests.matches, matchId, getLibraryMatch);
}

export function recentMatchIds(matches: readonly LibraryMatch[]): string[] {
  return [...matches]
    .sort((a, b) => Date.parse(b.played_at) - Date.parse(a.played_at))
    .slice(0, TREND_MATCHES)
    .map((match) => match.id);
}
