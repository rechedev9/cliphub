// RealApiClient.rerenderVideoMusic: the Library music dialog's request contract.
import assert from 'node:assert/strict';
import test from 'node:test';
import { buildEditRequest } from './edit-request.ts';
import { RealApiClient } from './real.ts';
import { DEFAULT_EDIT_CONFIG, DEFAULT_VARIANT, type ReelIntent } from './reel-store.ts';
import type { Video } from './types.ts';

const JOB = '11111111-2222-4333-8444-555555555555';
const RENDER_URL = `/api/demos/${JOB}/renders/${DEFAULT_VARIANT}`;
const SONG = 'song-tikitaka-1';

type Seedable = { intents: Map<string, ReelIntent>; reels: Map<string, Video> };

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

/** A clean reel on this client, as a reload would rehydrate it from localStorage. */
function seedReel(client: RealApiClient, status: Video['status']): string {
  const intent: ReelIntent = {
    videoId: `${JOB}__seg-1`,
    jobId: JOB,
    segmentIds: ['seg-1'],
    mode: 'clean',
    variant: DEFAULT_VARIANT,
    editConfig: { ...DEFAULT_EDIT_CONFIG },
    title: 'Test reel',
    map: 'de_inferno',
    score: '13-7',
    targetName: 'zack',
    createdAt: Date.now(),
  };
  const seedable = client as unknown as Seedable;
  seedable.intents.set(intent.videoId, intent);
  seedable.reels.set(intent.videoId, {
    id: intent.videoId,
    jobId: intent.jobId,
    title: intent.title,
    map: intent.map,
    score: intent.score,
    targetName: intent.targetName,
    mode: intent.mode,
    variant: intent.variant,
    editConfig: intent.editConfig,
    status,
    createdAt: intent.createdAt,
  });
  return intent.videoId;
}

/** Accepts the render POST and reports the re-queued render on the follow-up reconcile. */
function renderFetch(): { posts: Array<{ url: string; body: unknown }>; restore: () => void } {
  const original = globalThis.fetch;
  const posts: Array<{ url: string; body: unknown }> = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? 'GET';
    if (method === 'POST') {
      posts.push({ url, body: JSON.parse(String(init?.body)) });
      if (url === RENDER_URL) return json({ accepted: true }, 202);
    }
    if (method === 'GET' && url === `/api/demos/${JOB}/status`) return json({ status: 'done' });
    if (method === 'GET' && url === RENDER_URL) return json({ status: 'queued' });
    return json({ error: `unexpected ${method} ${url}`, code: 'error' }, 500);
  }) as typeof globalThis.fetch;
  return { posts, restore: () => { globalThis.fetch = original; } };
}

test('a music rerender rejects an unchanged mix and a reel that is not ready without POSTing', async () => {
  const fake = renderFetch();
  try {
    const ready = new RealApiClient();
    const readyId = seedReel(ready, 'ready');
    await assert.rejects(() => ready.rerenderVideoMusic(readyId, {}), /distinto al actual/);

    const composing = new RealApiClient();
    const composingId = seedReel(composing, 'composing');
    await assert.rejects(() => composing.rerenderVideoMusic(composingId, { songId: SONG }), /tiene que estar listo/);

    assert.deepEqual(fake.posts, []);
  } finally {
    fake.restore();
  }
});

test('a music rerender POSTs the new mix with the reel segments and edit once', async () => {
  const fake = renderFetch();
  try {
    const client = new RealApiClient();
    const videoId = seedReel(client, 'ready');
    const video = await client.rerenderVideoMusic(videoId, { songId: SONG, musicVolume: 0.35 });

    assert.deepEqual(fake.posts, [{
      url: RENDER_URL,
      body: {
        music: { key: SONG, volume: 0.35 },
        segment_ids: ['seg-1'],
        edit: JSON.parse(JSON.stringify(buildEditRequest(DEFAULT_EDIT_CONFIG))),
      },
    }]);
    assert.equal(video.mode, 'music');
    assert.equal(video.songId, SONG);
    assert.equal(video.musicVolume, 0.35);
    assert.notEqual(video.status, 'ready');
  } finally {
    fake.restore();
  }
});
