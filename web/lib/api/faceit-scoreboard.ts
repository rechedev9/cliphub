/**
 * FACEIT room scoreboard for a demo downloaded from FACEIT. The orchestrator
 * resolves the match from the demo file name and checks it against the parsed
 * roster; any failure here means "show the demo's own scoreboard instead".
 */

export type FaceitScoreboardPlayer = {
  playerId: string;
  nickname: string;
  steamId?: string;
  avatar?: string;
  skillLevel?: number;
  elo?: number;
  kills: number;
  deaths: number;
  assists: number;
  headshots: number;
  hsPct: number;
  adr: number;
  kd: number;
  kr: number;
  mvps: number;
  rounds2k: number;
  rounds3k: number;
  rounds4k: number;
  rounds5k: number;
};

export type FaceitScoreboardTeam = {
  name: string;
  score: number;
  firstHalf: number;
  secondHalf: number;
  overtime: number;
  won: boolean;
  averageElo?: number;
  players: FaceitScoreboardPlayer[];
};

export type FaceitScoreboard = {
  matchId: string;
  roomUrl: string;
  map: string;
  rounds: number;
  teams: FaceitScoreboardTeam[];
};

type Rec = Record<string, unknown>;

function asRecord(value: unknown): Rec | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Rec) : null;
}

function num(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function optNum(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : undefined;
}

function optStr(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

const ROOM_URL_RE = /^https:\/\/www\.faceit\.com\/[a-z]{2}\/cs2\/room\/[A-Za-z0-9_-]+$/;
const STEAM_ID_RE = /^\d{17}$/;

function parsePlayer(raw: unknown): FaceitScoreboardPlayer | null {
  const p = asRecord(raw);
  const playerId = optStr(p?.player_id);
  const nickname = optStr(p?.nickname);
  if (!p || playerId === undefined || nickname === undefined) return null;
  const steamId = optStr(p.steam_id64);
  return {
    playerId,
    nickname,
    steamId: steamId !== undefined && STEAM_ID_RE.test(steamId) ? steamId : undefined,
    avatar: optStr(p.avatar),
    skillLevel: optNum(p.skill_level),
    elo: optNum(p.elo),
    kills: num(p.kills),
    deaths: num(p.deaths),
    assists: num(p.assists),
    headshots: num(p.headshots),
    hsPct: num(p.hs_pct),
    adr: num(p.adr),
    kd: num(p.kd),
    kr: num(p.kr),
    mvps: num(p.mvps),
    rounds2k: num(p.double_kills),
    rounds3k: num(p.triple_kills),
    rounds4k: num(p.quadro_kills),
    rounds5k: num(p.penta_kills),
  };
}

function parseTeam(raw: unknown): FaceitScoreboardTeam | null {
  const t = asRecord(raw);
  if (!t || !Array.isArray(t.players)) return null;
  const players = t.players.map(parsePlayer);
  if (players.some((p) => p === null)) return null;
  return {
    name: optStr(t.name) ?? '',
    score: num(t.score),
    firstHalf: num(t.first_half),
    secondHalf: num(t.second_half),
    overtime: num(t.overtime),
    won: t.won === true,
    averageElo: optNum(t.average_elo),
    players: players as FaceitScoreboardPlayer[],
  };
}

/**
 * Parses the orchestrator's snake_case `{ scoreboard }` body into the shape the
 * same-origin proxy serves; null when it is not a usable scoreboard.
 */
export function parseFaceitScoreboard(raw: unknown): FaceitScoreboard | null {
  const board = asRecord(asRecord(raw)?.scoreboard);
  const matchId = optStr(board?.match_id);
  const roomUrl = optStr(board?.room_url);
  if (!board || matchId === undefined || roomUrl === undefined || !ROOM_URL_RE.test(roomUrl)) return null;
  if (!Array.isArray(board.teams) || board.teams.length === 0) return null;
  const teams = board.teams.map(parseTeam);
  if (teams.some((t) => t === null)) return null;
  return { matchId, roomUrl, map: optStr(board.map) ?? '', rounds: num(board.rounds), teams: teams as FaceitScoreboardTeam[] };
}

/** The FACEIT scoreboard of a scanned job, or null for a non-FACEIT demo or any failure. */
export async function getFaceitScoreboard(jobId: string, signal?: AbortSignal): Promise<FaceitScoreboard | null> {
  try {
    const res = await fetch(`/api/demos/${encodeURIComponent(jobId)}/faceit-scoreboard`, { cache: 'no-store', signal });
    if (!res.ok) return null;
    // The proxy already whitelisted and reshaped the upstream body.
    const board = asRecord(asRecord(await res.json())?.scoreboard);
    return board && Array.isArray(board.teams) && board.teams.length > 0 ? (board as FaceitScoreboard) : null;
  } catch {
    return null;
  }
}
