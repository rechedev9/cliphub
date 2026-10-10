import type { JobSpecSummary } from "./api-types.ts";

const MINUTE = 60;
const HOUR = 3600;
const DAY = 86400;

// "45 s", "12 min", "1 h 5 min". Seconds are dropped from one minute up.
export function formatDuration(seconds: number): string {
  const total = Math.max(0, Math.round(seconds));
  if (total < MINUTE) return `${total} s`;
  const minutes = Math.round(total / MINUTE);
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${hours} h` : `${hours} h ${rest} min`;
}

export function formatBytes(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = Math.max(0, bytes);
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  const digits = unit === 0 || value >= 100 ? 0 : 1;
  return `${value.toFixed(digits).replace(".", ",")} ${units[unit] ?? "B"}`;
}

// "hace 5 s", "hace 3 min", "hace 2 h", "hace 4 d". Future times read as "ahora".
export function formatRelative(at: number, now: number): string {
  const seconds = Math.round((now - at) / 1000);
  if (seconds < 5) return "ahora";
  if (seconds < MINUTE) return `hace ${seconds} s`;
  if (seconds < HOUR) return `hace ${Math.floor(seconds / MINUTE)} min`;
  if (seconds < DAY) return `hace ${Math.floor(seconds / HOUR)} h`;
  return `hace ${Math.floor(seconds / DAY)} d`;
}

export function formatClock(at: number): string {
  return new Date(at).toLocaleTimeString("es-ES", { hour: "2-digit", minute: "2-digit" });
}

export function formatDateTime(at: number): string {
  return new Date(at).toLocaleString("es-ES", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatDate(at: number): string {
  return new Date(at).toLocaleDateString("es-ES", { day: "numeric", month: "long" });
}

function roundMinutes(minutes: number, direction: "down" | "up"): number {
  // Under ten minutes a 5 minute grid would hide most of the range.
  const step = minutes < 10 ? 1 : 5;
  const rounded = direction === "down" ? Math.floor(minutes / step) : Math.ceil(minutes / step);
  return rounded * step;
}

// The start estimate as a range: from the estimate to the estimate plus 35 percent.
export function estimateRangeText(estimatedStartAt: number, now: number): string {
  const delaySeconds = Math.max(0, (estimatedStartAt - now) / 1000);
  if (delaySeconds < MINUTE) return "empieza en menos de un minuto";
  const low = Math.max(1, roundMinutes(delaySeconds / MINUTE, "down"));
  const high = Math.max(low, roundMinutes((delaySeconds * 1.35) / MINUTE, "up"));
  if (high >= 120) {
    return `empieza en ${formatDuration(low * MINUTE)} a ${formatDuration(high * MINUTE)}`;
  }
  return low === high ? `empieza en unos ${low} min` : `empieza en unos ${low} a ${high} min`;
}

export function percentOf(part: number, total: number): number {
  if (total <= 0) return 0;
  return Math.min(100, Math.max(0, Math.round((part / total) * 100)));
}

// Same formula the portal uses for the cost estimate (contract section D).
export function captureSeconds(spec: JobSpecSummary): number {
  if (spec.tickrate <= 0) return 0;
  const ticks = spec.windows.reduce((sum, window) => sum + (window.tickEnd - window.tickStart), 0);
  return Math.round(ticks / spec.tickrate + 2 * spec.windows.length);
}

// Link codes are shown as XXXX-XXXX; users may type them lower case or without the dash.
export function normalizeUserCode(input: string): string | null {
  const compact = input.toUpperCase().replace(/[^A-Z0-9]/g, "");
  if (!/^[ABCDEFGHJKLMNPQRSTUVWXYZ23456789]{8}$/.test(compact)) return null;
  return `${compact.slice(0, 4)}-${compact.slice(4)}`;
}

// Only same-site paths are accepted as a post-login destination.
export function safeCallbackPath(value: string | undefined): string | null {
  if (value === undefined || !value.startsWith("/")) return null;
  if (value.startsWith("//") || value.includes("\\")) return null;
  return value;
}
