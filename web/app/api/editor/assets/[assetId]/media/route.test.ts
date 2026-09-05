import assert from 'node:assert/strict';
import { test } from 'node:test';
import { GET } from './route.ts';

const id = 'e16560b4-6ee2-48a6-8e40-15e5d712856d';

test('asset preview keeps range semantics and session credentials server-side', async (t) => {
  const calls: { url: string; init: RequestInit | undefined }[] = [];
  t.mock.method(globalThis, 'fetch', async (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    return new Response(new Uint8Array([1, 2, 3]), { status: 206, headers: {
      'Content-Type': 'audio/ogg', 'Content-Range': 'bytes 4-6/10', 'Content-Length': '3',
      'Accept-Ranges': 'bytes', 'Content-Disposition': 'attachment; filename="private-path.ogg"', 'X-ClipHub-Token': 'never-forward',
    } });
  });
  const response = await GET(new Request('http://localhost/api/editor/assets/media', { headers: { Range: 'bytes=4-6' } }), { params: Promise.resolve({ assetId: id }) });
  assert.equal(response.status, 206);
  assert.equal(calls.length, 1);
  assert.ok(calls[0]?.url.endsWith(`/api/editor/assets/${id}/media`));
  assert.equal(new Headers(calls[0]?.init?.headers).get('range'), 'bytes=4-6');
  assert.equal(calls[0]?.init?.redirect, 'manual');
  assert.equal(response.headers.get('content-range'), 'bytes 4-6/10');
  assert.equal(response.headers.get('content-type'), 'audio/ogg');
  assert.equal(response.headers.get('content-disposition'), null);
  assert.equal(response.headers.get('x-cliphub-token'), null);
  assert.deepEqual(new Uint8Array(await response.arrayBuffer()), new Uint8Array([1, 2, 3]));
});

for (const assetId of ['../source.dem', 'not-a-uuid', '']) {
  test(`asset preview rejects invalid ID ${assetId}`, async (t) => {
    const fetch = t.mock.method(globalThis, 'fetch', async () => new Response());
    const response = await GET(new Request('http://localhost/'), { params: Promise.resolve({ assetId }) });
    assert.equal(response.status, 400);
    assert.equal(fetch.mock.callCount(), 0);
  });
}

test('asset preview preserves the service_unavailable error contract', async (t) => {
  t.mock.method(console, 'error', () => undefined);
  t.mock.method(globalThis, 'fetch', async () => { throw new Error('offline'); });
  const response = await GET(new Request('http://localhost/'), { params: Promise.resolve({ assetId: id }) });
  assert.equal(response.status, 503);
  const body: unknown = await response.json();
  assert.deepEqual(body, { code: 'service_unavailable', error: 'analysis service unavailable' });
});
