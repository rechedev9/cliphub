// RealApiClient regression: a ready render that disagrees with the intent, or a
// durable POST rejection, must never make the reconcile tick re-POST forever.
import assert from 'node:assert/strict';
import test from 'node:test';
import { RealApiClient } from './real.ts';
import { buildEditRequest } from './edit-request.ts';
import { MISMATCH_REDRIVE_FAILURE_REASON } from './failure-reason.ts';
import { DEFAULT_VARIANT, DEFAULT_EDIT_CONFIG, FULL_DEMO_REEL_SUFFIX, type ReelIntent } from './reel-store.ts';
import type { EditConfig, Video } from './types.ts';

const RECORDING_UNCONFIGURED_ERROR =
  'recording is not configured on this machine; set ZV_RECORDER_PATH, ZV_HLAE_PATH and ZV_CS2_PATH and restart the orchestrator';
const EVERY_SEGMENT = ['seg-001', 'seg-002', 'seg-003', 'seg-004', 'seg-005', 'seg-006'];
const RECAP_EDIT: EditConfig = { ...DEFAULT_EDIT_CONFIG, format: 'landscape-16x9', matchRecap: true, nativeHud: true };

type Call = { url: string; method: string; body?: Record<string, unknown> };

/** Mutable orchestrator truth for one job; tests move it between ticks. */
type Orchestrator = {
  jobStatus: string;
  /** GET /renders/{variant} body, or null for 404 (nothing rendered yet). */
  render: Record<string, unknown> | null;
  generate: () => Response;
  renderPost: () => Response;
};

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

function readyRender(artifactPrefix: string, segmentIds: string[], edit: EditConfig = DEFAULT_EDIT_CONFIG): Record<string, unknown> {
  return { status: 'ready', artifact_prefix: artifactPrefix, videos: ['video.mp4'], segment_ids: segmentIds, edit: buildEditRequest(edit) };
}

/** Fakes fetch for one job and records every call; restore in `finally`. */
function fakeOrchestrator(jobId: string, state: Orchestrator): { calls: Call[]; posts: (path: string) => Call[]; restore: () => void } {
  const original = globalThis.fetch;
  const calls: Call[] = [];
  const base = `/api/demos/${jobId}`;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? 'GET';
    const call: Call = { url, method };
    // Trust boundary: the client serialised this body itself.
    if (typeof init?.body === 'string') call.body = JSON.parse(init.body) as Record<string, unknown>;
    calls.push(call);
    if (url === `${base}/status`) return json({ status: state.jobStatus });
    if (url === `${base}/renders/${DEFAULT_VARIANT}` && method === 'GET') {
      return state.render ? json(state.render) : json({ error: 'render variant not found' }, 404);
    }
    if (url === `${base}/generate` && method === 'POST') return state.generate();
    if (url === `${base}/renders/${DEFAULT_VARIANT}` && method === 'POST') return state.renderPost();
    throw new Error(`unexpected fetch: ${method} ${url}`);
  }) as typeof globalThis.fetch;
  return {
    calls,
    posts: (path) => calls.filter((c) => c.method === 'POST' && c.url === `${base}${path}`),
    restore: () => { globalThis.fetch = original; },
  };
}

/** One Library poll plus the fire-and-forget drive() it may start. */
async function tick(client: RealApiClient): Promise<void> {
  await client.listVideos();
  await new Promise((resolve) => setTimeout(resolve, 0));
  await new Promise((resolve) => setTimeout(resolve, 0));
}

// Seeding the private maps is the only way to reconcile without a live scan/parse flow.
type Seedable = { intents: Map<string, ReelIntent>; reels: Map<string, Video> };

function seedReel(client: RealApiClient, jobId: string, recap = false): ReelIntent {
  const intent: ReelIntent = {
    videoId: recap ? `${jobId}__${FULL_DEMO_REEL_SUFFIX}` : `${jobId}__seg-001`,
    jobId,
    segmentIds: recap ? [] : ['seg-001'],
    mode: 'clean',
    variant: DEFAULT_VARIANT,
    editConfig: recap ? { ...RECAP_EDIT } : { ...DEFAULT_EDIT_CONFIG },
    title: 'Test reel',
    map: 'de_dust2',
    score: '13-7',
    targetName: 'zack',
    createdAt: Date.now(),
  };
  const seedable = client as unknown as Seedable;
  seedable.intents.set(intent.videoId, intent);
  seedable.reels.set(intent.videoId, {
    id: intent.videoId,
    jobId,
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
  return intent;
}

test('a mismatching ready render re-drives once, fails on the next mismatching revision, and one retry re-drives once more', async () => {
  const state: Orchestrator = {
    jobStatus: 'recorded',
    render: readyRender('rev-1', EVERY_SEGMENT),
    generate: () => json({ id: 'job-1', task: 'record' }, 202),
    renderPost: () => json({ accepted: true }, 202),
  };
  const fake = fakeOrchestrator('job-1', state);
  try {
    const client = new RealApiClient();
    const { videoId } = seedReel(client, 'job-1');
    const status = async (): Promise<Video | null> => client.getVideo(videoId);

    await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'first mismatch re-drives /generate once');
    assert.deepEqual(fake.posts('/generate')[0]?.body?.segment_ids, ['seg-001'], 'the re-drive carries the selection');

    await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'the same revision waits, no second POST');

    state.render = readyRender('rev-2', EVERY_SEGMENT);
    await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'a second mismatching revision latches instead of POSTing');
    assert.equal((await status())?.status, 'failed');
    assert.equal((await status())?.failureReason, MISMATCH_REDRIVE_FAILURE_REASON);

    state.render = readyRender('rev-3', EVERY_SEGMENT);
    await tick(client);
    await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'latched: further revisions and ticks never POST');
    assert.equal((await status())?.status, 'failed');

    await client.retryVideo(videoId);
    await tick(client);
    assert.equal(fake.posts('/generate').length, 2, 'one explicit retry issues exactly one more POST');
    assert.notEqual((await status())?.status, 'failed', 'the retry lifts the latch');

    state.render = readyRender('rev-4', ['seg-001']);
    await tick(client);
    assert.equal((await status())?.status, 'ready', 'a matching revision after the retry is ready');
    assert.equal(fake.posts('/generate').length, 2);
  } finally {
    fake.restore();
  }
});

test('a durable 409 latches failed; only an explicit retry re-POSTs', async () => {
  const state: Orchestrator = {
    jobStatus: 'parsed',
    render: null,
    generate: () => json({ error: RECORDING_UNCONFIGURED_ERROR }, 409),
    renderPost: () => json({ accepted: true }, 202),
  };
  const fake = fakeOrchestrator('job-2', state);
  try {
    const client = new RealApiClient();
    const { videoId } = seedReel(client, 'job-2');
    for (let i = 0; i < 5; i++) await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'idle polling never re-POSTs a durable rejection');
    const reel = await client.getVideo(videoId);
    assert.equal(reel?.status, 'failed');
    assert.equal(reel?.failureReason, RECORDING_UNCONFIGURED_ERROR);

    await client.retryVideo(videoId);
    for (let i = 0; i < 3; i++) await tick(client);
    assert.equal(fake.posts('/generate').length, 2, 'retry POSTs once, then the fresh 409 latches again');
    assert.equal((await client.getVideo(videoId))?.status, 'failed');
  } finally {
    fake.restore();
  }
});

test('a transient 503 on the re-drive does not consume it', async () => {
  let generateStatus = 503;
  const state: Orchestrator = {
    jobStatus: 'recorded',
    render: readyRender('rev-1', EVERY_SEGMENT),
    generate: () => (generateStatus === 503 ? json({ code: 'service_unavailable' }, 503) : json({}, 202)),
    renderPost: () => json({ accepted: true }, 202),
  };
  const fake = fakeOrchestrator('job-3', state);
  try {
    const client = new RealApiClient();
    seedReel(client, 'job-3');
    await tick(client);
    assert.equal(fake.posts('/generate').length, 1);
    generateStatus = 202;
    await tick(client);
    assert.equal(fake.posts('/generate').length, 2, 'the 503 did not record a re-drive; the next tick POSTs again');
    await tick(client);
    assert.equal(fake.posts('/generate').length, 2, 'once accepted, the same revision waits');
  } finally {
    fake.restore();
  }
});

test('a matching ready render is ready without any POST', async () => {
  const cases = [
    { name: 'Short with its selection', recap: false, segmentIds: ['seg-001'], edit: DEFAULT_EDIT_CONFIG },
    { name: 'Full Demo recap rendering every round', recap: true, segmentIds: EVERY_SEGMENT, edit: RECAP_EDIT },
  ];
  for (const tc of cases) {
    const state: Orchestrator = {
      jobStatus: 'done',
      render: readyRender('rev-1', tc.segmentIds, tc.edit),
      generate: () => json({}, 202),
      renderPost: () => json({ accepted: true }, 202),
    };
    const fake = fakeOrchestrator('job-4', state);
    try {
      const client = new RealApiClient();
      const { videoId } = seedReel(client, 'job-4', tc.recap);
      await tick(client);
      await tick(client);
      assert.equal(fake.calls.filter((c) => c.method === 'POST').length, 0, `${tc.name}: no POST`);
      assert.equal((await client.getVideo(videoId))?.status, 'ready', `${tc.name}: ready`);
    } finally {
      fake.restore();
    }
  }
});

test('retrying a failed render POSTs /renders with the selection for a Short and without one for a recap', async () => {
  const cases = [
    { name: 'Short', recap: false, wantSegmentIds: ['seg-001'] },
    { name: 'Full Demo recap', recap: true, wantSegmentIds: undefined },
  ];
  for (const tc of cases) {
    const state: Orchestrator = {
      jobStatus: 'recorded',
      render: { status: 'failed', error: 'ffmpeg exited with status 1' },
      generate: () => json({}, 202),
      renderPost: () => json({ accepted: true }, 202),
    };
    const fake = fakeOrchestrator('job-5', state);
    try {
      const client = new RealApiClient();
      const { videoId } = seedReel(client, 'job-5', tc.recap);
      await client.retryVideo(videoId);
      const renderPosts = fake.posts(`/renders/${DEFAULT_VARIANT}`);
      assert.equal(renderPosts.length, 1, `${tc.name}: one render POST`);
      assert.equal(fake.posts('/generate').length, 0, `${tc.name}: a failed render is not recaptured`);
      const body = renderPosts[0]?.body ?? {};
      assert.deepEqual(body.segment_ids, tc.wantSegmentIds, `${tc.name}: segment_ids`);
      assert.equal('segment_ids' in body, tc.wantSegmentIds !== undefined, `${tc.name}: key presence`);
    } finally {
      fake.restore();
    }
  }
});

test('a failed render whose `error` says the capture is not reusable recaptures instead of re-rendering', async () => {
  // The orchestrator's RenderVariantState serialises its failure as `error`,
  // not `failure_reason`; reading the wrong key hid every recapture hint.
  const state: Orchestrator = {
    jobStatus: 'recorded',
    render: { status: 'failed', error: 'recording_not_reusable: capture fingerprint predates observer-steamid-input-v2' },
    generate: () => json({}, 202),
    renderPost: () => json({ accepted: true }, 202),
  };
  const fake = fakeOrchestrator('job-6', state);
  try {
    const client = new RealApiClient();
    const { videoId } = seedReel(client, 'job-6');
    await tick(client);
    // The reconcile tick reads the hint and re-drives capture on its own; a
    // plain ffmpeg failure would have parked the reel on FALLO instead.
    assert.equal(fake.posts('/generate').length, 1, 'recapture goes through /generate');
    assert.equal(fake.posts(`/renders/${DEFAULT_VARIANT}`).length, 0, 'no render-only POST for a dead capture');
    const reel = (await client.listVideos()).find((v) => v.id === videoId);
    assert.equal(reel?.status, 'recording');
  } finally {
    fake.restore();
  }
});
