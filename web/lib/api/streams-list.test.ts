import assert from 'node:assert/strict';
import test from 'node:test';
import { RealStreamsApiClient } from './streams.ts';

test('listJobs reuses the last body when the stream list answers 304', async () => {
  const listed = [{ id: 's1', status: 'ready', created_at: '2026-09-06T12:00:00Z', clip_count: 2 }];
  const original = globalThis.fetch;
  const headers: Array<string | null> = [];
  let calls = 0;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    if (url !== '/api/streams') return new Response(JSON.stringify({ error: `unexpected ${url}` }), { status: 500 });
    calls += 1;
    headers.push(new Headers(init?.headers).get('If-None-Match'));
    if (calls === 1) {
      return new Response(JSON.stringify({ jobs: listed }), {
        status: 200,
        headers: { 'content-type': 'application/json', ETag: 'W/"streams1"' },
      });
    }
    return new Response(null, { status: 304, headers: { ETag: 'W/"streams1"' } });
  }) as typeof globalThis.fetch;
  try {
    const client = new RealStreamsApiClient();
    const first = await client.listJobs();
    const second = await client.listJobs();
    assert.equal(calls, 2);
    assert.equal(headers[0], null);
    assert.equal(headers[1], 'W/"streams1"');
    assert.equal(first, second);
    assert.equal(first[0]?.clip_count, 2);
  } finally {
    globalThis.fetch = original;
  }
});
