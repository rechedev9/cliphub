export const FACEIT = "https://www.faceit.com";

const ROOM_ID = /1-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi;
const DEMO_NAME = /^1-[0-9a-f-]{36}(?:-\d+)+\.dem\.zst$/;
const RESOURCE_HOST = ".faceit-cdn.net";
const SIGNED_HOSTS = [".backblazeb2.com", ".faceit-cdn.net"];

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

export type SignResult = { kind: "signed"; url: string } | { kind: "needs_user" } | { kind: "failed"; status: number };

export function matchScript(roomId: string): string {
  return `fetch(${JSON.stringify(matchApiPath(roomId))}, { credentials: 'include' }).then((r) => r.text())`;
}

export function roomIds(text: string): string[] {
  const out: string[] = [];
  for (const hit of text.matchAll(ROOM_ID)) {
    const id = hit[0].toLowerCase();
    if (!out.includes(id)) out.push(id);
  }
  return out;
}

function matchApiPath(roomId: string): string {
  return `/api/match/v2/match/${roomId}`;
}

export function roomUrl(roomId: string): string {
  return `${FACEIT}/en/cs2/room/${roomId}`;
}

export function demoFileName(resourceUrl: string): string {
  const name = new URL(resourceUrl).pathname.split("/").pop() ?? "";
  return DEMO_NAME.test(name) ? name : "";
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

function team(raw: unknown, result: unknown): FaceitTeam | null {
  if (!isRecord(raw)) return null;
  const roster = Array.isArray(raw.roster) ? raw.roster : [];
  const players = roster.filter(isRecord).map((p) => ({ nickname: str(p.nickname), steamid64: /^\d{17}$/.test(str(p.gameId)) ? str(p.gameId) : "" }));
  const score = isRecord(result) && typeof result.score === "number" ? result.score : 0;
  return { name: str(raw.name), score, players };
}

export function parseFaceitMatch(text: unknown, roomId: string): FaceitMatch | null {
  if (typeof text !== "string") return null;
  let body: unknown;
  try {
    body = JSON.parse(text);
  } catch {
    return null;
  }
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

export function signScript(resourceUrl: string): string {
  const body = JSON.stringify(JSON.stringify({ resource_url: resourceUrl }));
  return `location.origin !== '${FACEIT}' ? Promise.resolve({ status: -1, body: '' }) : fetch('/api/download/v2/demos/download-url', { method: 'POST', credentials: 'include', headers: { 'content-type': 'application/json' }, body: ${body} }).then(async (r) => ({ status: r.status, body: await r.text() }))`;
}

export function parseSigned(raw: unknown): SignResult {
  if (!isRecord(raw) || typeof raw.status !== "number") return { kind: "failed", status: 0 };
  if (raw.status === 401 || raw.status === 403) return { kind: "needs_user" };
  if (raw.status !== 200) return { kind: "failed", status: raw.status };
  try {
    const body: unknown = JSON.parse(str(raw.body));
    const url = isRecord(body) && isRecord(body.payload) ? str(body.payload.download_url) : "";
    return httpsHost(url, SIGNED_HOSTS) ? { kind: "signed", url } : { kind: "failed", status: 200 };
  } catch {
    return { kind: "failed", status: 200 };
  }
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
