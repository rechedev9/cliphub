import { parseErrorCode } from "./parse.ts";

export type FetchResult<T> =
  | { ok: true; data: T }
  | { ok: false; code: string; detail: string | null };

function validate<T>(body: unknown, parse: (value: unknown) => T): FetchResult<T> {
  try {
    return { ok: true, data: parse(body) };
  } catch (err) {
    return { ok: false, code: "bad_response", detail: err instanceof Error ? err.message : null };
  }
}

export function statusCode(status: number, body: unknown): string {
  if (status === 401) return "unauthorized";
  if (status === 404) return "not_found";
  return parseErrorCode(body) ?? `http_${status}`;
}

export async function fetchJson<T>(url: string, parse: (value: unknown) => T): Promise<FetchResult<T>> {
  let res: Response;
  try {
    res = await fetch(url, { cache: "no-store", headers: { Accept: "application/json" } });
  } catch {
    return { ok: false, code: "network", detail: null };
  }
  const body: unknown = await res.json().catch(() => null);
  if (!res.ok) return { ok: false, code: statusCode(res.status, body), detail: null };
  return validate(body, parse);
}
