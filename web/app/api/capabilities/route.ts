import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../demos/_lib';

export const runtime = 'nodejs';

/** GET /api/capabilities — capture-readiness snapshot; only known fields are forwarded. */
export async function GET(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/capabilities`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  type UpstreamTool = { name?: string; source?: string; configured?: boolean; accessible?: boolean; path?: string };
  const data = (await res.json()) as {
    record?: { enabled?: boolean; tools?: UpstreamTool[] };
    render?: { enabled?: boolean; tools?: UpstreamTool[] };
    compose?: { enabled?: boolean };
    faceit?: { enabled?: boolean };
    steam?: { enabled?: boolean; gcConfigured?: boolean; historyConfigured?: boolean };
  };
  // Forward only the fields the UI uses; drop each tool's absolute disk `path` so
  // local filesystem layout never leaves the box, even if this bind is exposed.
  const tools = (list?: UpstreamTool[]) =>
    (list ?? []).map((t) => ({ name: t.name ?? '', source: t.source ?? 'none', configured: Boolean(t.configured), accessible: Boolean(t.accessible) }));
  return NextResponse.json(
    {
      record: { enabled: Boolean(data.record?.enabled), tools: tools(data.record?.tools) },
      render: { enabled: Boolean(data.render?.enabled), tools: tools(data.render?.tools) },
      compose: { enabled: Boolean(data.compose?.enabled) },
      faceit: { enabled: Boolean(data.faceit?.enabled) },
      steam: {
        enabled: Boolean(data.steam?.enabled),
        gcConfigured: Boolean(data.steam?.gcConfigured ?? data.steam?.enabled),
        historyConfigured: Boolean(data.steam?.historyConfigured),
      },
    },
    { headers: { 'cache-control': 'no-store' } },
  );
}
