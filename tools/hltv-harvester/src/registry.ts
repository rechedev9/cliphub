export const DEFAULT_COMMUNITY_URL = "https://cliphub.rechedev.cloud";

export const REGISTRY_BATCH = 500;

const REGISTRY_TIMEOUT_MS = 15_000;

export type RegistryReply = { ok: true; registered: Set<number> } | { ok: false; error: string };

export type RegistryQuery = { base: string; ids: number[]; fetch?: typeof fetch; timeoutMs?: number };

export function communityURL(flag: string, env: NodeJS.ProcessEnv): string {
  const raw = (flag || env.CLIPHUB_COMMUNITY_URL || DEFAULT_COMMUNITY_URL).trim();
  return raw === "off" ? "" : raw.replace(/\/+$/, "");
}

export async function registeredMatches(opts: RegistryQuery): Promise<RegistryReply> {
  const { base, ids } = opts;
  if (base === "") return { ok: false, error: "community API turned off" };
  const call = opts.fetch ?? fetch;
  const registered = new Set<number>();
  const unique = [...new Set(ids)].filter((id) => Number.isInteger(id) && id > 0);
  try {
    for (let i = 0; i < unique.length; i += REGISTRY_BATCH) {
      const batch = unique.slice(i, i + REGISTRY_BATCH);
      const res = await call(`${base}/v1/pro-match-ids?ids=${batch.join(",")}`, { signal: AbortSignal.timeout(opts.timeoutMs ?? REGISTRY_TIMEOUT_MS) });
      if (!res.ok) return { ok: false, error: `community API answered ${res.status}` };
      const body: unknown = await res.json();
      const list = typeof body === "object" && body !== null && "registered" in body ? body.registered : undefined;
      if (!Array.isArray(list)) return { ok: false, error: "community API sent no registered list" };
      for (const id of list) if (typeof id === "number" && batch.includes(id)) registered.add(id);
    }
  } catch (err) {
    return { ok: false, error: String(err) };
  }
  return { ok: true, registered };
}
