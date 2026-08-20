// Unit tests for the pure reel-reconcile core. Run: node --test reel-reconcile.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { canHaveRenderState, decideReelReconcile, deriveReelView, requiresRecapture, retryReelAction, shouldReconcileVideoStatus, unrecoverableJobGoneView, viewForJobGone, viewForRecordAdmission } from './reel-reconcile.ts';
import type { ReconcileInput } from './reel-reconcile.ts';
import type { EditConfig } from './types.ts';
import type { MusicChoice } from './reel-music.ts';
import { DEFAULT_EDIT_CONFIG } from './reel-store.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';

/** deriveReelView with sane defaults so each case states only what it varies. */
function view(over: Partial<ReconcileInput>) {
  return deriveReelView({ jobStatus: 'parsed', renderStatus: 'none', ...over });
}

test('parsed + no render → drive record', () => {
  assert.deepEqual(view({ jobStatus: 'parsed' }), { status: 'queued', action: 'record' });
});

test('recording → show recording, do not re-drive record', () => {
  assert.deepEqual(view({ jobStatus: 'recording' }), { status: 'recording', action: 'none' });
});

test('recording with progress → carries segments done/total to the card', () => {
  assert.deepEqual(
    view({ jobStatus: 'recording', captureProgress: { done: 2, total: 4 } }),
    { status: 'recording', action: 'none', captureProgress: { done: 2, total: 4 } },
  );
});

test('recording without progress → no captureProgress key (indeterminate bar)', () => {
  const v = view({ jobStatus: 'recording' });
  assert.equal('captureProgress' in v, false);
});

test('progress is ignored when not recording (never leaks onto other stages)', () => {
  assert.deepEqual(
    view({ jobStatus: 'recorded', captureProgress: { done: 2, total: 4 } }),
    { status: 'composing', action: 'render' },
  );
});

test('recorded + no render → drive render', () => {
  assert.deepEqual(view({ jobStatus: 'recorded' }), { status: 'composing', action: 'render' });
});

test('composed + no render → drive render', () => {
  assert.deepEqual(view({ jobStatus: 'composed' }), { status: 'composing', action: 'render' });
});

test('done + no render → drive render', () => {
  assert.deepEqual(view({ jobStatus: 'done' }), { status: 'composing', action: 'render' });
});

test('render queued → composing, no action', () => {
  assert.deepEqual(view({ jobStatus: 'recorded', renderStatus: 'queued' }), { status: 'composing', action: 'none' });
});

test('render rendering → composing, do not re-drive render', () => {
  assert.deepEqual(view({ jobStatus: 'recorded', renderStatus: 'rendering' }), { status: 'composing', action: 'none' });
});

test('render ready → ready', () => {
  assert.deepEqual(view({ jobStatus: 'done', renderStatus: 'ready' }), { status: 'ready', action: 'none' });
});

test('render ready wins even if job flags failed (a finished reel is ready)', () => {
  assert.deepEqual(
    view({ jobStatus: 'failed', jobFailureReason: 'x', renderStatus: 'ready' }),
    { status: 'ready', action: 'none' },
  );
});

test('render warnings stay terminal but block publication', () => {
  assert.deepEqual(
    view({
      jobStatus: 'failed',
      jobFailureReason: 'later failure',
      renderStatus: 'review_required',
      renderWarnings: ['frozen frame at 00:12.400'],
      renderArtifactPrefix: 'jobs/j/renders/v/revisions/r',
    }),
    {
      status: 'review_required',
      action: 'none',
      warnings: ['frozen frame at 00:12.400'],
      reviewArtifactPrefix: 'jobs/j/renders/v/revisions/r',
    },
  );
});

test('review-required reels remain on idle reconciliation for cross-tab updates', () => {
  assert.equal(shouldReconcileVideoStatus('review_required'), true);
  assert.equal(shouldReconcileVideoStatus('ready'), false);
  assert.equal(shouldReconcileVideoStatus('failed'), false);
});

test('job failed → failed with reason', () => {
  assert.deepEqual(
    view({ jobStatus: 'failed', jobFailureReason: 'recorder exited with code 1' }),
    { status: 'failed', action: 'none', failureReason: 'recorder exited with code 1' },
  );
});

test('render failed → failed with reason', () => {
  assert.deepEqual(
    view({ jobStatus: 'recorded', renderStatus: 'failed', renderFailureReason: 'ffmpeg error' }),
    { status: 'failed', action: 'none', failureReason: 'ffmpeg error' },
  );
});

test('render failed with non-reusable capture → re-record instead of looping render', () => {
  assert.deepEqual(
    view({
      jobStatus: 'recorded',
      renderStatus: 'failed',
      renderFailureReason: 'recording result capture_mode must be "real"',
    }),
    { status: 'queued', action: 'record' },
  );
  assert.deepEqual(
    view({
      jobStatus: 'recorded',
      renderStatus: 'failed',
      renderFailureReason: 'recording_not_reusable:recording result capture_mode must be "real"',
    }),
    { status: 'queued', action: 'record' },
  );
});

test('job failed with non-reusable capture → re-record', () => {
  assert.deepEqual(
    view({
      jobStatus: 'failed',
      jobFailureReason: 'recording_not_reusable:recording result lacks completed POV verification',
    }),
    { status: 'queued', action: 'record' },
  );
});

test('requiresRecapture matches prefix and legacy English strings', () => {
  assert.equal(requiresRecapture(undefined), false);
  assert.equal(requiresRecapture(''), false);
  assert.equal(requiresRecapture('ffmpeg error'), false);
  assert.equal(requiresRecapture('compose failed'), false);
  assert.equal(requiresRecapture('recording result capture_mode must be "real"'), true);
  assert.equal(requiresRecapture('recording_not_reusable:anything'), true);
  assert.equal(requiresRecapture('recording result lacks completed POV verification'), true);
  assert.equal(requiresRecapture('recording result capture input fingerprint does not match its plan'), true);
  assert.equal(requiresRecapture('legacy recording result contains fields from a newer capture contract'), true);
  assert.equal(requiresRecapture('recording result publication is pending'), true);
});

test('ordinary render failure keeps failed + no re-record action (retry re-renders)', () => {
  const v = view({
    jobStatus: 'recorded',
    renderStatus: 'failed',
    renderFailureReason: 'ffmpeg exited with code 1',
  });
  assert.equal(v.status, 'failed');
  assert.equal(v.action, 'none');
  assert.equal(v.failureReason, 'ffmpeg exited with code 1');
  assert.equal(requiresRecapture(v.failureReason), false);
});

test('all known non-reusable render strings map to re-record', () => {
  const reasons = [
    'recording result capture_mode must be "real"',
    'recording_not_reusable:recording result capture_mode must be "real"',
    'recording result lacks completed POV verification',
    'recording_not_reusable:recording result lacks completed POV verification',
    'recording result capture input fingerprint does not match its plan',
    'legacy recording result contains fields from a newer capture contract',
    'recording result publication is pending',
  ];
  for (const reason of reasons) {
    const v = view({
      jobStatus: 'recorded',
      renderStatus: 'failed',
      renderFailureReason: reason,
    });
    assert.equal(v.status, 'queued', `status for ${reason}`);
    assert.equal(v.action, 'record', `action for ${reason}`);
    assert.equal(requiresRecapture(reason), true, `requiresRecapture(${reason})`);
  }
});

test('still parsing → queued, no action (only drive from parsed onward)', () => {
  assert.deepEqual(view({ jobStatus: 'parsing' }), { status: 'queued', action: 'none' });
  assert.deepEqual(view({ jobStatus: 'scanned' }), { status: 'queued', action: 'none' });
});

test('unknown job status → queued, no action (never guess an action)', () => {
  assert.deepEqual(view({ jobStatus: 'wat' }), { status: 'queued', action: 'none' });
});

test('failed without a reason still reports failed (no spurious empty reason key)', () => {
  assert.deepEqual(view({ jobStatus: 'failed' }), { status: 'failed', action: 'none' });
});

test('unrecoverableJobGoneView: failed + unrecoverable with a failure reason', () => {
  const v = unrecoverableJobGoneView();
  assert.equal(v.status, 'failed');
  assert.equal(v.action, 'none');
  assert.equal(v.unrecoverable, true);
  assert.ok(v.failureReason, 'the card needs a human-readable reason');
});

test('viewForJobGone: latches only after consecutive 404 ticks', () => {
  // One 404 must not latch: delete+re-forge would destroy the artifact.
  const cases: Array<{ strikes: number; latched: boolean }> = [
    { strikes: 0, latched: false },
    { strikes: 1, latched: false },
    { strikes: 2, latched: true },
    { strikes: 3, latched: true },
  ];
  for (const { strikes, latched } of cases) {
    const view = viewForJobGone(strikes);
    if (latched) {
      assert.deepEqual(view, unrecoverableJobGoneView(), `strikes=${strikes} should latch`);
    } else {
      assert.equal(view, null, `strikes=${strikes} should leave the view untouched`);
    }
  }
});

test('a normal job failure stays recoverable (retry can re-drive it)', () => {
  const v = view({ jobStatus: 'failed', jobFailureReason: 'recorder exited with code 1' });
  assert.equal('unrecoverable' in v, false);
});

test('a normal render failure stays recoverable (retry can re-drive it)', () => {
  const v = view({ jobStatus: 'recorded', renderStatus: 'failed', renderFailureReason: 'ffmpeg error' });
  assert.equal('unrecoverable' in v, false);
});

test('canHaveRenderState: true only once a render POST can have been driven', () => {
  for (const s of ['recorded', 'composing', 'composed', 'review_required', 'done']) {
    assert.equal(canHaveRenderState(s), true, `${s} should allow a render GET`);
  }
});

test('canHaveRenderState: includes failed (a render can be ready before the job fails)', () => {
  // deriveReelView surfaces a finished render as ready even when the job later
  // flags failed, so the render GET must still fire for a failed job.
  assert.equal(canHaveRenderState('failed'), true);
});

test('canHaveRenderState: false for every pre-recorded status (skip the guaranteed 404)', () => {
  for (const s of ['queued', 'scanning', 'scanned', 'parsing', 'parsed', 'recording', 'wat']) {
    assert.equal(canHaveRenderState(s), false, `${s} must not issue a render GET`);
  }
});

test('decideReelReconcile does not adopt a ready Shorts pack as Full Demo', () => {
  const shortsEdit: EditConfig = { ...DEFAULT_EDIT_CONFIG };
  const cases: Array<{
    name: string;
    jobStatus: string;
    renderStatus: ReconcileInput['renderStatus'];
    intentEdit: EditConfig;
    renderEdit?: EditConfig;
    intentMusic?: MusicChoice;
    renderMusic?: MusicChoice;
    wantAction: 'record' | 'render' | 'none';
    wantStatus: string;
    wantAdopt: boolean;
  }> = [
    {
      name: 'full demo + recorded + ready shorts → record, do not adopt',
      jobStatus: 'recorded',
      renderStatus: 'ready',
      intentEdit: FULL_DEMO_EDIT,
      renderEdit: shortsEdit,
      wantAction: 'record',
      wantStatus: 'queued',
      wantAdopt: false,
    },
    {
      name: 'full demo + done + ready shorts → record, do not adopt',
      jobStatus: 'done',
      renderStatus: 'ready',
      intentEdit: FULL_DEMO_EDIT,
      renderEdit: shortsEdit,
      wantAction: 'record',
      wantStatus: 'queued',
      wantAdopt: false,
    },
    {
      name: 'full demo + ready without render edit → record (legacy shorts)',
      jobStatus: 'recorded',
      renderStatus: 'ready',
      intentEdit: FULL_DEMO_EDIT,
      wantAction: 'record',
      wantStatus: 'queued',
      wantAdopt: false,
    },
    {
      name: 'full demo + recorded + matching recap render → ready, adopt',
      jobStatus: 'recorded',
      renderStatus: 'ready',
      intentEdit: FULL_DEMO_EDIT,
      renderEdit: { ...FULL_DEMO_EDIT },
      wantAction: 'none',
      wantStatus: 'ready',
      wantAdopt: true,
    },
    {
      name: 'shorts + recorded + ready shorts → ready, adopt',
      jobStatus: 'recorded',
      renderStatus: 'ready',
      intentEdit: shortsEdit,
      renderEdit: shortsEdit,
      wantAction: 'none',
      wantStatus: 'ready',
      wantAdopt: true,
    },
    {
      name: 'full demo + parsed + no render → record',
      jobStatus: 'parsed',
      renderStatus: 'none',
      intentEdit: FULL_DEMO_EDIT,
      wantAction: 'record',
      wantStatus: 'queued',
      wantAdopt: false,
    },
    {
      name: 'full demo + recording + ready shorts → wait, do not adopt',
      jobStatus: 'recording',
      renderStatus: 'ready',
      intentEdit: FULL_DEMO_EDIT,
      renderEdit: shortsEdit,
      wantAction: 'none',
      wantStatus: 'recording',
      wantAdopt: false,
    },
    {
      name: 'full demo + recording + no render → wait, not failed',
      jobStatus: 'recording',
      renderStatus: 'none',
      intentEdit: FULL_DEMO_EDIT,
      wantAction: 'none',
      wantStatus: 'recording',
      wantAdopt: false,
    },
    {
      name: 'full demo + recorded + ready recap + different song → render, do not adopt',
      jobStatus: 'recorded',
      renderStatus: 'ready',
      intentEdit: FULL_DEMO_EDIT,
      renderEdit: { ...FULL_DEMO_EDIT },
      intentMusic: { songId: 'phonk-01', musicVolume: 1 },
      renderMusic: {},
      wantAction: 'render',
      wantStatus: 'composing',
      wantAdopt: false,
    },
    {
      name: 'full demo + recorded + ready recap + different edit → render, do not adopt',
      jobStatus: 'recorded',
      renderStatus: 'ready',
      intentEdit: { ...FULL_DEMO_EDIT, killEffect: 'punch-in' },
      renderEdit: { ...FULL_DEMO_EDIT },
      wantAction: 'render',
      wantStatus: 'composing',
      wantAdopt: false,
    },
    {
      name: 'full demo + recorded + ready recap + same song → ready, adopt',
      jobStatus: 'recorded',
      renderStatus: 'ready',
      intentEdit: FULL_DEMO_EDIT,
      renderEdit: { ...FULL_DEMO_EDIT },
      intentMusic: { songId: 'phonk-01', musicVolume: 0.8, gameVolume: 0.2 },
      renderMusic: { songId: 'phonk-01', musicVolume: 0.8, gameVolume: 0.2 },
      wantAction: 'none',
      wantStatus: 'ready',
      wantAdopt: true,
    },
  ];
  for (const tc of cases) {
    const got = decideReelReconcile({
      jobStatus: tc.jobStatus,
      renderStatus: tc.renderStatus,
      intentEdit: tc.intentEdit,
      renderEdit: tc.renderEdit,
      intentMusic: tc.intentMusic,
      renderMusic: tc.renderMusic,
    });
    assert.equal(got.view.action, tc.wantAction, `${tc.name} action`);
    assert.equal(got.view.status, tc.wantStatus, `${tc.name} status`);
    assert.equal(got.adoptEffective, tc.wantAdopt, `${tc.name} adopt`);
    assert.notEqual(got.view.status, 'failed', `${tc.name} must not latch failed`);
  }
});

test('retryReelAction does not re-drive record while the job is recording', () => {
  const cases: Array<{
    name: string;
    jobStatus: string;
    renderStatus: ReconcileInput['renderStatus'];
    renderFailureReason?: string;
    want: 'record' | 'render' | 'none';
  }> = [
    { name: 'full demo retry while recording', jobStatus: 'recording', renderStatus: 'none', want: 'none' },
    { name: 'recording even with a failed shorts pack', jobStatus: 'recording', renderStatus: 'failed', want: 'none' },
    { name: 'failed job still re-records', jobStatus: 'failed', renderStatus: 'none', want: 'record' },
    { name: 'failed render re-renders', jobStatus: 'recorded', renderStatus: 'failed', want: 'render' },
  ];
  for (const tc of cases) {
    assert.equal(
      retryReelAction({
        jobStatus: tc.jobStatus,
        renderStatus: tc.renderStatus,
        renderFailureReason: tc.renderFailureReason,
      }),
      tc.want,
      tc.name,
    );
  }
});

test('viewForRecordAdmission treats in-flight capture as progress, not a failed reel', () => {
  const cases: Array<{
    name: string;
    status: number;
    body: { error?: string; code?: string };
    wantStatus: string | null;
    wantAction?: 'record' | 'render' | 'none';
  }> = [
    {
      name: '409 recording is in-progress',
      status: 409,
      body: { error: 'job is not ready to record (status=recording)' },
      wantStatus: 'recording',
      wantAction: 'none',
    },
    {
      name: '202 accepted leaves polling to reconcile',
      status: 202,
      body: {},
      wantStatus: null,
    },
    {
      name: 'durable capture error stays failed',
      status: 409,
      body: { error: 'recap plan not ready' },
      wantStatus: 'failed',
      wantAction: 'none',
    },
  ];
  for (const tc of cases) {
    const got = viewForRecordAdmission(tc.status, tc.body);
    if (tc.wantStatus === null) {
      assert.equal(got, null, tc.name);
      continue;
    }
    assert.ok(got, tc.name);
    assert.equal(got.status, tc.wantStatus, `${tc.name} status`);
    assert.equal(got.action, tc.wantAction, `${tc.name} action`);
    if (tc.wantStatus === 'recording') {
      assert.equal('failureReason' in got, false, `${tc.name} must not carry the 409 as failureReason`);
    }
  }
});
