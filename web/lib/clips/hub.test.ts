import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamJob } from '../api/streams.ts';
import type { Match, Video } from '../api/types.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';
import {
  activeJobCount,
  buildHubModel,
  clipFilterCounts,
  firstRunComplete,
  firstRunProgress,
  fullChipLabel,
  hubNextStep,
  matchMetaParts,
  HUB_ROW_STAGE,
  hubTransitions,
  matchesClipFilter,
  matchRowStage,
  outputState,
  outputTagLabel,
  outputType,
  recBusy,
  roundsFromScore,
  settleHubSnapshot,
  shortsChipTone,
  toOutput,
  type HubSnapshot,
} from './hub.ts';

function match(id: string, status?: string): Match {
  return {
    id,
    map: 'de_mirage',
    score: '13-9',
    playedAt: '2026-09-01T10:00:00Z',
    stats: { kills: 20, deaths: 10, assists: 3, mvps: 2, kd: 2 },
    decentPlays: 5,
    ...(status === undefined ? {} : { status }),
  };
}

function reel(overrides: Partial<Video> & Pick<Video, 'id' | 'status'>): Video {
  return {
    title: `reel ${overrides.id}`,
    map: 'de_mirage',
    score: '13-9',
    mode: 'clean',
    createdAt: 1000,
    ...overrides,
  };
}

test('outputType: a landscape recap is a Full POV, everything else a Short', () => {
  assert.equal(outputType(reel({ id: 'a', status: 'ready', editConfig: FULL_DEMO_EDIT })), 'full');
  assert.equal(outputType(reel({ id: 'b', status: 'ready' })), 'short');
  assert.equal(
    outputType(reel({ id: 'c', status: 'ready', editConfig: { ...FULL_DEMO_EDIT, matchRecap: false } })),
    'short',
  );
});

test('outputState maps the video pipeline onto the handoff vocabulary', () => {
  const cases: Array<[Video['status'], string]> = [
    ['queued', 'queue'],
    ['recording', 'rec'],
    ['composing', 'render'],
    ['ready', 'ready'],
    ['review_required', 'ready'],
    ['failed', 'failed'],
  ];
  for (const [status, state] of cases) {
    assert.equal(outputState(status), state, status);
  }
});

test('matchRowStage: plan-ready is ready, scanned is unpicked, anything earlier or unknown is parsing', () => {
  const cases: Array<[string | undefined, string]> = [
    [undefined, HUB_ROW_STAGE.ready],
    ['queued', HUB_ROW_STAGE.parsing],
    ['scanning', HUB_ROW_STAGE.parsing],
    ['parsing', HUB_ROW_STAGE.parsing],
    ['scanned', HUB_ROW_STAGE.unpicked],
    ['parsed', HUB_ROW_STAGE.ready],
    ['recording', HUB_ROW_STAGE.ready],
    ['recorded', HUB_ROW_STAGE.ready],
    ['composing', HUB_ROW_STAGE.ready],
    ['composed', HUB_ROW_STAGE.ready],
    ['done', HUB_ROW_STAGE.ready],
    // Same gate as the produce page: an unknown status is not a plan.
    ['validating', HUB_ROW_STAGE.parsing],
  ];
  for (const [status, want] of cases) assert.equal(matchRowStage(status), want, status);
});

test('hubTransitions announces a parse only when a parsing row becomes ready, not unpicked', () => {
  const before = buildHubModel([match('m1', 'parsing'), match('m2', 'parsing'), match('m3', 'scanned')], []);
  const after = buildHubModel([match('m1', 'parsed'), match('m2', 'scanned'), match('m3', 'parsed')], []);
  assert.deepEqual(
    hubTransitions(before, after).parsed.map((row) => row.match.id),
    ['m1'],
  );
});

function fulfilled<T>(value: T): PromiseSettledResult<T> {
  return { status: 'fulfilled', value };
}

function rejected<T>(reason: string): PromiseSettledResult<T> {
  return { status: 'rejected', reason };
}

type Sources = Parameters<typeof settleHubSnapshot>[0];

test('settleHubSnapshot keeps the previous value of every rejected source', () => {
  const prev: HubSnapshot = {
    matches: [match('m1')],
    videos: [reel({ id: 'v1', status: 'ready', jobId: 'm1' })],
    streams: [{ id: 's1', status: 'rendering', created_at: '' }],
    failure: null,
  };
  const fresh = {
    matches: [match('m2')],
    videos: [reel({ id: 'v2', status: 'queued', jobId: 'm2' })],
    streams: [] as StreamJob[],
  };
  const cases: Array<{ name: string; results: Sources; want: HubSnapshot }> = [
    {
      name: 'all fulfilled',
      results: [fulfilled(fresh.matches), fulfilled(fresh.videos), fulfilled(fresh.streams)],
      want: { ...fresh, failure: null },
    },
    {
      name: 'matches rejected',
      results: [rejected('m'), fulfilled(fresh.videos), fulfilled(fresh.streams)],
      want: { matches: prev.matches, videos: fresh.videos, streams: fresh.streams, failure: 'm' },
    },
    {
      name: 'videos rejected',
      results: [fulfilled(fresh.matches), rejected('v'), fulfilled(fresh.streams)],
      want: { matches: fresh.matches, videos: prev.videos, streams: fresh.streams, failure: 'v' },
    },
    {
      name: 'streams rejected',
      results: [fulfilled(fresh.matches), fulfilled(fresh.videos), rejected('s')],
      want: { matches: fresh.matches, videos: fresh.videos, streams: prev.streams, failure: 's' },
    },
  ];
  for (const { name, results, want } of cases) {
    assert.deepEqual(settleHubSnapshot(results, prev), want, name);
  }
});

test('settleHubSnapshot throws only when both demo sources fail; a first load starts from empty', () => {
  const videos = [reel({ id: 'v1', status: 'ready', jobId: 'm1' })];
  const prev: HubSnapshot = { matches: [match('m1')], videos, streams: [], failure: null };
  for (const previous of [prev, null]) {
    assert.throws(() => settleHubSnapshot([rejected('m'), rejected('v'), fulfilled([])], previous), /^m$/);
  }
  // Local reels still list when the jobs index is down: the Clips lens works offline.
  const reelsOnly = settleHubSnapshot([rejected('m'), fulfilled(videos), fulfilled([])], null);
  assert.deepEqual(reelsOnly, { matches: [], videos, streams: [], failure: 'm' });
  const matchesOnly = settleHubSnapshot([fulfilled([match('m1')]), rejected('v'), rejected('s')], null);
  assert.deepEqual(matchesOnly, { matches: [match('m1')], videos: [], streams: [], failure: 'v' });
});

test('toOutput carries progress only while REC or render report one', () => {
  const rec = toOutput(reel({ id: 'a', status: 'recording', captureProgress: { done: 3, total: 10 } }));
  assert.equal(rec.percent, 30);
  assert.deepEqual(rec.rounds, { done: 3, total: 10 });
  const render = toOutput(reel({ id: 'b', status: 'composing', captureProgress: { done: 10, total: 10, percent: 41 } }));
  assert.equal(render.percent, 41);
  assert.equal(render.rounds, null);
  const queued = toOutput(reel({ id: 'c', status: 'queued', captureProgress: { done: 0, total: 10 } }));
  assert.equal(queued.percent, null);
  const review = toOutput(reel({ id: 'd', status: 'review_required' }));
  assert.equal(review.reviewRequired, true);
  assert.equal(review.state, 'ready');
});

test('buildHubModel groups reels under their partida and keeps orphans apart', () => {
  const model = buildHubModel(
    [match('m1', 'parsed'), match('m2', 'parsing'), match('m3', 'scanned')],
    [
      reel({ id: 'v1', status: 'ready', jobId: 'm1', createdAt: 1 }),
      reel({ id: 'v2', status: 'recording', jobId: 'm1', editConfig: FULL_DEMO_EDIT, createdAt: 2 }),
      reel({ id: 'v3', status: 'ready', jobId: 'gone', createdAt: 3 }),
      reel({ id: 'v4', status: 'ready', createdAt: 4 }),
    ],
  );
  assert.equal(model.rows.length, 3);
  assert.equal(model.rows[0].shorts.map((o) => o.id).join(','), 'v1');
  assert.equal(model.rows[0].fulls.map((o) => o.id).join(','), 'v2');
  assert.equal(model.rows[0].stage, HUB_ROW_STAGE.ready);
  assert.equal(model.rows[1].stage, HUB_ROW_STAGE.parsing);
  // scanned: roster returned, nobody picked a POV yet — not "still parsing".
  assert.equal(model.rows[2].stage, HUB_ROW_STAGE.unpicked);
  // Orphans (jobId matches no listed partida) stay out of every row.
  assert.deepEqual(model.orphans.map((o) => o.id), ['v4', 'v3']);
  assert.deepEqual(model.clips.map((o) => o.id), ['v4', 'v3', 'v2', 'v1']);
  assert.equal(model.clips[3].match?.id, 'm1');
  assert.equal(model.clips[0].match, null);
});

test('buildHubModel lists the newest output first inside a row', () => {
  const model = buildHubModel(
    [match('m1')],
    [
      reel({ id: 'old', status: 'ready', jobId: 'm1', createdAt: 1 }),
      reel({ id: 'new', status: 'ready', jobId: 'm1', createdAt: 9 }),
    ],
  );
  assert.deepEqual(model.rows[0].shorts.map((o) => o.id), ['new', 'old']);
});

test('clip filters and counts agree with each other', () => {
  const outputs = [
    toOutput(reel({ id: 'a', status: 'ready' })),
    toOutput(reel({ id: 'b', status: 'composing' })),
    toOutput(reel({ id: 'c', status: 'ready', editConfig: FULL_DEMO_EDIT })),
    toOutput(reel({ id: 'd', status: 'failed' })),
  ];
  // A failed output is done, not "en marcha": it must not count as working.
  assert.deepEqual(clipFilterCounts(outputs), { all: 4, short: 3, full: 1, ready: 2, working: 1 });
  assert.equal(matchesClipFilter(outputs[1], 'working'), true);
  assert.equal(matchesClipFilter(outputs[3], 'working'), false);
  assert.equal(matchesClipFilter(outputs[2], 'short'), false);
});

test('recBusy and activeJobCount count what is actually moving', () => {
  const model = buildHubModel(
    [match('m1', 'parsing'), match('m2', 'scanned'), match('m3')],
    [
      reel({ id: 'a', status: 'recording', jobId: 'm3' }),
      reel({ id: 'b', status: 'queued', jobId: 'm3' }),
      reel({ id: 'c', status: 'ready', jobId: 'm3' }),
    ],
  );
  assert.equal(recBusy(model), true);
  // m1 parsing + 'a' on REC; the queued 'b' and unpicked m2 are backlog.
  assert.equal(activeJobCount(model), 2);
  assert.equal(
    activeJobCount(model, [
      { id: 's1', status: 'rendering', created_at: '' },
      { id: 's2', status: 'ready', created_at: '' },
    ]),
    3,
  );
});

test('fullChipLabel follows the latest Full POV state', () => {
  assert.equal(fullChipLabel([]), 'Full POV · sin generar');
  assert.equal(fullChipLabel([toOutput(reel({ id: 'a', status: 'recording', editConfig: FULL_DEMO_EDIT }))]), 'Full POV · REC');
  assert.equal(fullChipLabel([toOutput(reel({ id: 'a', status: 'ready', editConfig: FULL_DEMO_EDIT }))]), 'Full POV · listo');
  assert.equal(fullChipLabel([toOutput(reel({ id: 'a', status: 'queued', editConfig: FULL_DEMO_EDIT }))]), 'Full POV · en cola');
});

test('outputTagLabel: one uppercase tag per state, with progress when reported', () => {
  const cases: Array<[Parameters<typeof outputTagLabel>[0], string]> = [
    [{ state: 'ready', percent: null, rounds: null }, 'LISTO'],
    [{ state: 'ready', percent: null, rounds: null, reviewRequired: true }, 'REVISIÓN QA'],
    [{ state: 'render', percent: 41, rounds: null }, 'RENDER 41%'],
    [{ state: 'render', percent: null, rounds: null }, 'RENDER'],
    [{ state: 'rec', percent: 15, rounds: { done: 3, total: 20 } }, 'REC R3/20'],
    [{ state: 'rec', percent: null, rounds: null }, 'REC'],
    [{ state: 'queue', percent: null, rounds: null }, 'EN COLA'],
    [{ state: 'failed', percent: null, rounds: null }, 'FALLÓ'],
  ];
  for (const [input, want] of cases) assert.equal(outputTagLabel(input), want);
});

test('shortsChipTone: working > existing > none', () => {
  const ready = toOutput(reel({ id: 'a', status: 'ready' }));
  const working = toOutput(reel({ id: 'b', status: 'composing' }));
  assert.equal(shortsChipTone([]), 'neutral');
  assert.equal(shortsChipTone([ready]), 'success');
  assert.equal(shortsChipTone([ready, working]), 'primary');
});

test('roundsFromScore: sums both halves, null when unparseable', () => {
  const cases: Array<[string, number | null]> = [
    ['13-9', 22],
    ['16 - 14', 30],
    ['', null],
    ['13', null],
  ];
  for (const [score, want] of cases) assert.equal(roundsFromScore(score), want);
});

test('hubTransitions: reports rows that finished parsing and outputs that became ready', () => {
  const prev = buildHubModel([match('m1', 'parsing'), match('m2')], [
    reel({ id: 'v1', jobId: 'm2', status: 'composing' }),
    reel({ id: 'v2', jobId: 'm2', status: 'ready' }),
  ]);
  const next = buildHubModel([match('m1', 'parsed'), match('m2')], [
    reel({ id: 'v1', jobId: 'm2', status: 'ready' }),
    reel({ id: 'v2', jobId: 'm2', status: 'ready' }),
    reel({ id: 'v3', jobId: 'm2', status: 'ready' }),
  ]);
  const got = hubTransitions(prev, next);
  assert.deepEqual(got.parsed.map((row) => row.match.id), ['m1']);
  // v2 was already ready and v3 is new (first seen ready), so only v1 flipped.
  assert.deepEqual(got.ready.map((clip) => clip.id), ['v1']);
});

test('hubNextStep: parsing waits, scanned picks, an empty ready row asks for its first clip', () => {
  const row = (status: string | undefined, videos: Video[] = []) => buildHubModel([match('m1', status)], videos).rows[0];
  assert.equal(hubNextStep(row('parsing')), 'wait');
  assert.equal(hubNextStep(row('scanned')), 'pick');
  assert.equal(hubNextStep(row('parsed')), 'firstClip');
  assert.equal(hubNextStep(row('parsed', [reel({ id: 'v1', status: 'queued', jobId: 'm1' })])), 'none');
  assert.equal(
    hubNextStep(row('parsed', [reel({ id: 'v1', status: 'ready', jobId: 'm1', editConfig: FULL_DEMO_EDIT })])),
    'none',
  );
});

test('matchMetaParts keeps player, K/D and highlights, and always ends with the date', () => {
  assert.deepEqual(matchMetaParts({ ...match('m1'), player: 'donk' }, 'hace 2 h'), [
    'donk',
    '20/10 K/D',
    '5 highlights',
    'hace 2 h',
  ]);
  const bare: Match = {
    ...match('m2'),
    stats: { kills: 0, deaths: 0, assists: 0, mvps: 0, kd: 0 },
    decentPlays: 0,
  };
  assert.deepEqual(matchMetaParts(bare, 'importada el 1 sept 2026'), ['importada el 1 sept 2026']);
});

test('firstRunProgress flips each step from hub data and completes only with a clip', () => {
  assert.deepEqual(firstRunProgress(buildHubModel([], [])), { load: false, pick: false, produce: false });
  assert.deepEqual(firstRunProgress(buildHubModel([match('m1', 'scanned')], [])), {
    load: true,
    pick: false,
    produce: false,
  });
  const parsed = buildHubModel([match('m1', 'parsed')], []);
  assert.deepEqual(firstRunProgress(parsed), { load: true, pick: true, produce: false });
  assert.equal(firstRunComplete(firstRunProgress(parsed)), false);
  const produced = buildHubModel([match('m1', 'parsed')], [reel({ id: 'v1', status: 'queued', jobId: 'm1' })]);
  assert.equal(firstRunComplete(firstRunProgress(produced)), true);
});
