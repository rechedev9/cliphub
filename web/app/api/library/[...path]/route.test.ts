import assert from 'node:assert/strict';
import { test } from 'node:test';
import { GET, PATCH } from './route.ts';

const DEMO_ID = '11111111-1111-4111-8111-111111111111';
const MATCH_ID = '22222222-2222-4222-8222-222222222222';
const CAPABILITY = 'library-route-test';

process.env.CLIPHUB_PROXY_MUTATION_CAPABILITY = CAPABILITY;

type Handler = (request: Request, context: { params: Promise<{ path: string[] }> }) => Promise<Response>;

/** A same-origin Studio request to /api/library/<path>, as the renderer sends it. */
function call(handler: Handler, { method, path, body }: { method: string; path: string[]; body?: unknown }): Promise<Response> {
  const request = new Request(`http://127.0.0.1:3000/api/library/${path.join('/')}`, {
    method,
    headers: { host: '127.0.0.1:3000', cookie: `cliphub_proxy_capability=${CAPABILITY}`, 'content-type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return handler(request, { params: Promise.resolve({ path }) });
}

test('a demo title saves through the proxy and the saved demo keeps it', async (t) => {
  const sent: unknown[] = [];
  t.mock.method(globalThis, 'fetch', async (_url: string, init?: RequestInit) => {
    sent.push(JSON.parse(String(init?.body)));
    return Response.json({ id: DEMO_ID, tags: [], title: 'Final de la LAN', job_ids: [] });
  });
  const response = await call(PATCH, { method: 'PATCH', path: ['demos', DEMO_ID], body: { title: 'Final de la LAN' } });
  assert.equal(response.status, 200);
  assert.deepEqual(sent, [{ title: 'Final de la LAN' }]);
  assert.deepEqual(await response.json(), { id: DEMO_ID, tags: [], title: 'Final de la LAN', job_ids: [] });
});

test('a match read through the proxy keeps the title of its demo', async (t) => {
  const match = { id: MATCH_ID, demo_id: DEMO_ID, map: 'de_mirage', demo_title: 'Final de la LAN' };
  t.mock.method(globalThis, 'fetch', async () => Response.json(match));
  const response = await call(GET, { method: 'GET', path: ['matches', MATCH_ID] });
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), match);
});
