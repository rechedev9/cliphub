const DEFAULT_MAX_JSON_BYTES = 262144;

// Errors are { error, code }. Callers read the code; the text is for logs and curl.
export function jsonError(status: number, code: string): Response {
  return Response.json({ error: code, code }, { status });
}

// A refusal a store function hands back for its route to send as is.
export interface ApiFailure {
  ok: false;
  status: number;
  code: string;
  error: string;
  extra?: Record<string, unknown>;
}

export function apiFailure(status: number, code: string): ApiFailure {
  return { ok: false, status, code, error: code };
}

export function failureResponse(failure: ApiFailure): Response {
  return Response.json(
    { error: failure.error, code: failure.code, ...failure.extra },
    { status: failure.status },
  );
}

export function unauthorized(): Response {
  return Response.json({ error: "unauthorized" }, { status: 401 });
}

export function notFound(): Response {
  return jsonError(404, "not_found");
}

export function leaseLost(): Response {
  return jsonError(409, "lease_lost");
}

export function invalidState(): Response {
  return jsonError(409, "invalid_state");
}

export function badRequest(): Response {
  return jsonError(400, "invalid_request");
}

// Reads a JSON body without trusting its size. undefined means it was too large or not JSON.
export async function readJson(
  request: Request,
  maxBytes: number = DEFAULT_MAX_JSON_BYTES,
): Promise<unknown> {
  if (!request.body) return undefined;
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > maxBytes) {
      await reader.cancel();
      return undefined;
    }
    chunks.push(value);
  }
  try {
    const parsed: unknown = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    return parsed;
  } catch {
    return undefined;
  }
}

// The declared body length, or null when the header is missing or not a whole number.
export function contentLength(request: Request): number | null {
  const raw = request.headers.get("content-length");
  if (raw === null || !/^\d+$/.test(raw)) return null;
  const value = Number(raw);
  return Number.isSafeInteger(value) ? value : null;
}

// The address the reverse proxy saw. Caddy overwrites the header, so the last entry is ours.
export function clientIp(request: Request): string {
  const forwarded = request.headers.get("x-forwarded-for") ?? "";
  const last = forwarded.split(",").pop()?.trim();
  return last ? last : "unknown";
}

// A numeric query parameter, or null when it is absent or not a number.
export function numberParam(raw: string | null): number | null {
  if (raw === null || raw.trim() === "") return null;
  const value = Number(raw);
  return Number.isFinite(value) ? value : null;
}
