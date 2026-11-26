// RealApiClient regression: a ready render that disagrees with the intent, or a
// durable POST rejection, must never make the reconcile tick re-POST forever.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { RealApiClient } from './real.ts';
import { buildEditRequest } from './edit-request.ts';
import { CHANGE_NOT_APPLIED_REASON, MISMATCH_REDRIVE_FAILURE_REASON, parseFailureReason } from './failure-reason.ts';
import { DEFAULT_VARIANT, DEFAULT_EDIT_CONFIG, FULL_DEMO_REEL_SUFFIX, coerceIntents, videoFromIntent, withAdmissionFailure, type AdmissionFailure, type ReelIntent } from './reel-store.ts';
import type { EditConfig, Video } from './types.ts';
import { FULL_DEMO_CAPTURE_VARIANT, currentFullDemoOptions, fullDemoPlanEdit, isFullDemoSnapshot } from '../full-demo-plan.ts';

const RECORDING_UNCONFIGURED_ERROR =
  'recording is not configured on this machine; set ZV_RECORDER_PATH, ZV_HLAE_PATH and ZV_CS2_PATH and restart the orchestrator';
const EVERY_SEGMENT = ['seg-001', 'seg-002', 'seg-003', 'seg-004', 'seg-005', 'seg-006'];
const RECAP_EDIT: EditConfig = { ...DEFAULT_EDIT_CONFIG, format: 'landscape-16x9', matchRecap: true, nativeHud: true };

type Call = { url: string; method: string; body?: Record<string, unknown> };

/** Mutable orchestrator truth for one job; tests move it between ticks. */
type Orchestrator = {
  /** The render variant the reel reads; unset is DEFAULT_VARIANT. */
  variant?: string;
  jobStatus: string;
  /** The job's failure_reason while it reads failed. */
  jobFailure?: string;
  generateRun?: Record<string, unknown>;
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
    if (url === `${base}/status`) return json({ status: state.jobStatus, failure_reason: state.jobFailure, generate: state.generateRun });
    const renders = `${base}/renders/${state.variant ?? DEFAULT_VARIANT}`;
    if (url === renders && method === 'GET') {
      return state.render ? json(state.render) : json({ error: 'render variant not found' }, 404);
    }
    if (url === `${base}/generate` && method === 'POST') return state.generate();
    if (url === renders && method === 'POST') return state.renderPost();
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
type Seedable = { intents: Map<string, ReelIntent>; reels: Map<string, Video>; driveLatch: Map<string, AdmissionFailure> };

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

test('a recapture that failed and kept the earlier recording shows that render with the failure, and one retry records again', async () => {
  const drift = 'recorder failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001';
  const state: Orchestrator = {
    jobStatus: 'recorded',
    render: readyRender('rev-1', EVERY_SEGMENT),
    generate: () => json({ id: 'job-9', task: 'record' }, 202),
    renderPost: () => json({ accepted: true }, 202),
  };
  const fake = fakeOrchestrator('job-9', state);
  try {
    const client = new RealApiClient();
    const { videoId } = seedReel(client, 'job-9');

    await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'the changed reel re-drives /generate once');

    state.generateRun = { variant: DEFAULT_VARIANT, active: true };
    await tick(client);
    assert.equal((await client.getVideo(videoId))?.status, 'queued', 'an accepted run waits in the queue');

    state.generateRun = { variant: 'full-hud-60', active: false, failure_reason: drift, failure_code: 'capture_flake' };
    await tick(client);
    assert.equal((await client.getVideo(videoId))?.status, 'queued', 'a failed run of another variant is not this reel');

    const failedRun = (): Record<string, unknown> => ({ variant: DEFAULT_VARIANT, active: false, failure_reason: drift, failure_code: 'capture_flake', failed_at: new Date().toISOString() });
    state.generateRun = failedRun();
    await tick(client);
    await tick(client);
    const kept = await client.getVideo(videoId);
    assert.equal(kept?.status, 'ready', 'the earlier render stays on show');
    assert.equal(kept?.changePending, true);
    assert.equal(kept?.failureReason, drift);
    assert.equal(kept?.failureCode, 'capture_flake');
    assert.ok(kept?.downloadUrl, 'the earlier video stays playable');
    assert.equal(fake.posts('/generate').length, 1, 'a failed capture never records again by itself');

    await new Promise((resolve) => setTimeout(resolve, 5));
    await client.retryVideo(videoId);
    await tick(client);
    assert.equal(fake.posts('/generate').length, 2, 'one retry issues exactly one more POST');
    const retried = await client.getVideo(videoId);
    assert.equal(retried?.status, 'queued', 'a status read from before the accepted retry is not its failure');
    assert.equal(retried?.failureReason, undefined);
    assert.equal(retried?.changePending, undefined);

    state.generateRun = { variant: DEFAULT_VARIANT, active: true };
    await tick(client);
    assert.equal((await client.getVideo(videoId))?.status, 'queued');

    state.generateRun = failedRun();
    await tick(client);
    assert.equal((await client.getVideo(videoId))?.failureReason, drift, 'the retried capture failed too');
    assert.equal(fake.posts('/generate').length, 2);
  } finally {
    fake.restore();
  }
});

test('a reel with no render stops after its capture failed and kept an earlier recording, and one retry records again', async () => {
  const drift = 'recorder failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001';
  const state: Orchestrator = {
    jobStatus: 'recorded',
    render: null,
    generate: () => json({ id: 'job-10', task: 'record' }, 202),
    renderPost: () => json({ accepted: true }, 202),
  };
  const fake = fakeOrchestrator('job-10', state);
  try {
    const client = new RealApiClient();
    const { videoId } = seedReel(client, 'job-10');

    await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'the reel asks for its capture once');

    state.jobStatus = 'recording';
    await tick(client);
    state.jobStatus = 'recorded';
    state.generateRun = { variant: DEFAULT_VARIANT, active: false, failure_reason: drift, failure_code: 'capture_flake', failed_at: new Date().toISOString() };
    for (let i = 0; i < 3; i++) await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'a failed capture never records again by itself');
    const failed = await client.getVideo(videoId);
    assert.equal(failed?.status, 'failed');
    assert.equal(failed?.failureReason, drift);
    assert.equal(failed?.failureCode, 'capture_flake');

    await new Promise((resolve) => setTimeout(resolve, 5));
    await client.retryVideo(videoId);
    await tick(client);
    assert.equal(fake.posts('/generate').length, 2, 'one retry issues exactly one more POST');
    assert.equal((await client.getVideo(videoId))?.status, 'recording', 'the failure from before the retry is not its failure');
  } finally {
    fake.restore();
  }
});

test('an earlier failure of the variant does not stop the first capture of a reel, and that capture failing is told at once', async () => {
  const failedRun = (): Record<string, unknown> => ({ variant: DEFAULT_VARIANT, active: false, failure_reason: 'recorder crashed', failed_at: new Date().toISOString() });
  const state: Orchestrator = {
    jobStatus: 'recorded',
    render: null,
    generateRun: failedRun(),
    generate: () => json({ id: 'job-11', task: 'record' }, 202),
    renderPost: () => json({ accepted: true }, 202),
  };
  const fake = fakeOrchestrator('job-11', state);
  try {
    const client = new RealApiClient();
    const { videoId } = seedReel(client, 'job-11');

    await new Promise((resolve) => setTimeout(resolve, 5));
    await tick(client);
    assert.equal(fake.posts('/generate').length, 1, 'a reel that never recorded still asks for its capture');
    await tick(client);
    assert.equal((await client.getVideo(videoId))?.status, 'recording');

    state.generateRun = failedRun();
    await tick(client);
    assert.equal((await client.getVideo(videoId))?.status, 'failed', 'a capture that fails before the page saw it recording is told at once');
    assert.equal(fake.posts('/generate').length, 1);

    await new Promise((resolve) => setTimeout(resolve, 5));
    await client.retryVideo(videoId);
    assert.equal(fake.posts('/generate').length, 2, 'a retry right after that failure records at once, without waiting out the admission');
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

test('a reel read back after a reload still matches its ready render, so CS2 does not record it again', async () => {
  const cases: Array<{ name: string; edit: EditConfig }> = [
    {
      name: 'Crear horizontal of a Kill Feed Short',
      edit: {
        ...DEFAULT_EDIT_CONFIG, format: 'landscape-16x9', killEffect: 'clean', transition: 'cut', captureHud: 'deathnotices',
        portraitSafeKillfeed: true, voiceComms: true, voiceVolume: 0.6, subtitles: true, subtitlesHidden: ['line-b', 'line-a'],
      },
    },
    {
      name: 'one-player Short with player cards, subtitles and a pre-roll',
      edit: {
        ...DEFAULT_EDIT_CONFIG, preRollSeconds: 8, centeredKillfeed: true, voiceComms: true, subtitles: true,
        openingCard: 'neon', endCard: 'neon', coverCard: 'neon',
      },
    },
    { name: '16:9 one-player summary in the clean style', edit: { ...DEFAULT_EDIT_CONFIG, format: 'landscape-16x9', captureHud: 'clean', preRollSeconds: 5 } },
  ];
  for (const tc of cases) {
    const state: Orchestrator = {
      jobStatus: 'done',
      render: readyRender('rev-1', ['seg-001'], tc.edit),
      generate: () => json({}, 202),
      renderPost: () => json({ accepted: true }, 202),
    };
    const fake = fakeOrchestrator('job-7', state);
    try {
      const client = new RealApiClient();
      const seeded = seedReel(client, 'job-7');
      // The intent as Mis vídeos reads it back from localStorage after a reload.
      const [reloaded] = coerceIntents(JSON.parse(JSON.stringify([{ ...seeded, editConfig: tc.edit }])));
      assert.ok(reloaded, tc.name);
      (client as unknown as Seedable).intents.set(reloaded.videoId, reloaded);
      await tick(client);
      await tick(client);
      assert.equal(fake.calls.filter((c) => c.method === 'POST').length, 0, `${tc.name}: no POST`);
      assert.equal((await client.getVideo(reloaded.videoId))?.status, 'ready', `${tc.name}: ready`);
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
    // The reconcile tick reads the hint and asks /generate, which records only if the
    // orchestrator allows it; a plain ffmpeg failure would have parked the reel on FALLO.
    assert.equal(fake.posts('/generate').length, 1, 'recapture goes through /generate');
    assert.equal(fake.posts('/generate')[0]?.body?.allow_capture, false, 'a page load forbids the capture');
    assert.equal(fake.posts(`/renders/${DEFAULT_VARIANT}`).length, 0, 'no render-only POST for a dead capture');
    const reel = (await client.listVideos()).find((v) => v.id === videoId);
    assert.equal(reel?.status, 'recording');
  } finally {
    fake.restore();
  }
});

/** The approved Full Demo of the planner fixture; TrueView changes its plan and so its hash. */
const NOT_REUSABLE = 'recording_not_reusable: capture fingerprint predates observer-steamid-input-v2';
const RENDER_ONLY_CHANGE: EditConfig = { ...DEFAULT_EDIT_CONFIG, intro: true };

/** Waiting for the click that may record is no failure: the last render stays under Listos with the notice. */
function assertWaitsForClick(card: Video | null, name: string): void {
  assert.equal(card?.status, 'ready', `${name}: not counted as failed`);
  assert.ok(card?.downloadUrl, `${name}: the last render stays playable and downloadable`);
  assert.equal(card?.changePending, true, `${name}: the card says the change is not applied`);
  assert.equal(card?.failureReason, undefined, name);
}

function fullDemoEdit(trueview: boolean): EditConfig {
  const value: unknown = JSON.parse(readFileSync(new URL('../full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(value));
  value.document.options = currentFullDemoOptions(value.document.options);
  if (trueview) {
    value.document.options.capture.trueview = true;
    value.document.plan_hash = 'b'.repeat(64);
    value.approval.approved_plan_hash = value.document.plan_hash;
  }
  return fullDemoPlanEdit(value);
}

test('a reload never records with CS2 to apply a stored change: the last render stays ready with the notice, and Reintentar applies it', async () => {
  const killfeed: EditConfig = { ...DEFAULT_EDIT_CONFIG, format: 'landscape-16x9', captureHud: 'deathnotices', portraitSafeKillfeed: true };
  const cases: Array<{ name: string; rendered: EditConfig; wanted: EditConfig; wantedIds?: string[] }> = [
    { name: 'capture style', rendered: killfeed, wanted: { ...killfeed, captureHud: 'clean' } },
    { name: 'kill feed framing', rendered: killfeed, wanted: { ...killfeed, portraitSafeKillfeed: false } },
    { name: 'pre-roll', rendered: DEFAULT_EDIT_CONFIG, wanted: { ...DEFAULT_EDIT_CONFIG, preRollSeconds: 8 } },
    { name: 'victim replay', rendered: DEFAULT_EDIT_CONFIG, wanted: { ...DEFAULT_EDIT_CONFIG, victimReplay: true } },
    { name: 'a play the capture lacks', rendered: DEFAULT_EDIT_CONFIG, wanted: DEFAULT_EDIT_CONFIG, wantedIds: ['seg-001', 'seg-002'] },
    { name: 'Full Demo with TrueView', rendered: fullDemoEdit(false), wanted: fullDemoEdit(true) },
  ];
  for (const tc of cases) {
    const full = tc.wanted.fullDemo !== undefined;
    const variant = tc.wanted.format === 'landscape-16x9' ? FULL_DEMO_CAPTURE_VARIANT : DEFAULT_VARIANT;
    const state: Orchestrator = {
      variant,
      jobStatus: 'done',
      render: readyRender('rev-1', full ? EVERY_SEGMENT : ['seg-001'], tc.rendered),
      generate: () => json({ id: 'job-8', task: 'record' }, 202),
      renderPost: () => json({ accepted: true }, 202),
    };
    const fake = fakeOrchestrator('job-8', state);
    try {
      const client = new RealApiClient();
      const segmentIds = tc.wantedIds ?? (full ? [] : ['seg-001']);
      const stored = {
        videoId: `job-8__${full ? FULL_DEMO_REEL_SUFFIX : segmentIds.join('_')}`, jobId: 'job-8', segmentIds, mode: 'clean', variant,
        editConfig: tc.wanted, title: 'Test reel', map: 'de_dust2', score: '13-7', targetName: 'zack', createdAt: 1,
      };
      // The intent as Mis vídeos reads it back from localStorage after a reload.
      const [reloaded] = coerceIntents(JSON.parse(JSON.stringify([stored])));
      assert.ok(reloaded, tc.name);
      const seedable = client as unknown as Seedable;
      seedable.intents.set(reloaded.videoId, reloaded);
      seedable.reels.set(reloaded.videoId, videoFromIntent(reloaded));
      await tick(client);
      await tick(client);
      assert.equal(fake.calls.filter((c) => c.method === 'POST').length, 0, `${tc.name}: a page load sends nothing that can record`);
      assertWaitsForClick(await client.getVideo(reloaded.videoId), tc.name);

      const retried = await client.retryVideo(reloaded.videoId);
      assert.equal(fake.posts('/generate').length, 1, `${tc.name}: Reintentar applies the change`);
      assert.equal(retried.changePending, undefined, `${tc.name}: the notice goes once the change is on its way`);
    } finally {
      fake.restore();
    }
  }
});

test('a page load never lets /generate record: capture_required keeps the last render ready, or fails with none, and Reintentar records', async () => {
  const cases: Array<{ name: string; jobStatus: string; jobFailure?: string; render: Record<string, unknown> | null; edit?: EditConfig }> = [
    { name: 'a render-only change over freed clips', jobStatus: 'done', render: readyRender('rev-1', ['seg-001']), edit: RENDER_ONLY_CHANGE },
    { name: 'a capture no longer reusable under the last render', jobStatus: 'failed', jobFailure: NOT_REUSABLE, render: readyRender('rev-1', ['seg-001']), edit: RENDER_ONLY_CHANGE },
    { name: 'a capture no longer reusable', jobStatus: 'done', render: { status: 'failed', error: NOT_REUSABLE } },
    { name: 'no render of the variant yet', jobStatus: 'parsed', render: null },
  ];
  for (const tc of cases) {
    const state: Orchestrator = {
      jobStatus: tc.jobStatus,
      jobFailure: tc.jobFailure,
      render: tc.render,
      generate: () => json({ code: 'capture_required', error: 'applying this change needs a CS2 capture' }, 409),
      renderPost: () => json({ accepted: true }, 202),
    };
    const fake = fakeOrchestrator('job-9', state);
    try {
      const client = new RealApiClient();
      const intent = seedReel(client, 'job-9');
      if (tc.edit) intent.editConfig = tc.edit;
      await tick(client);
      await tick(client);
      const asked = fake.posts('/generate');
      assert.equal(asked.length, 1, `${tc.name}: one ask, then the card waits`);
      assert.equal(asked[0]?.body?.allow_capture, false, `${tc.name}: a page load forbids a capture`);
      assert.equal(fake.posts(`/renders/${DEFAULT_VARIANT}`).length, 0, `${tc.name}: no render-only POST either`);
      const card = await client.getVideo(intent.videoId);
      if (tc.render?.status === 'ready') {
        assertWaitsForClick(card, tc.name);
      } else {
        assert.equal(card?.status, 'failed', `${tc.name}: with no last render the card is the failure`);
        assert.equal(card?.failureReason, CHANGE_NOT_APPLIED_REASON, `${tc.name}: the card says the change is not applied`);
        assert.equal(parseFailureReason(card?.failureReason).retryCanHelp, true, `${tc.name}: Reintentar is offered`);
      }

      state.generate = () => json({ id: 'job-9', task: 'record' }, 202);
      await client.retryVideo(intent.videoId);
      const sent = fake.posts('/generate');
      assert.equal(sent.length, 2, `${tc.name}: Reintentar asks again`);
      assert.equal(sent[1]?.body !== undefined && 'allow_capture' in sent[1].body, false, `${tc.name}: a click may record`);
    } finally {
      fake.restore();
    }
  }
});

test('a stored refusal or a page load task the worker refused keeps the last render ready without asking again', async () => {
  const refused = 'capture_required: applying this change needs a CS2 capture, which only an explicit request starts';
  const cases = [
    { name: 'a reload after a refused ask', jobStatus: 'done', jobFailure: undefined, latched: true },
    { name: 'a queued task the worker refused', jobStatus: 'failed', jobFailure: refused, latched: false },
  ];
  for (const tc of cases) {
    const state: Orchestrator = {
      jobStatus: tc.jobStatus,
      jobFailure: tc.jobFailure,
      render: readyRender('rev-1', ['seg-001']),
      generate: () => json({ id: 'job-10', task: 'record' }, 202),
      renderPost: () => json({ accepted: true }, 202),
    };
    const fake = fakeOrchestrator('job-10', state);
    try {
      const client = new RealApiClient();
      const seeded = seedReel(client, 'job-10');
      seeded.editConfig = RENDER_ONLY_CHANGE;
      if (tc.latched) {
        // As the constructor rehydrates a refusal persisted on the intent.
        const failure: AdmissionFailure = { failureReason: CHANGE_NOT_APPLIED_REASON, retryAction: 'record' };
        const stored = withAdmissionFailure(seeded, failure);
        const seedable = client as unknown as Seedable;
        seedable.intents.set(stored.videoId, stored);
        seedable.reels.set(stored.videoId, videoFromIntent(stored));
        seedable.driveLatch.set(stored.videoId, failure);
      }
      await tick(client);
      await tick(client);
      assert.equal(fake.calls.filter((c) => c.method === 'POST').length, 0, `${tc.name}: a page load asks nothing`);
      assertWaitsForClick(await client.getVideo(seeded.videoId), tc.name);

      await client.retryVideo(seeded.videoId);
      const sent = fake.posts('/generate');
      assert.equal(sent.length, 1, `${tc.name}: Reintentar records`);
      assert.equal(sent[0]?.body !== undefined && 'allow_capture' in sent[0].body, false, `${tc.name}: a click may record`);
    } finally {
      fake.restore();
    }
  }
});
