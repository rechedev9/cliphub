import test from 'node:test';
import assert from 'node:assert/strict';
import {
  IMMUTABLE_CACHE_CONTROL,
  callOrchestrator,
  ifNoneMatchInit,
  listCacheHeaders,
  notModifiedFromUpstream,
  proxyStream,
} from './_lib.ts';

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

test('proxyStream keeps a caller immutable policy over upstream must-revalidate', async () => {
  const { response } = await withUpstream(
    () =>
      new Response('video-bytes', {
        status: 200,
        headers: {
          'content-type': 'video/mp4',
          'content-length': '11',
          'cache-control': 'private, max-age=0, must-revalidate',
          'last-modified': 'Thu, 10 Sep 2026 12:00:00 GMT',
        },
      }),
    () => proxyStream(UPSTREAM, 'video/mp4', undefined, IMMUTABLE_CACHE_CONTROL),
  );

  assert.equal(response.status, 200);
  assert.equal(response.headers.get('cache-control'), IMMUTABLE_CACHE_CONTROL);
  assert.equal(response.headers.get('last-modified'), 'Thu, 10 Sep 2026 12:00:00 GMT');
});

test('proxyStream copies upstream cache validators and prefers upstream Cache-Control', async () => {
  const { response, init } = await withUpstream(
    () =>
      new Response('jpeg-bytes', {
        status: 200,
        headers: {
          'content-type': 'image/jpeg',
          'content-length': '10',
          'cache-control': 'private, max-age=0, must-revalidate',
          'last-modified': 'Thu, 10 Sep 2026 12:00:00 GMT',
          etag: '"cover1"',
        },
      }),
    () =>
      proxyStream(
        UPSTREAM,
        'image/jpeg',
        new Request(UPSTREAM, {
          headers: { 'If-Modified-Since': 'Thu, 10 Sep 2026 12:00:00 GMT', 'If-None-Match': '"cover1"' },
        }),
      ),
  );

  const headers = init?.headers as Record<string, string>;
  assert.equal(headers['if-modified-since'], 'Thu, 10 Sep 2026 12:00:00 GMT');
  assert.equal(headers['if-none-match'], '"cover1"');
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('cache-control'), 'private, max-age=0, must-revalidate');
  assert.equal(response.headers.get('last-modified'), 'Thu, 10 Sep 2026 12:00:00 GMT');
  assert.equal(response.headers.get('etag'), '"cover1"');
});

test('proxyStream mirrors a 304 instead of turning it into an error', async () => {
  const { response } = await withUpstream(
    () =>
      new Response(null, {
        status: 304,
        headers: {
          'cache-control': 'private, max-age=0, must-revalidate',
          'last-modified': 'Thu, 10 Sep 2026 12:00:00 GMT',
          etag: '"cover1"',
        },
      }),
    () =>
      proxyStream(
        UPSTREAM,
        'image/jpeg',
        new Request(UPSTREAM, { headers: { 'If-Modified-Since': 'Thu, 10 Sep 2026 12:00:00 GMT' } }),
      ),
  );

  assert.equal(response.status, 304);
  assert.equal(response.headers.get('cache-control'), 'private, max-age=0, must-revalidate');
  assert.equal(response.headers.get('last-modified'), 'Thu, 10 Sep 2026 12:00:00 GMT');
  assert.equal(response.headers.get('etag'), '"cover1"');
  assert.equal(await response.text(), '');
});

test('ifNoneMatchInit forwards only a present validator', () => {
  assert.equal(ifNoneMatchInit(new Request('http://127.0.0.1/api/demos/jobs')), undefined);
  const init = ifNoneMatchInit(new Request('http://127.0.0.1/api/demos/jobs', {
    headers: { 'If-None-Match': 'W/"jobs1"' },
  }));
  assert.deepEqual(init, { headers: { 'If-None-Match': 'W/"jobs1"' } });
});

test('notModifiedFromUpstream mirrors a 304 ETag and ignores a 200', () => {
  assert.equal(notModifiedFromUpstream(new Response('ok', { status: 200 })), null);
  const mirrored = notModifiedFromUpstream(new Response(null, {
    status: 304,
    headers: { ETag: 'W/"jobs1"' },
  }));
  assert.equal(mirrored?.status, 304);
  assert.equal(mirrored?.headers.get('ETag'), 'W/"jobs1"');
  assert.equal(mirrored?.headers.get('Cache-Control'), 'private, no-cache');
});

test('conditional poll survives the orchestrator transport as a 304 without an error', async (t) => {
  const errors = t.mock.method(console, 'error', () => {});
  const { response, init } = await withUpstream(
    () => new Response(null, { status: 304, headers: { ETag: 'W/"jobs1"' } }),
    async () => {
      const upstream = await callOrchestrator(UPSTREAM, {
        headers: { 'If-None-Match': 'W/"jobs1"' },
      });
      assert.ok(upstream);
      const mirrored = notModifiedFromUpstream(upstream);
      assert.ok(mirrored, '304 must reach the conditional poll handler');
      return mirrored;
    },
  );
  assert.equal(response.status, 304);
  assert.equal(response.headers.get('ETag'), 'W/"jobs1"');
  assert.equal(response.headers.get('Cache-Control'), 'private, no-cache');
  assert.equal(await response.text(), '');
  assert.equal((init?.headers as Record<string, string>)['If-None-Match'], 'W/"jobs1"');
  assert.equal(init?.redirect, 'manual');
  assert.equal(errors.mock.callCount(), 0);
});

test('orchestrator transport still rejects redirects without exposing Location', async (t) => {
  t.mock.method(console, 'error', () => {});
  for (const status of [301, 302, 303, 307, 308]) {
    const { response, init } = await withUpstream(
      () => new Response(null, { status, headers: { Location: 'https://example.invalid/private' } }),
      async () => {
        const response = await callOrchestrator(UPSTREAM);
        assert.ok(response);
        return response;
      },
    );
    assert.equal(response.status, 502, `redirect ${status}`);
    assert.equal(response.headers.get('Location'), null);
    assert.equal(init?.redirect, 'manual');
  }
});

test('listCacheHeaders copies the orchestrator ETag onto a rewritten body', () => {
  const headers = listCacheHeaders(new Response('{}', { headers: { ETag: 'W/"jobs1"' } }));
  assert.equal(headers.get('ETag'), 'W/"jobs1"');
  assert.equal(headers.get('Cache-Control'), 'private, no-cache');
});
