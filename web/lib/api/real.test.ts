// RealApiClient request shape for one Short/Full POV constructor beat, and the
// batch-status rows the server could not read.
//
// The constructor asks for the match, the Short plays and the recap rounds at
// the same time. Those three used to poll /status each and fetch /plan twice in
// a three-deep waterfall; here they must cost one /status plus a single wave of
// the documents they actually need, all derived from one parsed kill plan.
import assert from 'node:assert/strict';
import test from 'node:test';
import { RealApiClient } from './real.ts';
import { DEFAULT_VARIANT, DEFAULT_EDIT_CONFIG, type ReelIntent } from './reel-store.ts';
import type { Video } from './types.ts';

const JOB = '11111111-2222-4333-8444-555555555555';
const STATUS_URL = `/api/demos/${JOB}/status`;
const PLAN_URL = `/api/demos/${JOB}/plan`;
const RECAP_URL = `/api/demos/${JOB}/recap-plan`;
const ROSTER_URL = `/api/demos/${JOB}/roster`;

/** Two segments plus a same-timeline duplicate, so the dedup rule governs the count. */
const PLAN = {
  demo: { map: 'de_inferno' },
  target: { steamid64: '76561198000000000', name_in_demo: 'zack', team_at_start: 'T' },
  stats: { total_kills_target: 5 },
  segments: [
    { id: 'seg-1', round: 3, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }, { weapon: 'ak47' }] },
    { id: 'seg-1-again', round: 3, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }] },
    { id: 'seg-2', round: 7, tick_start: 900, tick_end: 980, kills: [{ weapon: 'awp' }] },
  ],
};

const RECAP_PLAN = {
  demo: { map: 'de_inferno' },
  segments: [
    { id: 'round-1', round: 1, tick_start: 1, tick_end: 50 },
    { id: 'round-2', round: 2, tick_start: 60, tick_end: 120 },
  ],
};

const ROSTER = {
  players: [
    { steamid64: '76561198000000000', name: 'zack', team: 'T', kills: 24, deaths: 12, assists: 4, adr: 91.5 },
    { steamid64: '76561198000000001', name: 'other', team: 'CT', kills: 3, deaths: 20, assists: 1 },
  ],
  match: { map: 'de_inferno', score_ct: 7, score_t: 13, rounds: 20 },
};

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

/** Lets every microtask chain behind a released response run to completion. */
async function drain(): Promise<void> {
  for (let i = 0; i < 3; i++) await new Promise((resolve) => setTimeout(resolve, 0));
}

/**
 * A fetch that holds every request open until `release()`, so each call to it
 * returns exactly the wave of URLs that were in flight together. That is what
 * makes the waterfall observable: a request that only opens after an earlier
 * response landed shows up in a later wave.
 */
function gateFetch(reply: (url: string) => Response): {
  calls: string[];
  release: () => Promise<string[]>;
  restore: () => void;
} {
  const original = globalThis.fetch;
  const calls: string[] = [];
  let open: Array<{ url: string; resolve: (res: Response) => void }> = [];
  globalThis.fetch = (async (input: string | URL | Request) => {
    const url = String(input);
    calls.push(url);
    return new Promise<Response>((resolve) => {
      open.push({ url, resolve });
    });
  }) as typeof globalThis.fetch;
  return {
    calls,
    release: async () => {
      const wave = open;
      open = [];
      for (const request of wave) request.resolve(reply(request.url));
      await drain();
      return wave.map((request) => request.url).sort();
    },
    restore: () => {
      globalThis.fetch = original;
    },
  };
}

function planReadyReply(url: string): Response {
  if (url === STATUS_URL) return json({ status: 'done' });
  if (url === PLAN_URL) return json(PLAN);
  if (url === RECAP_URL) return json(RECAP_PLAN);
  if (url === ROSTER_URL) return json(ROSTER);
  return json({ error: `unexpected fetch: ${url}`, code: 'error' }, 500);
}

test('a plan-ready constructor beat reads /status once and its documents in one wave', async () => {
  const gate = gateFetch(planReadyReply);
  try {
    const client = new RealApiClient();
    const beat = Promise.all([client.getMatch(JOB), client.findClips(JOB), client.findRecapClips(JOB)]);

    assert.deepEqual(await gate.release(), [STATUS_URL], 'the three calls share one status read');
    assert.deepEqual(
      await gate.release(),
      [PLAN_URL, RECAP_URL, ROSTER_URL].sort(),
      'plan, recap-plan and roster open together',
    );
    assert.deepEqual(await gate.release(), [], 'no third hop: the waterfall is gone');

    const [match, plays, rounds] = await beat;
    assert.equal(gate.calls.length, 4);
    assert.equal(gate.calls.filter((url) => url === STATUS_URL).length, 1);
    assert.equal(gate.calls.filter((url) => url === PLAN_URL).length, 1, 'the kill plan is fetched once');
    // The Match and the Plays come from that one parsed plan; the duplicate
    // timeline segment is dropped in both.
    assert.deepEqual(plays.map((play) => play.id), ['seg-1', 'seg-2']);
    assert.equal(match?.decentPlays, 2);
    assert.equal(match?.map, 'Inferno');
    assert.equal(match?.player, 'zack');
    assert.equal(match?.status, 'done');
    assert.equal(match?.stats.kills, 24, 'stats come from the roster row, not the plan');
    assert.deepEqual(rounds.map((round) => round.id), ['round-1', 'round-2']);
  } finally {
    gate.restore();
  }
});

test('a beat on a demo without a kill plan reads only /status and /roster', async () => {
  const gate = gateFetch((url) => (url === STATUS_URL ? json({ status: 'scanned' }) : planReadyReply(url)));
  try {
    const client = new RealApiClient();
    const beat = Promise.all([client.getMatch(JOB), client.findClips(JOB), client.findRecapClips(JOB)]);

    assert.deepEqual(await gate.release(), [STATUS_URL]);
    assert.deepEqual(await gate.release(), [ROSTER_URL], 'no plan or recap-plan before the demo is parsed');
    assert.deepEqual(await gate.release(), []);

    const [match, plays, rounds] = await beat;
    assert.equal(match?.status, 'scanned');
    assert.equal(match?.map, 'Inferno');
    assert.deepEqual(plays, []);
    assert.deepEqual(rounds, []);
  } finally {
    gate.restore();
  }
});

test('per-document failures stay in their own call', async () => {
  const cases: Array<{
    name: string;
    broken: string;
    wantMatch: 'ok' | 'throws';
    wantPlays: 'ok' | 'throws';
    wantRounds: 'ok' | 'throws';
  }> = [
    { name: 'recap-plan', broken: RECAP_URL, wantMatch: 'ok', wantPlays: 'ok', wantRounds: 'throws' },
    { name: 'roster', broken: ROSTER_URL, wantMatch: 'ok', wantPlays: 'ok', wantRounds: 'ok' },
    { name: 'plan', broken: PLAN_URL, wantMatch: 'throws', wantPlays: 'throws', wantRounds: 'ok' },
    { name: 'status', broken: STATUS_URL, wantMatch: 'throws', wantPlays: 'throws', wantRounds: 'throws' },
  ];
  for (const tc of cases) {
    const original = globalThis.fetch;
    globalThis.fetch = (async (input: string | URL | Request) => {
      const url = String(input);
      if (url === tc.broken) return json({ error: 'boom', code: 'internal' }, 500);
      return planReadyReply(url);
    }) as typeof globalThis.fetch;
    try {
      const client = new RealApiClient();
      const settled = await Promise.allSettled([client.getMatch(JOB), client.findClips(JOB), client.findRecapClips(JOB)]);
      const got = settled.map((result) => (result.status === 'fulfilled' ? 'ok' : 'throws'));
      assert.deepEqual(got, [tc.wantMatch, tc.wantPlays, tc.wantRounds], `broken ${tc.name}`);
      // A missing roster still yields a match, named after the plan's target.
      if (tc.broken === ROSTER_URL && settled[0].status === 'fulfilled') {
        assert.equal(settled[0].value?.player, 'zack');
        assert.equal(settled[0].value?.stats.kills, 5, 'plan fallback stats');
      }
    } finally {
      globalThis.fetch = original;
    }
  }
});

test('a later beat re-reads the plan instead of serving the previous one', async () => {
  const gate = gateFetch(planReadyReply);
  try {
    const client = new RealApiClient();
    const first = client.findClips(JOB);
    await gate.release();
    await gate.release();
    await first;
    // Re-picking a POV rewrites the plan while /status returns to a plan-ready
    // status, so nothing may survive a settled beat.
    const second = client.findClips(JOB);
    assert.deepEqual(await gate.release(), [STATUS_URL]);
    assert.deepEqual(await gate.release(), [PLAN_URL]);
    await second;
    assert.equal(gate.calls.filter((url) => url === PLAN_URL).length, 2);
  } finally {
    gate.restore();
  }
});

type Seedable = { intents: Map<string, ReelIntent>; reels: Map<string, Video> };

/** A queued reel on this client, as a reload would rehydrate it from localStorage. */
function seedReel(client: RealApiClient): string {
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
    status: 'queued',
    createdAt: intent.createdAt,
  });
  return intent.videoId;
}

/** One Library reconcile beat, including the record/render POST it may fire. */
async function reconcileTick(client: RealApiClient): Promise<void> {
  await client.listVideos();
  await drain();
}

function fakeBatch(item: () => Record<string, unknown>): { calls: Array<{ url: string; method: string }>; restore: () => void } {
  const original = globalThis.fetch;
  const calls: Array<{ url: string; method: string }> = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? 'GET';
    calls.push({ url, method });
    if (url.startsWith('/api/demos/batch-status')) return json({ items: [item()] });
    if (url === `/api/demos/${JOB}/generate` && method === 'POST') return json({ id: 'job', task: 'record' }, 202);
    return json({ error: `unexpected fetch: ${method} ${url}`, code: 'error' }, 500);
  }) as typeof globalThis.fetch;
  return { calls, restore: () => { globalThis.fetch = original; } };
}

test('an unreadable batch-status row changes nothing about the reel', async () => {
  const cases: Array<{ name: string; item: Record<string, unknown> }> = [
    {
      // The job read fine but its render state did not. Without the guard the
      // missing render half reads as "no render yet", which on a parsed job
      // fires an unrequested capture.
      name: 'render half unreadable',
      item: { job_id: JOB, variant: DEFAULT_VARIANT, job: { status: 'parsed' }, render: null, error: 'read render state: unexpected end of JSON input' },
    },
    {
      name: 'whole row unreadable',
      item: { job_id: JOB, variant: DEFAULT_VARIANT, job: null, render: null, error: 'get job status: database is locked' },
    },
    {
      name: 'error carried as an object',
      item: { job_id: JOB, variant: DEFAULT_VARIANT, job: null, render: null, error: { code: 'internal', message: 'read render state' } },
    },
  ];
  for (const tc of cases) {
    const fake = fakeBatch(() => tc.item);
    try {
      const client = new RealApiClient();
      const videoId = seedReel(client);
      await reconcileTick(client);
      await reconcileTick(client);
      const reel = await client.getVideo(videoId);
      assert.equal(reel?.status, 'queued', `${tc.name}: view is untouched`);
      assert.equal(reel?.unrecoverable, undefined, `${tc.name}: no job-gone latch`);
      assert.equal(fake.calls.filter((call) => call.method === 'POST').length, 0, `${tc.name}: no POST`);
      assert.equal(
        fake.calls.filter((call) => call.url === PLAN_URL).length,
        0,
        `${tc.name}: nothing else is read either`,
      );
    } finally {
      fake.restore();
    }
  }
});

test('unreadable rows do not accumulate job-gone strikes', async () => {
  let item: Record<string, unknown> = {
    job_id: JOB,
    variant: DEFAULT_VARIANT,
    job: null,
    render: null,
    error: 'get job status: database is locked',
  };
  const fake = fakeBatch(() => item);
  try {
    const client = new RealApiClient();
    const videoId = seedReel(client);
    await reconcileTick(client);
    await reconcileTick(client);
    // A real 404 still needs its own two consecutive misses to latch.
    item = { job_id: JOB, variant: DEFAULT_VARIANT, job: null, render: null };
    await reconcileTick(client);
    assert.equal((await client.getVideo(videoId))?.unrecoverable, undefined, 'one 404 must not latch');
    await reconcileTick(client);
    assert.equal((await client.getVideo(videoId))?.unrecoverable, true, 'two 404s latch as before');
  } finally {
    fake.restore();
  }
});

for (const status of ['parsing', 'parsed']) {
  test(`getScan reports ${status} targeted imports without requiring a roster`, async () => {
    const gate = gateFetch(url => url === STATUS_URL ? json({ status }) : json({ error: 'roster not ready' }, 409));
    try {
      const scan = new RealApiClient().getScan(JOB);
      assert.deepEqual(await gate.release(), [STATUS_URL]);
      assert.deepEqual(await gate.release(), []);
      assert.deepEqual(await scan, { status, players: [] });
    } finally {
      gate.restore();
    }
  });
}
