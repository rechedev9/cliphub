import test from 'node:test';
import assert from 'node:assert/strict';
import { RealStreamsApiClient } from './streams.ts';

test('stream render admission sends the approved edit-plan revision', async () => {
  const originalFetch = globalThis.fetch;
  let request: { input: RequestInfo | URL; init?: RequestInit } | undefined;
  globalThis.fetch = async (input, init): Promise<Response> => {
    request = { input, init };
    return Response.json({ status: 'queued', videos: [] }, { status: 202 });
  };

  try {
    const revision = '2026-07-28T10:00:00.123Z';
    await new RealStreamsApiClient().startRender(
      '11111111-1111-4111-8111-111111111111',
      'streamer-vertical-stack-40-60',
      revision,
    );
    assert.ok(request);
    assert.equal(request.init?.method, 'POST');
    assert.deepEqual(request.init?.headers, { 'Content-Type': 'application/json' });
    assert.deepEqual(JSON.parse(String(request.init?.body)), {
      expected_edit_plan_updated_at: revision,
    });
  } finally {
    globalThis.fetch = originalFetch;
  }
});
