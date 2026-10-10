import test from 'node:test';
import assert from 'node:assert/strict';
import type { CloudJob } from './parse.ts';
import {
  anyCloudJobActive,
  cloudFailureView,
  cloudJobView,
  cloudJobViewOn,
  cloudJobsByMatch,
  cloudJobsNewlyReady,
  cloudSubmitErrorMessage,
  estimateRange,
} from './view.ts';

const NOW = Date.parse('2026-10-09T10:00:00Z');
/** En and em dashes, which user-facing copy must not contain. */
const DASHES = new RegExp(`[${String.fromCharCode(0x2013, 0x2014)}]`);
const MINUTE = 60_000;

function job(overrides: Partial<CloudJob>): CloudJob {
  return {
    id: '11111111-1111-4111-8111-111111111111',
    localJobId: '22222222-2222-4222-8222-222222222222',
    kind: 'short',
    title: 'R3 4k - Kill Feed',
    status: 'queued',
    stage: null,
    percent: null,
    queue: null,
    failure: null,
    cancelRequested: false,
    stalled: null,
    createdAt: NOW - 5 * MINUTE,
    finishedAt: null,
    videos: [],
    ...overrides,
  };
}

const FAILURE_CODES = [
  'capture_flake', 'interrupted', 'worker_lost', 'demo_download_failed', 'internal',
  'demo_incompatible', 'unplayable_start', 'target_not_found', 'spec_mismatch', 'invalid_spec',
  'render_failed', 'job_failed', 'timeout', 'upload_failed',
  'capture_incompatible', 'steam_unavailable', 'disk_full', 'tools_missing',
  'demo_upload_failed', 'results_expired',
  'limit_uploads', 'limit_storage', 'cloud_storage_full', 'download_failed',
] as const;

const SUBMIT_CODES = [
  'limit_active', 'limit_daily', 'cloud_queue_full', 'cloud_storage_full',
  'cloud_access_pending', 'cloud_access_blocked', 'portal_unreachable',
  'not_linked', 'job_not_parsed', 'unknown_segment', 'demo_too_large', 'invalid_spec', 'kind_unavailable',
  'not_found', 'not_configured',
  'studio_version_mismatch', 'limit_storage', 'limit_uploads',
] as const;

test('the estimate is a range from the start to 35 percent later, on 5 minute steps', () => {
  assert.equal(estimateRange(0), 'menos de 5 min');
  assert.equal(estimateRange(299), 'menos de 5 min');
  assert.equal(estimateRange(20 * 60), 'unos 20 a 30 min');
  assert.equal(estimateRange(25 * 60), 'unos 25 a 35 min');
  assert.equal(estimateRange(5 * 60), 'unos 5 a 10 min');
  assert.equal(estimateRange(50 * 60), 'entre 50 min y 1 h 10 min');
  assert.equal(estimateRange(120 * 60), 'entre 2 h y 2 h 45 min');
  assert.equal(estimateRange(-30), 'menos de 5 min');
});

test('every status shows the line the design names', () => {
  const cases: Array<{ name: string; job: CloudJob; line: string }> = [
    { name: 'demo upload', job: job({ status: 'uploading_demo', percent: 43 }), line: 'SUBIENDO DEMO 43 %' },
    {
      name: 'queued with an estimate',
      job: job({ status: 'queued', queue: { position: 3, estimatedStartAt: NOW + 20 * MINUTE, estimatedDoneAt: NOW + 28 * MINUTE, state: 'online' } }),
      line: 'EN COLA · puesto 3 · empieza en unos 20 a 30 min',
    },
    {
      name: 'queued while the cloud is paused',
      job: job({ status: 'queued', queue: { position: 3, estimatedStartAt: null, estimatedDoneAt: null, state: 'paused' } }),
      line: 'La nube está en pausa. Tu puesto (3) se mantiene',
    },
    {
      name: 'queued while the worker is offline',
      job: job({ status: 'queued', queue: { position: 2, estimatedStartAt: null, estimatedDoneAt: null, state: 'offline' } }),
      line: 'La nube no está conectada ahora. Tu puesto (2) se mantiene',
    },
    { name: 'capturing', job: job({ status: 'running', stage: 'capturing', percent: 62 }), line: 'GRABANDO EN LA NUBE 62 %' },
    { name: 'rendering', job: job({ status: 'running', stage: 'rendering', percent: 40 }), line: 'RENDER EN LA NUBE 40 %' },
    { name: 'downloading to this PC', job: job({ status: 'downloading', percent: 80 }), line: 'DESCARGANDO 80 %' },
    { name: 'canceled', job: job({ status: 'canceled' }), line: 'CANCELADO' },
  ];
  for (const testCase of cases) {
    assert.equal(cloudJobView(testCase.job, NOW).line, testCase.line, testCase.name);
  }
});

test('a queued job without an estimate states its position and invents no time', () => {
  const view = cloudJobView(
    job({ status: 'queued', queue: { position: 4, estimatedStartAt: null, estimatedDoneAt: null, state: 'online' } }),
    NOW,
  );
  assert.equal(view.line, 'EN COLA · puesto 4');
  assert.equal(view.bar, false);
});

test('a job can be cancelled while the cloud still works on it, and removed only afterwards', () => {
  for (const status of ['uploading_demo', 'queued', 'running', 'uploading'] as const) {
    const view = cloudJobView(job({ status }), NOW);
    assert.deepEqual([view.active, view.canCancel, view.canRemove], [true, true, false], status);
  }
  // The cloud already finished a job that is only downloading: a cancel there can never work.
  const downloading = cloudJobView(job({ status: 'downloading', percent: 30 }), NOW);
  assert.deepEqual([downloading.active, downloading.canCancel, downloading.canRemove], [true, false, false]);
  for (const status of ['done', 'failed', 'canceled'] as const) {
    const view = cloudJobView(job({ status, failure: status === 'failed' ? { code: 'timeout', message: '' } : null }), NOW);
    assert.deepEqual([view.active, view.canCancel, view.canRemove], [false, false, true], status);
  }
});

test('a requested cancel shows as cancelling and cannot be requested twice', () => {
  const view = cloudJobView(job({ status: 'running', stage: 'capturing', percent: 10, cancelRequested: true }), NOW);
  assert.equal(view.line, 'CANCELANDO');
  assert.equal(view.canCancel, false);
  assert.equal(view.active, true);
});

test('only videos verified on disk can be played', () => {
  const done = job({
    status: 'done',
    videos: [
      { name: 'short-01.mp4', variant: 'viral-60-clean', sizeBytes: 10, ready: true },
      { name: 'short-02.mp4', variant: 'viral-60-clean', sizeBytes: 10, ready: false },
    ],
  });
  assert.deepEqual(cloudJobView(done, NOW).playable, ['short-01.mp4']);
  const expired = cloudJobView(job({ status: 'done', videos: [] }), NOW);
  assert.deepEqual(expired.playable, []);
  assert.match(expired.line, /ya no está disponible/);
});

test('a moving job on a PC without an account asks to connect it and offers no cancel that cannot work', () => {
  for (const status of ['queued', 'running', 'uploading', 'downloading'] as const) {
    const view = cloudJobViewOn(job({ status, stage: status === 'running' ? 'capturing' : null, percent: 40 }), { now: NOW, linked: false });
    assert.match(view.line, /no está conectado a tu cuenta/, status);
    assert.deepEqual([view.needsLink, view.active, view.canCancel, view.bar, view.percent], [true, false, false, false, null], status);
  }
  const linked = cloudJobViewOn(job({ status: 'running', stage: 'capturing', percent: 40 }), { now: NOW, linked: true });
  assert.deepEqual(linked, cloudJobView(job({ status: 'running', stage: 'capturing', percent: 40 }), NOW));
  assert.equal(linked.needsLink, false);
});

test('a finished job reads the same with or without an account', () => {
  const done = job({ status: 'done', videos: [{ name: 'short-01.mp4', variant: 'viral-60-clean', sizeBytes: 10, ready: true }] });
  const failed = job({ status: 'failed', failure: { code: 'worker_lost', message: '' } });
  for (const finished of [done, failed, job({ status: 'canceled' })]) {
    assert.deepEqual(cloudJobViewOn(finished, { now: NOW, linked: false }), cloudJobView(finished, NOW), finished.status);
  }
});

test('an unknown status is shown as unknown and offers nothing destructive', () => {
  const view = cloudJobView(job({ status: 'unknown' }), NOW);
  assert.match(view.line, /Actualiza Studio/);
  assert.deepEqual([view.active, view.canCancel, view.canRemove], [false, false, false]);
});

test('every failure code has its own Spanish sentence without dashes', () => {
  const messages = FAILURE_CODES.map((code) => cloudFailureView({ code, message: 'raw worker text' }).message);
  assert.equal(new Set(messages).size, FAILURE_CODES.length);
  for (const message of messages) {
    assert.doesNotMatch(message, /raw worker text/);
    assert.doesNotMatch(message, DASHES);
  }
});

test('failure guidance matches what can actually help', () => {
  assert.match(cloudFailureView({ code: 'spec_mismatch', message: '' }).message, /Actualiza Studio y vuelve a intentarlo/);
  assert.match(cloudFailureView({ code: 'demo_upload_failed', message: '' }).message, /subir la demo/);
  for (const code of ['capture_incompatible', 'steam_unavailable', 'disk_full', 'tools_missing']) {
    const failure = cloudFailureView({ code, message: '' });
    // The portal fails a job with one of these only after giving up on it: nothing is paused for it.
    assert.match(failure.message, /varias veces/, code);
    assert.doesNotMatch(failure.message, /en pausa|No es un problema de tu demo/, code);
    assert.equal(failure.recordLocal, true, code);
  }
  // A demo CS2 cannot play fails on this PC too: no button may suggest otherwise.
  for (const code of ['demo_incompatible', 'unplayable_start', 'target_not_found']) {
    const failure = cloudFailureView({ code, message: '' });
    assert.deepEqual([failure.retryCloud, failure.recordLocal], [false, false], code);
  }
});

test('a failure code newer than this Studio shows the sentence the worker wrote', () => {
  assert.equal(cloudFailureView({ code: 'future_code', message: 'La nube no pudo con esto.' }).message, 'La nube no pudo con esto.');
  assert.match(cloudFailureView({ code: 'future_code', message: '  ' }).message, /falló en la nube/);
  assert.match(cloudFailureView({ code: 'constructor', message: '' }).message, /falló en la nube/);
});

test('a failed job puts the failure sentence on its status line', () => {
  const view = cloudJobView(job({ status: 'failed', failure: { code: 'timeout', message: 'x' } }), NOW);
  assert.equal(view.tone, 'failed');
  assert.equal(view.line, view.failure?.message);
  assert.match(view.line, /tardó demasiado/);
});

test('every submit rejection has its own sentence and none falls back to the generic one', () => {
  const generic = cloudSubmitErrorMessage({ code: 'something_else', status: 400 });
  const messages = SUBMIT_CODES.map((code) => cloudSubmitErrorMessage({ code, status: 429 }));
  assert.equal(new Set(messages).size, SUBMIT_CODES.length);
  for (const message of messages) {
    assert.notEqual(message, generic);
    assert.doesNotMatch(message, DASHES);
  }
});

test('submit rejections without a code still say what happened', () => {
  assert.match(cloudSubmitErrorMessage({ status: 401 }), /ya no está conectado/);
  assert.match(cloudSubmitErrorMessage({ status: 404 }), /Actualiza Studio/);
  // A coded 404 is the local match that is gone, not an old Studio.
  assert.match(cloudSubmitErrorMessage({ code: 'not_found', status: 404 }), /ya no está en este PC/);
  assert.match(cloudSubmitErrorMessage({ code: 'service_unavailable', status: 503 }), /servicio local/);
});

test('the hub polls fast only while some cloud job is still moving', () => {
  const here = { now: NOW, linked: true };
  assert.equal(anyCloudJobActive([job({ status: 'done' }), job({ status: 'failed', failure: { code: '', message: '' } })], here), false);
  assert.equal(anyCloudJobActive([job({ status: 'done' }), job({ status: 'downloading' })], here), true);
  assert.equal(anyCloudJobActive([], here), false);
  // Nothing can move on this screen while the PC has no account.
  assert.equal(anyCloudJobActive([job({ status: 'running' })], { now: NOW, linked: false }), false);
});

test('the daily limit sentence does not claim the time is used up when the video just does not fit', () => {
  const message = cloudSubmitErrorMessage({ code: 'limit_daily', status: 429 });
  assert.match(message, /no cabe en el tiempo de nube que te queda hoy/);
  assert.doesNotMatch(message, /agotado/);
});

test('a job is announced as ready once, when it turns done between two polls', () => {
  const running = job({ id: 'a', status: 'running' });
  const done = job({ id: 'a', status: 'done' });
  assert.deepEqual(cloudJobsNewlyReady([running], [done]).map((item) => item.id), ['a']);
  assert.deepEqual(cloudJobsNewlyReady([done], [done]), []);
  // The first poll after opening Studio must not replay old results.
  assert.deepEqual(cloudJobsNewlyReady([], [done]), []);
});

test('cloud jobs are grouped under the match they were created from', () => {
  const grouped = cloudJobsByMatch([
    job({ id: 'a', localJobId: 'm1' }),
    job({ id: 'b', localJobId: 'm2' }),
    job({ id: 'c', localJobId: 'm1' }),
  ]);
  assert.deepEqual(grouped.get('m1')?.map((item) => item.id), ['a', 'c']);
  assert.deepEqual(grouped.get('m2')?.map((item) => item.id), ['b']);
});

test('cancelling always says what it gives up, by where the job stands', () => {
  const queued = cloudJobView(job({ status: 'queued' }), NOW);
  assert.match(queued.cancelWarning ?? '', /puesto en la cola/);
  for (const status of ['running', 'uploading'] as const) {
    assert.match(cloudJobView(job({ status }), NOW).cancelWarning ?? '', /se pierde lo que lleva hecho/, status);
  }
  assert.match(cloudJobView(job({ status: 'uploading_demo' }), NOW).cancelWarning ?? '', /demo/);
  // No cancel button, no warning.
  for (const view of [
    cloudJobView(job({ status: 'done' }), NOW),
    cloudJobView(job({ status: 'downloading' }), NOW),
    cloudJobView(job({ status: 'running', cancelRequested: true }), NOW),
    cloudJobViewOn(job({ status: 'queued' }), { now: NOW, linked: false }),
  ]) {
    assert.deepEqual([view.canCancel, view.cancelWarning], [false, null]);
  }
});

test('removing asks first only when it deletes a video from this PC', () => {
  const video = { name: 'short-01.mp4', variant: 'viral-60-clean', sizeBytes: 10, ready: true };
  const one = cloudJobView(job({ status: 'done', videos: [video] }), NOW);
  assert.match(one.removeWarning ?? '', /Se borrará el vídeo de este PC/);
  assert.match(one.removeWarning ?? '', /Guárdalo antes/);
  const two = cloudJobView(job({ status: 'done', videos: [video, { ...video, name: 'short-02.mp4' }] }), NOW);
  assert.match(two.removeWarning ?? '', /Se borrarán los 2 vídeos de este PC/);
  // Nothing on disk to lose: one click, as before.
  for (const settled of [
    job({ status: 'failed', failure: { code: 'timeout', message: '' } }),
    job({ status: 'canceled' }),
    job({ status: 'done', videos: [] }),
    job({ status: 'done', videos: [{ ...video, ready: false }] }),
  ]) {
    const view = cloudJobView(settled, NOW);
    assert.deepEqual([view.canRemove, view.removeWarning], [true, null], settled.status);
  }
  for (const warning of [one.removeWarning, two.removeWarning]) assert.doesNotMatch(warning ?? '', DASHES);
});

test('a moving job says so when its status is only the last one known', () => {
  const running = job({ status: 'running', stage: 'capturing', percent: 40 });
  const stale = cloudJobView({ ...running, stalled: 'portal_unreachable' }, NOW);
  assert.equal(stale.line, 'Sin conexión con la nube. Último estado: GRABANDO EN LA NUBE 40 %');
  // Nothing is moving on screen, but the job is still followed and can still be cancelled.
  assert.deepEqual([stale.tone, stale.bar, stale.percent, stale.active, stale.canCancel], ['neutral', false, null, true, true]);
  assert.notEqual(stale.line, cloudJobView(running, NOW).line);

  const queued = job({ status: 'queued', queue: { position: 3, estimatedStartAt: NOW + 20 * MINUTE, estimatedDoneAt: null, state: 'online' } });
  // An estimate from before the outage is not repeated as if it still held.
  assert.equal(cloudJobView({ ...queued, stalled: 'portal_unreachable' }, NOW).line, 'Sin conexión con la nube. Último estado: EN COLA · puesto 3');

  for (const status of ['uploading_demo', 'queued', 'running', 'uploading'] as const) {
    assert.match(cloudJobView(job({ status, stalled: 'portal_unreachable' }), NOW).line, /^Sin conexión con la nube\. Último estado: /, status);
  }
  // A finished job is not waiting for the cloud.
  const done = job({ status: 'done', stalled: 'portal_unreachable' });
  assert.deepEqual(cloudJobView(done, NOW), cloudJobView({ ...done, stalled: null }, NOW));
});

test('a demo upload that is waiting to retry says so and can still be cancelled', () => {
  const view = cloudJobView(job({ status: 'uploading_demo', percent: 12, stalled: 'demo_upload' }), NOW);
  assert.equal(view.line, 'SUBIENDO DEMO · la subida se cortó y se reintenta sola');
  assert.deepEqual([view.active, view.canCancel, view.percent], [true, true, null]);
});

test('a download that keeps failing says so and can be removed after a warning', () => {
  const view = cloudJobView(job({ status: 'downloading', percent: 30, stalled: 'download' }), NOW);
  assert.equal(view.line, 'DESCARGANDO · la descarga a este PC falla y se reintenta sola');
  assert.deepEqual([view.active, view.canCancel, view.canRemove], [true, false, true]);
  assert.match(view.removeWarning ?? '', /no se guardará en este PC/);
  // A download that is going well is left alone.
  assert.equal(cloudJobView(job({ status: 'downloading', percent: 30 }), NOW).removeWarning, null);
});

test('a version mismatch at submit uses the agreed sentence', () => {
  assert.equal(
    cloudSubmitErrorMessage({ code: 'studio_version_mismatch', status: 409 }),
    'Tu Studio y la nube usan versiones distintas. Actualiza ClipHub Studio y vuelve a intentarlo.',
  );
});

test('the storage refusals of the demo upload read as what they are, not as a network failure', () => {
  const uploads = cloudFailureView({ code: 'limit_uploads', message: '' });
  assert.match(uploads.message, /dos demos tuyas subiendo/);
  const storage = cloudFailureView({ code: 'limit_storage', message: '' });
  assert.match(storage.message, /Tus demos ocupan todo tu espacio en la nube/);
  for (const failure of [uploads, storage, cloudFailureView({ code: 'cloud_storage_full', message: '' })]) {
    assert.doesNotMatch(failure.message, /Revisa tu conexión/);
    assert.deepEqual([failure.retryCloud, failure.recordLocal], [true, true]);
  }
  assert.match(cloudFailureView({ code: 'download_failed', message: '' }).message, /llegó dañado a este PC/);
});
