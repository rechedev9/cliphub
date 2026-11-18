import { orchestratorUrl, callOrchestrator, serviceUnavailable } from '../../../demos/_lib';

export const runtime = 'nodejs';

export async function GET(
  _request: Request,
  context: { params: Promise<{ code: string }> },
): Promise<Response> {
  const { code } = await context.params;
  if (!/^[a-z]{2}$/.test(code)) return new Response(null, { status: 404 });
  const res = await callOrchestrator(`${orchestratorUrl()}/api/faceit/flags/${code}`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return new Response(null, { status: 404 });
  return new Response(res.body, {
    status: 200,
    headers: {
      'content-type': 'image/svg+xml',
      'x-content-type-options': 'nosniff',
      'content-security-policy': "default-src 'none'; style-src 'unsafe-inline'",
      'cache-control': 'public, max-age=86400',
    },
  });
}
