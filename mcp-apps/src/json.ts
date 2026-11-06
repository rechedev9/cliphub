// Tool results arrive as untyped JSON; these readers narrow it without casts.
export type Json = Record<string, unknown>;

export function isRecord(value: unknown): value is Json {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function record(value: unknown): Json {
  return isRecord(value) ? value : {};
}

export function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}

export function num(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

export function list(value: unknown): Json[] {
  return Array.isArray(value) ? value.filter(isRecord) : [];
}
