import test from 'node:test';
import assert from 'node:assert/strict';
import { IMMUTABLE_CACHE_CONTROL, proxyStream } from './_lib.ts';

const UPSTREAM = 'http://127.0.0.1:8080/api/stream-jobs/11111111-1111-4111-8111-111111111111/source';

/** Swaps in a fixed upstream answer and returns what the proxy asked for. */
async function withUpstream(
  upstream: () => Response,
  call: () => Promise<Response>,
): Promise<{ response: Response; init?: RequestInit }> {
  const originalFetch = globalThis.fetch;
  let init: RequestInit | undefined;
  globalThis.fetch = async (_input, requestInit): Promise<Response> => {
    init = requestInit;
    return upstream();
  };
  try {
    return { response: await call(), init };
  } finally {
    globalThis.fetch = originalFetch;
  }
}

test('proxyStream caches only what the caller declares immutable', async () => {
  const cases = [
    {
      name: 'stream source video is cacheable forever on 200',
      upstream: (): Response =>
        new Response('video-bytes', {
          status: 200,
          headers: { 'content-type': 'video/mp4', 'content-length': '11', 'accept-ranges': 'bytes' },
        }),
      cacheControl: IMMUTABLE_CACHE_CONTROL,
      wantStatus: 200,
      wantCacheControl: 'private, max-age=31536000, immutable',
    },
    {
      name: 'a partial range of the source video carries the same policy',
      upstream: (): Response =>
        new Response('deo', {
          status: 206,
          headers: {
            'content-type': 'video/mp4',
            'content-range': 'bytes 2-4/11',
            'accept-ranges': 'bytes',
          },
        }),
      cacheControl: IMMUTABLE_CACHE_CONTROL,
      wantStatus: 206,
      wantCacheControl: 'private, max-age=31536000, immutable',
    },
    {
      name: 'artifacts without a caller policy stay no-store',
      upstream: (): Response =>
        new Response('jpeg-bytes', { status: 200, headers: { 'content-type': 'image/jpeg' } }),
      cacheControl: undefined,
      wantStatus: 200,
      wantCacheControl: 'no-store',
    },
    {
      name: 'a 404 for a source that has not landed yet is never pinned',
      upstream: (): Response =>
        Response.json({ error: 'source not found' }, { status: 404 }),
      cacheControl: IMMUTABLE_CACHE_CONTROL,
      wantStatus: 404,
      wantCacheControl: null,
    },
  ];

  for (const testCase of cases) {
    const { response } = await withUpstream(testCase.upstream, () =>
      proxyStream(UPSTREAM, 'video/mp4', undefined, testCase.cacheControl),
    );
    assert.equal(response.status, testCase.wantStatus, testCase.name);
    if (testCase.wantCacheControl === null) {
      assert.notEqual(response.headers.get('cache-control'), IMMUTABLE_CACHE_CONTROL, testCase.name);
      assert.deepEqual(await response.json(), { error: 'source not found' }, testCase.name);
    } else {
      assert.equal(response.headers.get('cache-control'), testCase.wantCacheControl, testCase.name);
    }
  }
});

test('proxyStream still forwards Range and mirrors the upstream range headers', async () => {
  const { response, init } = await withUpstream(
    () =>
      new Response('deo', {
        status: 206,
        headers: {
          'content-type': 'video/mp4',
          'content-length': '3',
          'content-range': 'bytes 2-4/11',
          'accept-ranges': 'bytes',
        },
      }),
    () =>
      proxyStream(
        UPSTREAM,
        'video/mp4',
        new Request(UPSTREAM, { headers: { range: 'bytes=2-4' } }),
        IMMUTABLE_CACHE_CONTROL,
      ),
  );

  assert.equal((init?.headers as Record<string, string>).range, 'bytes=2-4');
  assert.equal(response.status, 206);
  assert.equal(response.headers.get('content-range'), 'bytes 2-4/11');
  assert.equal(response.headers.get('accept-ranges'), 'bytes');
  assert.equal(response.headers.get('content-length'), '3');
  assert.equal(await response.text(), 'deo');
});
