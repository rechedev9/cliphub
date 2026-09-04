// RealApiClient batch-status reconcile: leftover render docs before recorded
// must not adopt a previous ready pack, re-drive record, or show composing.
import assert from 'node:assert/strict';
import test from 'node:test';
import { RealApiClient } from './real.ts';
import { buildEditRequest } from './edit-request.ts';
import { DEFAULT_VARIANT, DEFAULT_EDIT_CONFIG, type ReelIntent } from './reel-store.ts';
import type { EditConfig, Video } from './types.ts';

type Call = { url: string; method: string };

type Orchestrator = {
  jobStatus: string;
  render: Record<string, unknown> | null;
  generate: () => Response;
};

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
}

function readyRender(artifactPrefix: string, segmentIds: string[], edit: EditConfig = DEFAULT_EDIT_CONFIG): Record<string, unknown> {
  return { status: 'ready', artifact_prefix: artifactPrefix, videos: ['video.mp4'], segment_ids: segmentIds, edit: buildEditRequest(edit) };
}

function fakeBatch(jobId: string, state: Orchestrator): { calls: Call[]; restore: () => void } {
  const original = globalThis.fetch;
  const calls: Call[] = [];
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? 'GET';
    calls.push({ url, method });
    if (url.startsWith('/api/demos/batch-status')) {
      return json({
        items: [{
          job_id: jobId,
          variant: DEFAULT_VARIANT,
          job: { status: state.jobStatus },
          render: state.render,
        }],
      });
    }
    if (url === `/api/demos/${jobId}/generate` && method === 'POST') return state.generate();
    throw new Error(`unexpected fetch: ${method} ${url}`);
  }) as typeof globalThis.fetch;
  return { calls, restore: () => { globalThis.fetch = original; } };
}

async function tick(client: RealApiClient): Promise<void> {
  await client.listVideos();
  await new Promise((resolve) => setTimeout(resolve, 0));
  await new Promise((resolve) => setTimeout(resolve, 0));
}

type Seedable = { intents: Map<string, ReelIntent>; reels: Map<string, Video> };

function seedReel(client: RealApiClient, jobId: string): string {
  const intent: ReelIntent = {
    videoId: `${jobId}__seg-001`,
    jobId,
    segmentIds: ['seg-001'],
    mode: 'clean',
    variant: DEFAULT_VARIANT,
    editConfig: { ...DEFAULT_EDIT_CONFIG },
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
  return intent.videoId;
}

test('batch leftover render during recapture stays hidden', async () => {
  const cases: Array<{
    name: string;
    jobStatus: string;
    render: Record<string, unknown>;
    wantStatus: string;
    wantPosts: number;
  }> = [
    {
      name: 'matching ready leftover while recording must not stop the poll',
      jobStatus: 'recording',
      render: readyRender('old-pack', ['seg-001']),
      wantStatus: 'recording',
      wantPosts: 0,
    },
    {
      name: 'failed leftover while recording must not re-drive record',
      jobStatus: 'recording',
      render: { status: 'failed', error: 'recording_not_reusable: capture fingerprint predates observer-steamid-input-v2' },
      wantStatus: 'recording',
      wantPosts: 0,
    },
    {
      name: 'queued leftover while parsed must not show composing',
      jobStatus: 'parsed',
      render: { status: 'queued' },
      wantStatus: 'recording',
      wantPosts: 1,
    },
  ];
  for (const tc of cases) {
    const state: Orchestrator = {
      jobStatus: tc.jobStatus,
      render: tc.render,
      generate: () => json({ id: 'job', task: 'record' }, 202),
    };
    const fake = fakeBatch('job-batch', state);
    try {
      const client = new RealApiClient();
      const videoId = seedReel(client, 'job-batch');
      await tick(client);
      const reel = await client.getVideo(videoId);
      assert.equal(reel?.status, tc.wantStatus, tc.name);
      assert.equal(
        fake.calls.filter((c) => c.method === 'POST' && c.url.endsWith('/generate')).length,
        tc.wantPosts,
        `${tc.name}: generate POSTs`,
      );
    } finally {
      fake.restore();
    }
  }
});

test('batch ready render after done is still adopted', async () => {
  const state: Orchestrator = {
    jobStatus: 'done',
    render: readyRender('rev-1', ['seg-001']),
    generate: () => json({}, 202),
  };
  const fake = fakeBatch('job-done', state);
  try {
    const client = new RealApiClient();
    const videoId = seedReel(client, 'job-done');
    await tick(client);
    assert.equal((await client.getVideo(videoId))?.status, 'ready');
    assert.equal(fake.calls.filter((c) => c.method === 'POST').length, 0);
  } finally {
    fake.restore();
  }
});
