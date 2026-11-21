import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamJob, StreamRenderedOutput } from '../api/streams.ts';
import type { EditConfig, Video } from '../api/types.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';
import { buildHubModel } from './hub.ts';
import { buildVisorQueue, visorEntryId } from './visor-queue.ts';

function demoVideo(id: string, overrides: Partial<Video> = {}): Video {
  return {
    id,
    jobId: `job-${id}`,
    title: `Short ${id}`,
    map: 'de_mirage',
    score: '13-9',
    mode: 'clean',
    variant: 'viral-60-clean',
    status: 'ready',
    createdAt: 1000,
    downloadUrl: `/api/demos/job-${id}/renders/viral-60-clean/videos/seg-001.mp4`,
    ...overrides,
  };
}

function clipsOf(...videos: Video[]) {
  return buildHubModel([], videos).clips;
}

test('a finished demo Short is the one entry of the queue and the start', () => {
  const queue = buildVisorQueue({ clips: clipsOf(demoVideo('a')), streams: [], filter: null, requestedId: null });
  assert.deepEqual(queue.entries.map((entry) => entry.id), ['demo:a']);
  assert.equal(queue.start?.id, 'demo:a');
  assert.equal(queue.entries[0]?.item.playbackUrl, '/api/demos/job-a/renders/viral-60-clean/videos/seg-001.mp4');
});

const LANDSCAPE: EditConfig = { ...FULL_DEMO_EDIT, matchRecap: false };

function ids(input: Partial<Parameters<typeof buildVisorQueue>[0]>): string[] {
  return buildVisorQueue({ clips: [], streams: [], filter: null, requestedId: null, ...input }).entries.map((entry) => entry.id);
}

test('16:9 videos and Shorts that are not finished stay out of the queue', () => {
  const clips = clipsOf(
    demoVideo('ready'),
    demoVideo('full', { editConfig: FULL_DEMO_EDIT }),
    demoVideo('summary', { editConfig: LANDSCAPE }),
    demoVideo('queued', { status: 'queued' }),
    demoVideo('recording', { status: 'recording' }),
    demoVideo('composing', { status: 'composing' }),
    demoVideo('failed', { status: 'failed' }),
    demoVideo('no-file', { downloadUrl: undefined }),
  );
  assert.deepEqual(ids({ clips }), ['demo:ready']);
});

function streamOutput(clipId: string, overrides: Partial<StreamRenderedOutput> = {}): StreamRenderedOutput {
  return {
    artifact_revision: 'rev1',
    variant: 'stream-short',
    clip_id: clipId,
    artifact_name: `${clipId}.mp4`,
    format: 'video/mp4',
    aspect_ratio: '9:16',
    video_url: `/api/streams/s1/renders/stream-short/videos/${clipId}.mp4`,
    render_status: 'rendered',
    stale: false,
    review_required: false,
    ...overrides,
  };
}

function streamJob(id: string, outputs: StreamRenderedOutput[]): StreamJob {
  return { id, status: 'rendered', created_at: '2026-11-20T10:00:00Z', rendered_outputs: outputs };
}

test('finished vertical stream videos enter after the demo Shorts, in the order the list shows them', () => {
  const clips = clipsOf(demoVideo('old', { createdAt: 1000 }), demoVideo('new', { createdAt: 2000 }));
  const streams = [
    streamJob('s1', [
      streamOutput('c1'),
      streamOutput('wide', { aspect_ratio: '16:9' }),
      streamOutput('pending', { render_status: 'rendering' }),
      streamOutput('c2'),
    ]),
  ];
  assert.deepEqual(ids({ clips, streams }), [
    'demo:new',
    'demo:old',
    'stream:s1:stream-short:rev1:c1',
    'stream:s1:stream-short:rev1:c2',
  ]);
});

test('imported, recovered Shorts and a lote enter, a lote as one entry with its first Short', () => {
  const clips = clipsOf(
    demoVideo('imported', { jobId: undefined, publishable: false, createdAt: 3000 }),
    demoVideo('recovered', { recovered: true, createdAt: 2000 }),
    demoVideo('job7__auto__seg-001_seg-004', {
      jobId: 'job7',
      createdAt: 1000,
      downloadUrl: '/api/demos/job7/renders/viral-60-clean/videos/seg-001.mp4',
    }),
  );
  const queue = buildVisorQueue({ clips, streams: [], filter: null, requestedId: null });
  assert.deepEqual(queue.entries.map((entry) => entry.id), ['demo:imported', 'demo:recovered', 'demo:job7__auto__seg-001_seg-004']);
  assert.equal(queue.entries[2]?.item.artifactName, 'seg-001.mp4');
});

const MIXED = {
  clips: clipsOf(demoVideo('short', { createdAt: 2000 }), demoVideo('summary', { editConfig: LANDSCAPE })),
  streams: [streamJob('s1', [streamOutput('c1')])],
};

test('opened from the list, the queue follows the format and state chips of Mis vídeos', () => {
  assert.deepEqual(ids({ ...MIXED, filter: { format: 'all', stage: 'all' } }), ['demo:short', 'stream:s1:stream-short:rev1:c1']);
  assert.deepEqual(ids({ ...MIXED, filter: { format: 'all', stage: 'ready' } }), ['demo:short', 'stream:s1:stream-short:rev1:c1']);
  assert.deepEqual(ids({ ...MIXED, filter: { format: 'short', stage: 'all' } }), ['demo:short']);
  assert.deepEqual(ids({ ...MIXED, filter: { format: 'stream', stage: 'all' } }), ['stream:s1:stream-short:rev1:c1']);
  assert.deepEqual(ids({ ...MIXED, filter: { format: 'summary', stage: 'all' } }), []);
  assert.deepEqual(ids({ ...MIXED, filter: { format: 'all', stage: 'working' } }), []);
});

test('the queue starts on the requested Short, or on the first one when it is not in the queue', () => {
  const start = (requestedId: string | null, filter: Parameters<typeof buildVisorQueue>[0]['filter'] = null) =>
    buildVisorQueue({ ...MIXED, filter, requestedId }).start?.id ?? null;
  assert.equal(start('stream:s1:stream-short:rev1:c1'), 'stream:s1:stream-short:rev1:c1');
  assert.equal(start(null), 'demo:short');
  assert.equal(start('demo:gone'), 'demo:short');
  assert.equal(start('demo:summary'), 'demo:short');
  assert.equal(start('demo:short', { format: 'stream', stage: 'all' }), 'stream:s1:stream-short:rev1:c1');
});

test('an empty queue has no start and no fallback, so the Visor does not open', () => {
  const queue = buildVisorQueue({ clips: clipsOf(demoVideo('busy', { status: 'composing' })), streams: [], filter: null, requestedId: 'demo:busy' });
  assert.deepEqual(queue, { entries: [], start: null, fellBack: false });
});

test('a Short that becomes ready keeps the requested one as the start', () => {
  const before = buildVisorQueue({ ...MIXED, filter: null, requestedId: 'demo:short' });
  const after = buildVisorQueue({
    clips: clipsOf(demoVideo('fresh', { createdAt: 5000 }), demoVideo('short', { createdAt: 2000 })),
    streams: MIXED.streams,
    filter: null,
    requestedId: 'demo:short',
  });
  assert.equal(before.start?.id, 'demo:short');
  assert.equal(after.start?.id, 'demo:short');
  assert.equal(after.fellBack, false);
  assert.deepEqual(after.entries.map((entry) => entry.id), ['demo:fresh', 'demo:short', 'stream:s1:stream-short:rev1:c1']);
});

test('each entry carries the title the list shows', () => {
  const queue = buildVisorQueue({
    clips: clipsOf(demoVideo('a', { title: 'Short', targetName: 'ropz' })),
    streams: [streamJob('s1', [streamOutput('c1', { title: 'Ace en B' })])],
    filter: null,
    requestedId: null,
  });
  assert.deepEqual(queue.entries.map((entry) => entry.title), ['Short · Mirage · ropz', 'Ace en B']);
});

test('a requested Short that is missing, unfinished or filtered out falls back to the first one and says so', () => {
  const clips = clipsOf(demoVideo('short', { createdAt: 2000 }), demoVideo('busy', { status: 'composing' }));
  const fellBack = (requestedId: string | null, filter: Parameters<typeof buildVisorQueue>[0]['filter'] = null) => {
    const queue = buildVisorQueue({ clips, streams: MIXED.streams, filter, requestedId });
    return { start: queue.start?.id ?? null, fellBack: queue.fellBack };
  };
  assert.deepEqual(fellBack('demo:gone'), { start: 'demo:short', fellBack: true });
  assert.deepEqual(fellBack('demo:busy'), { start: 'demo:short', fellBack: true });
  assert.deepEqual(fellBack('demo:short', { format: 'stream', stage: 'all' }), { start: 'stream:s1:stream-short:rev1:c1', fellBack: true });
  assert.deepEqual(fellBack('demo:short'), { start: 'demo:short', fellBack: false });
  assert.deepEqual(fellBack(null), { start: 'demo:short', fellBack: false });
});

test('a finished demo Short has the Visor entry that its card and its listo notice open; a 16:9 video has none', () => {
  const entryOf = (video: Video) => {
    const [clip] = clipsOf(video);
    assert.ok(clip);
    return visorEntryId(clip);
  };
  assert.equal(entryOf(demoVideo('short')), 'demo:short');
  assert.equal(entryOf(demoVideo('job7__auto__seg-001_seg-004', { jobId: 'job7' })), 'demo:job7__auto__seg-001_seg-004');
  assert.equal(entryOf(demoVideo('full', { editConfig: FULL_DEMO_EDIT })), null);
  assert.equal(entryOf(demoVideo('summary', { editConfig: LANDSCAPE })), null);
  assert.equal(entryOf(demoVideo('busy', { status: 'composing' })), null);
  assert.equal(entryOf(demoVideo('no-file', { downloadUrl: undefined })), null);
});
