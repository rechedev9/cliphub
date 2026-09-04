import { FACEIT_NOT_CONFIGURED_CODE, SERVICE_UNAVAILABLE_CODE } from './types.ts';

export const FACEIT_CODES = {
  notConfigured: FACEIT_NOT_CONFIGURED_CODE,
  unauthorized: 'faceit_unauthorized',
  rateLimited: 'faceit_rate_limited',
  unavailable: 'faceit_unavailable',
  invalidResponse: 'faceit_invalid_response',
} as const;

export class FaceitServiceError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = 'FaceitServiceError';
    this.code = code;
    this.status = status;
  }
}

export type FaceitPlayer = {
  id: string;
  nickname: string;
  avatar?: string;
  profile_url: string;
  steam_id64?: string;
  country?: string;
  skill_level?: number;
  elo?: number;
};

export type FaceitFollowedPlayer = FaceitPlayer & {
  followed_at?: string;
  seeded?: boolean;
  region?: string;
  position?: number;
};

type FaceitMatchScore = {
  player_team?: string;
  winner_team?: string;
  for?: number;
  against?: number;
};

export type FaceitMatchStats = {
  map?: string;
  result: 'win' | 'loss' | 'unknown';
  kills: number;
  deaths: number;
  assists: number;
  adr?: number;
  kd_ratio?: number;
  headshots_percent?: number;
};

export type FaceitMatch = {
  id: string;
  room_url: string;
  started_at?: string;
  finished_at?: string;
  competition?: string;
  score: FaceitMatchScore;
  stats?: FaceitMatchStats;
};

export type FaceitFollowedList = {
  enabled: boolean;
  players: FaceitFollowedPlayer[];
};

const PLAYER_ID_RE = /^[A-Za-z0-9_-]{1,128}$/;

export function isFaceitPlayerID(value: string): boolean {
  return PLAYER_ID_RE.test(value);
}

export async function lookupFaceitPlayer(nickname: string): Promise<FaceitPlayer> {
  const url = `/api/faceit/players?nickname=${encodeURIComponent(nickname)}`;
  const res = await request(url);
  if (!res.ok) throw await serviceError(res);
  return parsePlayer((await res.json()) as unknown, 'player');
}

export async function listFaceitMatches(playerID: string, limit = 20): Promise<FaceitMatch[]> {
  if (!isFaceitPlayerID(playerID)) {
    throw new FaceitServiceError('FACEIT player id is invalid', 'invalid_player_id', 400);
  }
  const url = `/api/faceit/players/${encodeURIComponent(playerID)}/matches?limit=${limit}`;
  const res = await request(url);
  if (!res.ok) throw await serviceError(res);
  return parseMatches((await res.json()) as unknown);
}

export async function listFollowedFaceitPlayers(): Promise<FaceitFollowedList> {
  const res = await request('/api/faceit/followed');
  if (!res.ok) throw await serviceError(res);
  return parseFollowedList((await res.json()) as unknown);
}

export async function followFaceitPlayer(nickname: string): Promise<FaceitFollowedPlayer> {
  const res = await request('/api/faceit/followed', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ nickname }),
  });
  if (!res.ok) throw await serviceError(res);
  return parsePlayer((await res.json()) as unknown, 'player');
}

export async function unfollowFaceitPlayer(playerID: string): Promise<void> {
  if (!isFaceitPlayerID(playerID)) {
    throw new FaceitServiceError('FACEIT player id is invalid', 'invalid_player_id', 400);
  }
  const res = await request(`/api/faceit/followed/${encodeURIComponent(playerID)}`, { method: 'DELETE' });
  if (!res.ok) throw await serviceError(res);
}

async function request(url: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(url, { cache: 'no-store', ...init });
  } catch (err) {
    throw new FaceitServiceError(`servicio local inaccesible: ${String(err)}`, SERVICE_UNAVAILABLE_CODE, 503);
  }
}

async function serviceError(res: Response): Promise<FaceitServiceError> {
  let message = `HTTP ${res.status}`;
  let code = res.status === 503 ? SERVICE_UNAVAILABLE_CODE : `http_${res.status}`;
  try {
    const body = (await res.json()) as { error?: unknown; code?: unknown };
    if (typeof body.error === 'string' && body.error !== '') message = body.error;
    if (typeof body.code === 'string' && body.code !== '') code = body.code;
  } catch {
    return new FaceitServiceError(message, code, res.status);
  }
  return new FaceitServiceError(message, code, res.status);
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

function optionalNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function parsePlayer(raw: unknown, key: string): FaceitFollowedPlayer {
  const root = asRecord(raw);
  const player = asRecord(root?.[key]);
  const id = optionalString(player?.id);
  const nickname = optionalString(player?.nickname);
  const profileURL = optionalString(player?.profile_url);
  if (player === null || id === undefined || nickname === undefined || profileURL === undefined) {
    throw new FaceitServiceError('FACEIT player response is invalid', FACEIT_CODES.invalidResponse, 502);
  }
  return {
    id,
    nickname,
    avatar: optionalString(player.avatar),
    profile_url: profileURL,
    steam_id64: optionalString(player.steam_id64),
    country: optionalString(player.country),
    skill_level: optionalNumber(player.skill_level),
    elo: optionalNumber(player.elo),
    followed_at: optionalString(player.followed_at),
    seeded: player.seeded === true ? true : undefined,
    region: optionalString(player.region),
    position: optionalNumber(player.position),
  };
}

function parseFollowedList(raw: unknown): FaceitFollowedList {
  const root = asRecord(raw);
  const playersRaw = root?.players;
  if (root === null || !Array.isArray(playersRaw)) {
    throw new FaceitServiceError('FACEIT follow list is invalid', FACEIT_CODES.invalidResponse, 502);
  }
  return {
    enabled: root.enabled === true,
    players: playersRaw.map((item) => parsePlayer({ player: item }, 'player')),
  };
}

function parseMatchResult(value: unknown): FaceitMatchStats['result'] {
  return value === 'win' || value === 'loss' ? value : 'unknown';
}

function parseMatch(raw: unknown): FaceitMatch {
  const match = asRecord(raw);
  const id = optionalString(match?.id);
  const roomURL = optionalString(match?.room_url);
  if (match === null || id === undefined || roomURL === undefined) {
    throw new FaceitServiceError('FACEIT match response is invalid', FACEIT_CODES.invalidResponse, 502);
  }
  const score = asRecord(match.score) ?? {};
  const statsRaw = asRecord(match.stats);
  return {
    id,
    room_url: roomURL,
    started_at: optionalString(match.started_at),
    finished_at: optionalString(match.finished_at),
    competition: optionalString(match.competition),
    score: {
      player_team: optionalString(score.player_team),
      winner_team: optionalString(score.winner_team),
      for: optionalNumber(score.for),
      against: optionalNumber(score.against),
    },
    stats: statsRaw === null
      ? undefined
      : {
          map: optionalString(statsRaw.map),
          result: parseMatchResult(statsRaw.result),
          kills: optionalNumber(statsRaw.kills) ?? 0,
          deaths: optionalNumber(statsRaw.deaths) ?? 0,
          assists: optionalNumber(statsRaw.assists) ?? 0,
          adr: optionalNumber(statsRaw.adr),
          kd_ratio: optionalNumber(statsRaw.kd_ratio),
          headshots_percent: optionalNumber(statsRaw.headshots_percent),
        },
  };
}

function parseMatches(raw: unknown): FaceitMatch[] {
  const root = asRecord(raw);
  const matches = root?.matches;
  if (root === null || !Array.isArray(matches)) {
    throw new FaceitServiceError('FACEIT match list is invalid', FACEIT_CODES.invalidResponse, 502);
  }
  return matches.map(parseMatch);
}
