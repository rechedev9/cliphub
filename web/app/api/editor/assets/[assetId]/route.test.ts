import assert from 'node:assert/strict';
import { test } from 'node:test';
import { GET } from './route.ts';

const id = 'e16560b4-6ee2-48a6-8e40-15e5d712856d';
test('restores the filename through the real metadata route without private storage fields', async (t) => {
  t.mock.method(globalThis, 'fetch', async (url: string) => {
    assert.ok(url.endsWith(`/api/editor/assets/${id}`));
    return Response.json({ id, file_name: 'intro.mp4', storage_key: 'private/path', sha256: 'a'.repeat(64) });
  });
  const result = await GET(new Request('http://localhost/'), { params: Promise.resolve({ assetId: id }) });
  assert.equal(result.status, 200);
  assert.equal(result.headers.get('cache-control'), 'private, no-store');
  assert.deepEqual(await result.json(), { id, file_name: 'intro.mp4', sha256: 'a'.repeat(64) });
});

test('metadata rejects invalid IDs before calling the service', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () => new Response());
  const result = await GET(new Request('http://localhost/'), { params: Promise.resolve({ assetId: '../private' }) });
  assert.equal(result.status, 400);
  assert.equal(fetch.mock.callCount(), 0);
});
