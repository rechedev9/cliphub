export type StoredPause =
  | { kind: "note"; text: string }
  | { kind: "auto"; code: string; detail: string; job: { id: string; title: string } | null };

const PAUSE_CODES = new Set([
  "capture_incompatible",
  "steam_unavailable",
  "disk_full",
  "tools_missing",
  "cs2_running",
  "consecutive_failures",
]);

export function encodePause(pause: StoredPause): string {
  return JSON.stringify(pause);
}

export function decodePause(raw: string | null): StoredPause | null {
  if (raw === null || raw.trim() === "") return null;
  const trimmed = raw.trim();
  if (trimmed.startsWith("{")) {
    try {
      const parsed: unknown = JSON.parse(trimmed);
      if (isStoredPause(parsed)) return parsed;
    } catch {
      return { kind: "note", text: trimmed };
    }
  }
  return legacyPause(trimmed);
}

function legacyPause(raw: string): StoredPause {
  const separator = raw.indexOf(":");
  const code = separator === -1 ? raw.trim() : raw.slice(0, separator).trim();
  if (!PAUSE_CODES.has(code)) return { kind: "note", text: raw };
  const rest = separator === -1 ? "" : raw.slice(separator + 1).trim();
  const named = namedJob(rest);
  return { kind: "auto", code, detail: named.rest, job: named.job };
}

function namedJob(detail: string): { job: { id: string; title: string } | null; rest: string } {
  const match = /^job (\S+) "([^"]*)":\s*([\s\S]*)$/.exec(detail);
  if (match === null) return { job: null, rest: detail };
  const [, id = "", title = "", rest = ""] = match;
  return { job: { id, title }, rest };
}

function isStoredPause(value: unknown): value is StoredPause {
  if (typeof value !== "object" || value === null) return false;
  const pause = value as Record<string, unknown>;
  if (pause.kind === "note") return typeof pause.text === "string";
  if (pause.kind !== "auto") return false;
  if (typeof pause.code !== "string" || typeof pause.detail !== "string") return false;
  if (pause.job === null) return true;
  return isJob(pause.job);
}

function isJob(value: unknown): value is { id: string; title: string } {
  if (typeof value !== "object" || value === null) return false;
  const job = value as Record<string, unknown>;
  return typeof job.id === "string" && typeof job.title === "string";
}
