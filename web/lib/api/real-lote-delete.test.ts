import assert from 'node:assert/strict';
import test from 'node:test';
import { RealApiClient } from './real.ts';
import { DEFAULT_EDIT_CONFIG, DEFAULT_VARIANT, type ReelIntent } from './reel-store.ts';
import { recoveredVideo, type RecoveredVideo } from './recovered-renders.ts';
import type { Video } from './types.ts';

const JOB = '11111111-2222-4333-8444-555555555555';
const RENDER_URL = `/api/demos/${JOB}/renders/${DEFAULT_VARIANT}`;
const LOTE = ['seg-001', 'seg-004', 'seg-007'];
const LOTE_ID = `${JOB}__auto__${LOTE.join('_')}`;

type Seedable = { intents: Map<string, ReelIntent>; reels: Map<string, Video>; recovered: Map<string, RecoveredVideo> };
type Disk = { videos: string[]; segmentIds: string[]; separateShorts: boolean };

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

function seedIntent(client: RealApiClient, options: { id: string; segmentIds: string[]; separateShorts: boolean }): void {
  const intent: ReelIntent = {
    videoId: options.id,
    jobId: JOB,
    segmentIds: options.segmentIds,
    mode: 'clean',
    variant: DEFAULT_VARIANT,
    editConfig: { ...DEFAULT_EDIT_CONFIG, separateShorts: options.separateShorts },
    title: 'Test reel',
    map: 'de_mirage',
    score: '13-7',
    createdAt: Date.now(),
  };
  const seedable = client as unknown as Seedable;
  seedable.intents.set(intent.videoId, intent);
  seedable.reels.set(intent.videoId, {
    id: intent.videoId,
    jobId: JOB,
    title: intent.title,
    map: intent.map,
    score: intent.score,
    mode: intent.mode,
    variant: intent.variant,
    editConfig: intent.editConfig,
    status: 'ready',
    createdAt: intent.createdAt,
  });
}

function diskFetch(disk: Disk): { deletes: string[]; restore: () => void } {
  const original = globalThis.fetch;
  const deletes: string[] = [];
  const prefix = `${RENDER_URL}/videos/`;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? 'GET';
    if (method === 'GET' && url === RENDER_URL) {
      const edit = {
        format: 'short-9x16', kill_effect: 'punch-in', transition: 'flash', cover_strategy: 'generated-gameplay',
        intro: false, outro: false, hook_text: false, kill_counter: false, separate_shorts: disk.separateShorts,
      };
      return json({ status: 'ready', videos: disk.videos, segment_ids: disk.segmentIds, edit });
    }
    if (method === 'DELETE' && url.startsWith(prefix)) {
      const name = url.slice(prefix.length);
      deletes.push(name);
      disk.videos = disk.videos.filter((video) => video !== name);
      return new Response(null, { status: 204 });
    }
    return json({ error: `unexpected ${method} ${url}`, code: 'error' }, 500);
  }) as typeof globalThis.fetch;
  return { deletes, restore: () => { globalThis.fetch = original; } };
}

function recoveredOnReload(disk: Disk): RecoveredVideo | null {
  const render = {
    status: 'ready',
    videoName: disk.videos[0],
    segmentIds: disk.segmentIds,
    editConfig: { ...DEFAULT_EDIT_CONFIG, separateShorts: disk.separateShorts },
  };
  return recoveredVideo({ jobId: JOB, status: 'done' }, DEFAULT_VARIANT, render, new Set());
}

test('Borrar on a lote card deletes every Short of the lote and it does not come back on reload', async () => {
  const disk: Disk = { videos: [...LOTE], segmentIds: LOTE, separateShorts: true };
  const fake = diskFetch(disk);
  try {
    const client = new RealApiClient();
    seedIntent(client, { id: LOTE_ID, segmentIds: LOTE, separateShorts: true });
    await client.deleteVideo(LOTE_ID);
    assert.deepEqual([...fake.deletes].sort(), LOTE);
    assert.equal((client as unknown as Seedable).intents.has(LOTE_ID), false);
    assert.equal(recoveredOnReload(disk), null);
  } finally { fake.restore(); }
});

test('Borrar on a recovered lote card deletes every Short of the lote', async () => {
  const disk: Disk = { videos: [...LOTE], segmentIds: LOTE, separateShorts: true };
  const fake = diskFetch(disk);
  try {
    const recovered = recoveredOnReload(disk);
    assert.ok(recovered);
    const client = new RealApiClient();
    (client as unknown as Seedable).recovered.set(recovered.video.id, recovered);
    await client.deleteVideo(recovered.video.id);
    assert.deepEqual([...fake.deletes].sort(), LOTE);
    assert.equal(recoveredOnReload(disk), null);
  } finally { fake.restore(); }
});

test('Borrar on a lote leaves the render alone when the style now holds another video', async () => {
  const disk: Disk = { videos: ['seg-009'], segmentIds: ['seg-009'], separateShorts: false };
  const fake = diskFetch(disk);
  try {
    const client = new RealApiClient();
    seedIntent(client, { id: LOTE_ID, segmentIds: LOTE, separateShorts: true });
    await client.deleteVideo(LOTE_ID);
    assert.deepEqual(fake.deletes, []);
    assert.deepEqual(disk.videos, ['seg-009']);
    assert.equal((client as unknown as Seedable).intents.has(LOTE_ID), false);
  } finally { fake.restore(); }
});

test('Borrar on a Short that is not part of a lote deletes only its video', async () => {
  const disk: Disk = { videos: ['seg-002'], segmentIds: ['seg-002'], separateShorts: false };
  const fake = diskFetch(disk);
  try {
    const client = new RealApiClient();
    seedIntent(client, { id: `${JOB}__seg-002`, segmentIds: ['seg-002'], separateShorts: false });
    await client.deleteVideo(`${JOB}__seg-002`);
    assert.deepEqual(fake.deletes, ['seg-002']);
    assert.equal((client as unknown as Seedable).intents.has(`${JOB}__seg-002`), false);
  } finally { fake.restore(); }
});
