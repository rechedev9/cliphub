import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamJob, StreamRenderedOutput } from '../api/streams.ts';
import type { EditConfig, Match, Video } from '../api/types.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';
import { buildHubModel } from './hub.ts';
import { DEFAULT_EDIT_CONFIG } from '../api/reel-store.ts';
import type { LoteShort } from './visor-actions.ts';
import { buildVisorQueue, sameVisorEntries, visorAfterDelete, visorEntryId } from './visor-queue.ts';

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

test('imported and recovered Shorts enter the queue', () => {
  const clips = clipsOf(
    demoVideo('imported', { jobId: undefined, publishable: false, createdAt: 3000 }),
    demoVideo('recovered', { recovered: true, createdAt: 2000 }),
  );
  assert.deepEqual(ids({ clips }), ['demo:imported', 'demo:recovered']);
});

const LOTE_ID = 'job7__auto__seg-001_seg-004_seg-009';

function loteVideo(overrides: Partial<Video> = {}): Video {
  return demoVideo(LOTE_ID, {
    jobId: 'job7',
    title: 'Auto-Shorts: 3 Shorts - Viral',
    targetName: 'ropz',
    editConfig: { ...DEFAULT_EDIT_CONFIG, separateShorts: true },
    downloadUrl: '/api/demos/job7/renders/viral-60-clean/videos/seg-001.mp4',
    ...overrides,
  });
}

function loteShorts(...names: string[]): ReadonlyMap<string, LoteShort[]> {
  return new Map([[LOTE_ID, names.map((name) => ({ name, url: `/api/demos/job7/renders/viral-60-clean/videos/${name}` }))]]);
}

test('every Short of a lote is its own entry, in the order the server lists them, each with its lote and position', () => {
  const clips = buildHubModel([partida('job7')], [demoVideo('newer', { createdAt: 5000 }), loteVideo()]).clips;
  const queue = buildVisorQueue({ clips, streams: [], filter: null, requestedId: null, lotes: loteShorts('seg-009', 'seg-001', 'seg-004') });
  assert.deepEqual(queue.entries.map((entry) => [entry.id, entry.mark, entry.title, entry.item.artifactName]), [
    ['demo:newer', null, 'Short newer', 'seg-001.mp4'],
    [`demo:${LOTE_ID}/seg-009`, 'Lote de Mirage · 1 de 3', 'Short 1 · Mirage · ropz', 'seg-009'],
    [`demo:${LOTE_ID}/seg-001`, 'Lote de Mirage · 2 de 3', 'Short 2 · Mirage · ropz', 'seg-001'],
    [`demo:${LOTE_ID}/seg-004`, 'Lote de Mirage · 3 de 3', 'Short 3 · Mirage · ropz', 'seg-004'],
  ]);
  assert.equal(queue.pending, false);
});

test('after a Short of a lote is deleted the others are renumbered', () => {
  const clips = clipsOf(loteVideo({ title: 'Auto-Shorts: 2 Shorts - Viral' }));
  const queue = buildVisorQueue({ clips, streams: [], filter: null, requestedId: `demo:${LOTE_ID}/seg-009`, lotes: loteShorts('seg-001', 'seg-009') });
  assert.deepEqual(queue.entries.map((entry) => entry.mark), ['Lote de Mirage · 1 de 2', 'Lote de Mirage · 2 de 2']);
  assert.equal(queue.start?.id, `demo:${LOTE_ID}/seg-009`);
  assert.equal(queue.fellBack, false);
});

test('a Short of a lote edits and changes the music of the whole lote, and downloads, publishes and deletes only itself', () => {
  const clips = buildHubModel([partida('job7')], [loteVideo({ songId: 'song-1' })]).clips;
  const queue = buildVisorQueue({ clips, streams: [], filter: null, requestedId: null, lotes: loteShorts('seg-004') });
  assert.deepEqual(actionsOf(queue), [[
    { kind: 'edit', label: 'Editar el lote', href: '/shorts/job7?editar=viral-60-clean' },
    { kind: 'music', label: 'Cambiar música del lote' },
    { kind: 'publish', label: 'Preparar publicación', href: `/mis-videos/job7/publicar/${LOTE_ID}?short=seg-004` },
    { kind: 'download', label: 'Descargar' },
    { kind: 'delete', label: 'Borrar' },
  ]]);
  assert.deepEqual(queue.entries[0]?.source, {
    kind: 'demo',
    video: clips[0]?.video,
    loteShort: { name: 'seg-004', url: '/api/demos/job7/renders/viral-60-clean/videos/seg-004' },
  });
  const silent = buildVisorQueue({ clips: clipsOf(loteVideo()), streams: [], filter: null, requestedId: null, lotes: loteShorts('seg-004') });
  assert.equal(actionsOf(silent)[0]?.find((action) => action.kind === 'music')?.label, 'Añadir música al lote');
});

test('the address of a lote opens its first Short, and an unknown map leaves the mark without one', () => {
  const clips = clipsOf(loteVideo({ map: 'Unknown' }));
  const queue = buildVisorQueue({ clips, streams: [], filter: null, requestedId: `demo:${LOTE_ID}`, lotes: loteShorts('seg-001', 'seg-004') });
  assert.equal(queue.start?.id, `demo:${LOTE_ID}/seg-001`);
  assert.equal(queue.fellBack, false);
  assert.deepEqual(queue.entries.map((entry) => entry.mark), ['Lote · 1 de 2', 'Lote · 2 de 2']);
});

test('while the Shorts of a lote are being read the lote stands as its card, and a requested Short waits instead of falling back', () => {
  const clips = clipsOf(demoVideo('other', { createdAt: 5000 }), loteVideo());
  const waiting = buildVisorQueue({ clips, streams: [], filter: null, requestedId: `demo:${LOTE_ID}/seg-004` });
  assert.deepEqual(waiting.entries.map((entry) => entry.id), ['demo:other', `demo:${LOTE_ID}`]);
  assert.deepEqual([waiting.start, waiting.fellBack, waiting.pending], [null, false, true]);
  const listed = buildVisorQueue({ clips, streams: [], filter: null, requestedId: 'demo:other' });
  assert.deepEqual([listed.start?.id, listed.fellBack], ['demo:other', false]);
  const read = buildVisorQueue({ clips, streams: [], filter: null, requestedId: `demo:${LOTE_ID}/seg-004`, lotes: loteShorts('seg-001') });
  assert.deepEqual([read.start?.id, read.fellBack, read.pending], ['demo:other', true, false]);
});

test('a lote with no Shorts left gives no entry', () => {
  assert.deepEqual(ids({ clips: clipsOf(loteVideo()), lotes: loteShorts() }), []);
});

test('Borrar on a Short of a lote moves to the next Short of the lote', () => {
  const entries = buildVisorQueue({ clips: clipsOf(loteVideo()), streams: [], filter: null, requestedId: null, lotes: loteShorts('seg-001', 'seg-004', 'seg-009') }).entries;
  assert.equal(visorAfterDelete(entries, `demo:${LOTE_ID}/seg-004`), `demo:${LOTE_ID}/seg-009`);
});

test('a lote on screen that renders again keeps its Shorts with the mark until the new ones are ready', () => {
  const lotes = loteShorts('seg-001', 'seg-004');
  const shown = buildVisorQueue({ clips: clipsOf(loteVideo()), streams: [], filter: null, requestedId: null, lotes }).entries;
  const rendering = loteVideo({ status: 'composing', downloadUrl: undefined });
  const held = buildVisorQueue({ clips: clipsOf(rendering), streams: [], filter: null, requestedId: `demo:${LOTE_ID}/seg-004`, lotes, shown });
  assert.deepEqual(held.entries.map((entry) => [entry.id, entry.rendering, entry.mark]), [
    [`demo:${LOTE_ID}/seg-001`, true, 'Lote de Mirage · 1 de 2'],
    [`demo:${LOTE_ID}/seg-004`, true, 'Lote de Mirage · 2 de 2'],
  ]);
  assert.equal(held.start?.id, `demo:${LOTE_ID}/seg-004`);
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
  assert.deepEqual(queue, { entries: [], start: null, fellBack: false, pending: false });
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

function partida(id: string): Match {
  return {
    id,
    map: 'de_mirage',
    score: '13-9',
    playedAt: '2026-09-01T10:00:00Z',
    stats: { kills: 20, deaths: 10, assists: 3, mvps: 2, kd: 2 },
    decentPlays: 5,
  };
}

function actionsOf(queue: ReturnType<typeof buildVisorQueue>): Array<Array<{ kind: string; label: string; href?: string }>> {
  return queue.entries.map((entry) => entry.actions.map((action) => ({ ...action })));
}

test('a demo Short offers Editar, Cambiar música, Preparar publicación, Descargar and Borrar, in that order', () => {
  const clips = buildHubModel([partida('job-a')], [demoVideo('a', { songId: 'song-1' })]).clips;
  const queue = buildVisorQueue({ clips, streams: [], filter: null, requestedId: null });
  assert.deepEqual(actionsOf(queue), [[
    { kind: 'edit', label: 'Editar', href: '/shorts/job-a?editar=viral-60-clean' },
    { kind: 'music', label: 'Cambiar música' },
    { kind: 'publish', label: 'Preparar publicación', href: '/mis-videos/job-a/publicar/a' },
    { kind: 'download', label: 'Descargar' },
    { kind: 'delete', label: 'Borrar' },
  ]]);
});

test('a stream Short offers Abrir proyecto, Descargar and Borrar', () => {
  const queue = buildVisorQueue({ clips: [], streams: [streamJob('s1', [streamOutput('c1')])], filter: null, requestedId: null });
  assert.deepEqual(actionsOf(queue), [[
    { kind: 'open-project', label: 'Abrir proyecto', href: '/streams/s1' },
    { kind: 'download', label: 'Descargar' },
    { kind: 'delete', label: 'Borrar' },
  ]]);
  assert.deepEqual(queue.entries[0]?.source, { kind: 'stream', clipId: 'c1' });
});

test('an imported Short only downloads and deletes, a recovered one also prepares its publication', () => {
  const clips = buildHubModel([partida('job-rec')], [
    demoVideo('imported__cloud1__seg-001.mp4', { jobId: undefined, recovered: true, publishable: false, createdAt: 2000 }),
    demoVideo('rec', { recovered: true, createdAt: 1000 }),
  ]).clips;
  const queue = buildVisorQueue({ clips, streams: [], filter: null, requestedId: null });
  assert.deepEqual(actionsOf(queue), [
    [{ kind: 'download', label: 'Descargar' }, { kind: 'delete', label: 'Borrar' }],
    [
      { kind: 'publish', label: 'Preparar publicación', href: '/mis-videos/job-rec/publicar/rec' },
      { kind: 'download', label: 'Descargar' },
      { kind: 'delete', label: 'Borrar' },
    ],
  ]);
});

test('a Short on screen that renders again keeps its previous version with the mark until the new one is ready', () => {
  const ready = demoVideo('a', { reviewArtifactPrefix: 'renders/viral-60-clean/revisions/11111111-1111-4111-8111-111111111111/' });
  const shown = buildVisorQueue({ clips: clipsOf(ready), streams: [], filter: null, requestedId: 'demo:a' }).entries;
  assert.equal(shown[0]?.rendering, false);

  const rendering = { ...ready, status: 'queued' as const, downloadUrl: undefined, reviewArtifactPrefix: undefined };
  const held = buildVisorQueue({ clips: clipsOf(rendering, demoVideo('busy', { status: 'composing' })), streams: [], filter: null, requestedId: 'demo:a', shown });
  assert.deepEqual(held.entries.map((entry) => [entry.id, entry.rendering]), [['demo:a', true]]);
  assert.equal(held.entries[0]?.item.playbackUrl, shown[0]?.item.playbackUrl);
  assert.deepEqual(actionsOf(held)[0]?.map((action) => action.kind), ['download', 'delete']);
  assert.equal(held.start?.id, 'demo:a');

  const rerendered = demoVideo('a', { reviewArtifactPrefix: 'renders/viral-60-clean/revisions/22222222-2222-4222-8222-222222222222/' });
  const after = buildVisorQueue({ clips: clipsOf(rerendered), streams: [], filter: null, requestedId: 'demo:a', shown: held.entries });
  assert.equal(after.entries[0]?.rendering, false);
  assert.match(after.entries[0]?.item.playbackUrl ?? '', /22222222-2222-4222-8222-222222222222/);

  const again = buildVisorQueue({ clips: clipsOf(rendering), streams: [], filter: null, requestedId: 'demo:a', shown: held.entries });
  assert.equal(sameVisorEntries(again.entries, held.entries), true);
  assert.equal(sameVisorEntries(after.entries, held.entries), false);

  const failed = buildVisorQueue({ clips: clipsOf({ ...rendering, status: 'failed' }), streams: [], filter: null, requestedId: 'demo:a', shown });
  assert.deepEqual(failed.entries, []);
});

test('after Borrar the Visor moves to the next Short, to the previous one at the end, and closes on the last one', () => {
  const entries = buildVisorQueue({
    clips: clipsOf(demoVideo('a', { createdAt: 3000 }), demoVideo('b', { createdAt: 2000 }), demoVideo('c', { createdAt: 1000 })),
    streams: [],
    filter: null,
    requestedId: null,
  }).entries;
  assert.equal(visorAfterDelete(entries, 'demo:a'), 'demo:b');
  assert.equal(visorAfterDelete(entries, 'demo:b'), 'demo:c');
  assert.equal(visorAfterDelete(entries, 'demo:c'), 'demo:b');
  assert.equal(visorAfterDelete(entries.slice(0, 1), 'demo:a'), null);
});

test('a demo Short whose demo is gone cannot be edited and adds music when it has none', () => {
  const queue = buildVisorQueue({ clips: clipsOf(demoVideo('orphan')), streams: [], filter: null, requestedId: null });
  assert.deepEqual(actionsOf(queue)[0]?.map((action) => action.label), ['Añadir música', 'Preparar publicación', 'Descargar', 'Borrar']);
  assert.equal(actionsOf(queue)[0]?.find((action) => action.kind === 'publish')?.href, '/mis-videos/-/publicar/orphan');
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
