import test from 'node:test';
import assert from 'node:assert/strict';
import { cloudJobUrl, cloudUrl, cloudVideoUrl, forwardCloudJSON, relayCloud, streamCloudVideo } from './_lib.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const BASE = 'http://127.0.0.1:8080/api/cloud';

type UpstreamCall = { url: string; init?: RequestInit };

/** Swaps in a fixed orchestrator answer and returns what the proxy asked for. */
async function withUpstream(upstream: () => Response, call: () => Promise<Response>): Promise<{ response: Response; calls: UpstreamCall[] }> {
  const originalFetch = globalThis.fetch;
  const calls: UpstreamCall[] = [];
  globalThis.fetch = async (input, init): Promise<Response> => {
    calls.push({ url: String(input), init });
    return upstream();
  };
  try {
    return { response: await call(), calls };
  } finally {
    globalThis.fetch = originalFetch;
  }
}

function header(init: RequestInit | undefined, name: string): string | null {
  return new Headers(init?.headers).get(name);
}

test('fixed resources map onto the orchestrator cloud API', () => {
  assert.equal(cloudUrl('account'), `${BASE}/account`);
  assert.equal(cloudUrl('link'), `${BASE}/link`);
  assert.equal(cloudUrl('jobs'), `${BASE}/jobs`);
});

test('a job URL is built only for a UUID', () => {
  assert.equal(cloudJobUrl(JOB), `${BASE}/jobs/${JOB}`);
  assert.equal(cloudJobUrl(JOB, '/cancel'), `${BASE}/jobs/${JOB}/cancel`);
  for (const bad of ['', 'abc', '../account', `${JOB}/../../jobs`, `${JOB}?x=1`, `${JOB} `]) {
    assert.equal(cloudJobUrl(bad), null, bad);
  }
});

test('a video URL needs a UUID and a plain mp4 file name', () => {
  assert.equal(cloudVideoUrl(JOB, 'short-01.mp4'), `${BASE}/jobs/${JOB}/videos/short-01.mp4`);
  for (const name of ['', '.mp4', '..', '../state.json', 'a/b.mp4', 'a\\b.mp4', 'cover.jpg', 'short-01.mp4?x=1', 'short 01.mp4', '%2e%2e.mp4', `${'a'.repeat(121)}.mp4`]) {
    assert.equal(cloudVideoUrl(JOB, name), null, name);
  }
  assert.equal(cloudVideoUrl('not-a-job', 'short-01.mp4'), null);
});

test('a Range request reaches the orchestrator and its partial answer comes back intact', async () => {
  const url = cloudVideoUrl(JOB, 'short-01.mp4');
  assert.ok(url);
  const { response, calls } = await withUpstream(
    () =>
      new Response('deo', {
        status: 206,
        headers: { 'content-type': 'video/mp4', 'content-range': 'bytes 2-4/11', 'content-length': '3', 'accept-ranges': 'bytes' },
      }),
    () => streamCloudVideo(url, new Request('http://127.0.0.1:3000/x', { headers: { Range: 'bytes=2-4' } })),
  );
  assert.equal(calls[0]?.url, url);
  assert.equal(header(calls[0]?.init, 'range'), 'bytes=2-4');
  assert.equal(response.status, 206);
  assert.equal(response.headers.get('content-range'), 'bytes 2-4/11');
  assert.equal(response.headers.get('content-length'), '3');
  assert.equal(response.headers.get('accept-ranges'), 'bytes');
  assert.equal(response.headers.get('content-type'), 'video/mp4');
  assert.equal(await response.text(), 'deo');
});

test('a full video answers 200 with its length, and a video not on disk yet is a JSON 404', async () => {
  const url = cloudVideoUrl(JOB, 'short-01.mp4');
  assert.ok(url);
  const full = await withUpstream(
    // Bytes, not a string: a string body would make fetch label the upstream as text/plain.
    () => new Response(new TextEncoder().encode('video-bytes'), { status: 200, headers: { 'content-length': '11', 'accept-ranges': 'bytes' } }),
    () => streamCloudVideo(url, new Request('http://127.0.0.1:3000/x')),
  );
  assert.equal(full.response.status, 200);
  assert.equal(header(full.calls[0]?.init, 'range'), null);
  assert.equal(full.response.headers.get('content-length'), '11');
  assert.equal(full.response.headers.get('content-type'), 'video/mp4');

  const missing = await withUpstream(
    () => Response.json({ error: 'video not ready' }, { status: 404 }),
    () => streamCloudVideo(url, new Request('http://127.0.0.1:3000/x')),
  );
  assert.equal(missing.response.status, 404);
  assert.deepEqual(await missing.response.json(), { error: 'video not ready' });
});

test('portal rejections keep their status and code on the way to the UI', async () => {
  for (const [status, code] of [[429, 'limit_daily'], [403, 'cloud_access_pending'], [503, 'cloud_queue_full'], [409, 'not_linked'], [503, 'portal_unreachable']] as const) {
    const response = await relayCloud(Response.json({ error: 'rejected', code }, { status }));
    assert.equal(response.status, status, code);
    assert.deepEqual(await response.json(), { error: 'rejected', code });
  }
});

test('an unreachable orchestrator is reported as the local service, not as the cloud', async () => {
  const response = await relayCloud(null);
  assert.equal(response.status, 503);
  assert.equal((await response.json()).code, 'service_unavailable');
});

test('accepted answers keep their status: 202 with its document, 204 with no body', async () => {
  const accepted = await relayCloud(Response.json({ id: JOB, status: 'queued' }, { status: 202 }));
  assert.equal(accepted.status, 202);
  assert.deepEqual(await accepted.json(), { id: JOB, status: 'queued' });
  assert.equal(accepted.headers.get('cache-control'), 'no-store');

  const gone = await relayCloud(new Response(null, { status: 204 }));
  assert.equal(gone.status, 204);
  assert.equal(await gone.text(), '');
});

test('a submit is forwarded as JSON, and an oversized body never reaches the orchestrator', async () => {
  const body = JSON.stringify({ job_id: JOB, kind: 'short' });
  const ok = await withUpstream(
    () => Response.json({ id: JOB }, { status: 202 }),
    () => forwardCloudJSON(new Request('http://127.0.0.1:3000/api/cloud/jobs', { method: 'POST', body }), cloudUrl('jobs')),
  );
  assert.equal(ok.calls[0]?.init?.method, 'POST');
  assert.equal(ok.calls[0]?.init?.body, body);
  assert.equal(header(ok.calls[0]?.init, 'content-type'), 'application/json');

  const tooLarge = await withUpstream(
    () => Response.json({}),
    () =>
      forwardCloudJSON(
        new Request('http://127.0.0.1:3000/api/cloud/jobs', { method: 'POST', body: '{}', headers: { 'content-length': String(64 * 1024 * 1024) } }),
        cloudUrl('jobs'),
      ),
  );
  assert.equal(tooLarge.response.status, 413);
  assert.equal(tooLarge.calls.length, 0);
});
