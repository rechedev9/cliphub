import assert from 'node:assert/strict';
import test from 'node:test';
import { ifNoneMatchInit, readConditionalJSON } from './conditional-json.ts';

test('ifNoneMatchInit sends the validator only when an ETag is known', () => {
  assert.deepEqual(ifNoneMatchInit(undefined), { cache: 'no-store', headers: {} });
  assert.deepEqual(ifNoneMatchInit('W/"abc"'), {
    cache: 'no-store',
    headers: { 'If-None-Match': 'W/"abc"' },
  });
});

test('readConditionalJSON reuses the cached list on 304', async () => {
  const cached = { etag: 'W/"abc"', value: [{ jobId: '1' }] };
  const next = await readConditionalJSON(
    new Response(null, { status: 304, headers: { ETag: 'W/"abc"' } }),
    cached,
    async () => {
      throw new Error('304 must not parse a body');
    },
  );
  assert.equal(next.value, cached.value);
  assert.equal(next.etag, 'W/"abc"');
});

test('readConditionalJSON stores a fresh 200 body and ETag', async () => {
  const next = await readConditionalJSON(
    new Response(JSON.stringify({ jobs: [{ jobId: '2' }] }), {
      status: 200,
      headers: { ETag: 'W/"def"', 'content-type': 'application/json' },
    }),
    null,
    async (res) => ((await res.json()) as { jobs: Array<{ jobId: string }> }).jobs,
  );
  assert.deepEqual(next.value, [{ jobId: '2' }]);
  assert.equal(next.etag, 'W/"def"');
});

test('readConditionalJSON rejects a 304 with no prior body', async () => {
  await assert.rejects(
    () => readConditionalJSON(new Response(null, { status: 304 }), null, async () => []),
    /304 without a cached body/,
  );
});
