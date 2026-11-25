export const FACEIT = "https://www.faceit.com";

const ROOM_ID = /1-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi;
const DEMO_NAME = /^1-[0-9a-f-]{36}(?:-\d+)+\.dem\.zst$/;
const RESOURCE_HOST = ".faceit-cdn.net";
const SIGNED_HOSTS = [".backblazeb2.com", ".faceit-cdn.net"];
const OFF_ORIGIN = -1;
const NO_ANSWER = 0;

export type FaceitPlayer = { nickname: string; steamid64: string };

export type FaceitTeam = { name: string; score: number; players: FaceitPlayer[] };

export type FaceitMatch = {
  roomId: string;
  map: string;
  startedAt: string;
  finishedAt: string;
  demoUrls: string[];
  teams: FaceitTeam[];
};

export type FaceitSidecar = {
  source: "faceit";
  room_id: string;
  room_url: string;
  demo_url: string;
  file: string;
  map: string;
  played_at: string;
  finished_at: string;
  teams: FaceitTeam[];
};

export type ApiReply = { status: number; body: string };

export type ReplyKind = "ok" | "needs_user" | "gone" | "error";

export function roomIds(text: string): string[] {
  const out: string[] = [];
  for (const hit of text.matchAll(ROOM_ID)) {
    const id = hit[0].toLowerCase();
    if (!out.includes(id)) out.push(id);
  }
  return out;
}

export function roomUrl(roomId: string): string {
  return `${FACEIT}/en/cs2/room/${roomId}`;
}

export function demoFileName(resourceUrl: string): string {
  const name = new URL(resourceUrl).pathname.split("/").pop() ?? "";
  return DEMO_NAME.test(name) ? name : "";
}

export function sidecarName(demoName: string): string {
  return demoName.replace(/\.dem\.zst$/, ".json");
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function httpsHost(raw: string, suffixes: string[]): boolean {
  try {
    const url = new URL(raw);
    return url.protocol === "https:" && suffixes.some((s) => url.hostname.endsWith(s));
  } catch {
    return false;
  }
}

export function isDemoResource(raw: string): boolean {
  return httpsHost(raw, [RESOURCE_HOST]) && demoFileName(raw) !== "";
}

export function matchScript(roomId: string, timeoutMs: number): string {
  return apiScript(`/api/match/v2/match/${roomId}`, null, timeoutMs);
}

export function signScript(resourceUrl: string, timeoutMs: number): string {
  return apiScript("/api/download/v2/demos/download-url", { resource_url: resourceUrl }, timeoutMs);
}

function apiScript(path: string, payload: unknown, timeoutMs: number): string {
  const init = payload === null
    ? `{ credentials: 'include', signal: AbortSignal.timeout(${Math.max(1, Math.round(timeoutMs))}) }`
    : `{ method: 'POST', credentials: 'include', headers: { 'content-type': 'application/json' }, body: ${JSON.stringify(JSON.stringify(payload))}, signal: AbortSignal.timeout(${Math.max(1, Math.round(timeoutMs))}) }`;
  return `location.origin !== ${JSON.stringify(FACEIT)} ? Promise.resolve({ status: ${OFF_ORIGIN}, body: '' }) : fetch(${JSON.stringify(path)}, ${init}).then(async (r) => ({ status: r.status, body: await r.text() })).catch(() => ({ status: ${NO_ANSWER}, body: '' }))`;
}

export function parseReply(raw: unknown): ApiReply {
  if (!isRecord(raw) || typeof raw.status !== "number") return { status: NO_ANSWER, body: "" };
  return { status: raw.status, body: str(raw.body) };
}

export function replyKind(reply: ApiReply, waiting: boolean): ReplyKind {
  if (reply.status === 200) return "ok";
  if (reply.status === 401 || reply.status === 403 || reply.status === OFF_ORIGIN) return "needs_user";
  if (reply.status === 404 || reply.status === 410) return "gone";
  if (waiting && reply.status === NO_ANSWER) return "needs_user";
  return "error";
}

function team(raw: unknown, result: unknown): FaceitTeam | null {
  if (!isRecord(raw)) return null;
  const roster = Array.isArray(raw.roster) ? raw.roster : [];
  const players = roster.filter(isRecord).map((p) => ({ nickname: str(p.nickname), steamid64: /^\d{17}$/.test(str(p.gameId)) ? str(p.gameId) : "" }));
  const score = isRecord(result) && typeof result.score === "number" ? result.score : 0;
  return { name: str(raw.name), score, players };
}

function json(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

export function parseFaceitMatch(text: string, roomId: string): FaceitMatch | null {
  const body = json(text);
  if (!isRecord(body) || !isRecord(body.payload)) return null;
  const p = body.payload;
  if (str(p.id).toLowerCase() !== roomId || p.status !== "FINISHED") return null;
  const factions = isRecord(p.teams) ? p.teams : {};
  const results = Array.isArray(p.results) && isRecord(p.results[0]) && isRecord(p.results[0].factions) ? p.results[0].factions : {};
  const teams = ["faction1", "faction2"].map((f) => team(factions[f], results[f])).filter((t): t is FaceitTeam => t !== null);
  const maps = (Array.isArray(p.maps) ? p.maps : []).filter(isRecord).map((m) => str(m.class_name)).filter((m) => m !== "");
  const demoUrls = [...new Set((Array.isArray(p.demoURLs) ? p.demoURLs : []).map(str).filter(isDemoResource))];
  return { roomId, map: maps.join(","), startedAt: str(p.startedAt), finishedAt: str(p.finishedAt), demoUrls, teams };
}

export function parseSignedUrl(text: string): string {
  const body = json(text);
  const url = isRecord(body) && isRecord(body.payload) ? str(body.payload.download_url) : "";
  return httpsHost(url, SIGNED_HOSTS) ? url : "";
}

export function faceitSidecarFor(match: FaceitMatch, resourceUrl: string): FaceitSidecar {
  return {
    source: "faceit",
    room_id: match.roomId,
    room_url: roomUrl(match.roomId),
    demo_url: resourceUrl,
    file: demoFileName(resourceUrl),
    map: match.map,
    played_at: match.startedAt,
    finished_at: match.finishedAt,
    teams: match.teams,
  };
}

export function isSidecarOf(text: string, demoName: string): boolean {
  const body = json(text);
  return isRecord(body) && body.source === "faceit" && body.file === demoName;
}
