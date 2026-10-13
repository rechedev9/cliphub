import { statusCode } from "./fetch-json.ts";

export type ActionResult = { ok: true; data: unknown } | { ok: false; code: string };

export interface ActionRequest {
  url: string;
  body?: Record<string, unknown>;
  method?: "POST" | "DELETE";
}

// Sends a same-origin mutation; the browser adds the Origin header the API requires.
export async function postAction(request: ActionRequest): Promise<ActionResult> {
  const method = request.method ?? "POST";
  let res: Response;
  try {
    res = await fetch(request.url, {
      method,
      headers: method === "POST" ? { "Content-Type": "application/json" } : undefined,
      body: method === "POST" ? JSON.stringify(request.body ?? {}) : undefined,
    });
  } catch {
    return { ok: false, code: "network" };
  }
  const data: unknown = res.status === 204 ? null : await res.json().catch(() => null);
  if (!res.ok) return { ok: false, code: statusCode(res.status, data) };
  return { ok: true, data };
}
