import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import {
  approveFullDemo, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoPlanEdit, isFullDemoOptions, isFullDemoSnapshot,
  loadFullDemoPlan, saveFullDemoPlan, uploadFullDemoAsset, type FullDemoOptions, type FullDemoSnapshot,
} from './full-demo-plan.ts';
import { buildEditRequest, editConfigsEqual } from './api/edit-request.ts';
import { coerceEditConfig, coerceIntents } from './api/reel-store.ts';
import { parseEffectiveEditConfig } from './api/render-hydration.ts';
import { fullDemoIntentConflict, shouldReuseReelIntent } from './api/reel-identity.ts';

// Serialized by the real Go planner in the synthetic FFmpeg canary. This is
// editorial evidence only: no HLAE capture attestation or production gate bypass.
function fixture(): FullDemoSnapshot {
  const value: unknown = JSON.parse(readFileSync(new URL('./full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(value));
  return value;
}

test('Go editorial document survives edit wire, local persistence and render hydration', () => {
  const snapshot = fixture();
  const options = snapshot.document.options;
  options.audio.voice.enabled = false; options.audio.voice.gain = 0;
  options.audio.game.gain = 0; options.audio.game.voice_priority = false;
  options.audio.music.ducking.enabled = false; options.audio.music.ducking.game_contribution = 0;
  options.sponsor.enabled = false; options.overlays.roster = false; options.overlays.scoreboard = false;
  options.outputs.cover_policy = 'no-cover';
  const edit = fullDemoPlanEdit(snapshot);
  const body = buildEditRequest(edit);
  assert.equal(body.voice_volume, 0);
  assert.equal(body.voice_comms, false);
  assert.equal(body.cover_strategy, 'no-cover');
  assert.deepEqual(body.full_demo, snapshot);
  assert.deepEqual(coerceEditConfig(JSON.parse(JSON.stringify(edit))), edit);
  assert.deepEqual(parseEffectiveEditConfig(JSON.parse(JSON.stringify(body))), edit);
  assert.ok(snapshot.document.rounds.some((round) => (round.kills?.length ?? 0) === 0));
});

for (const [name, mutate] of [
  ['range with unchanged round ID', (o: FullDemoOptions): void => { o.editorial.freeze_seconds = 5; }],
  ['game volume zero', (o: FullDemoOptions): void => { o.audio.game.gain = 0; }],
  ['crosshair', (o: FullDemoOptions): void => { o.capture.crosshair.allow_capture_default = false; }],
  ['playlist order', (o: FullDemoOptions): void => { o.audio.music.assets.reverse(); }],
  ['sponsor split', (o: FullDemoOptions): void => { o.sponsor.allow_split_round = true; }],
  ['overlay', (o: FullDemoOptions): void => { o.overlays.roster = true; }],
  ['cover', (o: FullDemoOptions): void => { o.outputs.cover_policy = 'generated-gameplay'; }],
] satisfies [string, (options: FullDemoOptions) => void][]) {
  test(`changing ${name} invalidates approval`, () => {
    const { document } = fixture();
    const options = structuredClone(document.options);
    assert.equal(fullDemoApprovalKey(document, options), document.plan_hash);
    mutate(options);
    assert.equal(fullDemoApprovalKey(document, options), null);
  });
}

test('identical options and hashes survive re-fetch, key ordering and approval timestamps', () => {
  const snapshot = fixture();
  const reordered = Object.fromEntries(Object.entries(snapshot.document.options).reverse());
  assert.ok(isFullDemoOptions(reordered));
  assert.equal(fullDemoOptionsKey(snapshot.document.options), fullDemoOptionsKey(reordered));
  const other = structuredClone(snapshot);
  other.document.plan_id = 'cccccccc-cccc-4ccc-8ccc-cccccccccccc';
  other.approval.timestamp = '2026-09-05T12:00:00Z';
  assert.ok(editConfigsEqual(fullDemoPlanEdit(snapshot), fullDemoPlanEdit(other)));
});

for (const [name, value] of [
  ['missing option', { ...fixture().document.options, sponsor: undefined }],
  ['null boolean', { ...fixture().document.options, capture: { ...fixture().document.options.capture, xray: null } }],
  ['unknown key', { ...fixture().document.options, pipeline: 'new' }],
  ['invalid profile', { ...fixture().document.options, profile_id: 'legacy' }],
  ['nonfinite gain', { ...fixture().document.options, audio: { ...fixture().document.options.audio, game: { gain: Number.NaN, voice_priority: false } } }],
  ['reversed window', { ...fixture().document.options, sponsor: { ...fixture().document.options.sponsor, window_start_seconds: 140, window_end_seconds: 130 } }],
] satisfies [string, unknown][]) {
  test(`rejects ${name} without defaulting`, () => assert.equal(isFullDemoOptions(value), false));
}

test('a blocker or stale approval cannot become a legacy persisted intent', () => {
  const snapshot = fixture();
  snapshot.document.blockers = [{ code: 'full_demo_asset_missing', message: 'Missing music', round_id: undefined }];
  assert.throws(() => approveFullDemo(snapshot.document));
  const edit = { ...fullDemoPlanEdit(snapshot) };
  assert.throws(() => coerceEditConfig(edit));
  assert.equal(parseEffectiveEditConfig(buildEditRequest(edit)), undefined);
  assert.deepEqual(coerceIntents([{ videoId: 'match__full-demo', jobId: 'match', segmentIds: [], editConfig: edit }]), []);
});

test('changed Full Demo cannot be silently accepted or overwrite an in-flight intent', () => {
  const first = fixture();
  const second = fixture(); second.document.plan_hash = 'b'.repeat(64); second.approval.approved_plan_hash = second.document.plan_hash;
  const existing = { variant: 'gameplay-pov-60', mode: 'clean', editConfig: fullDemoPlanEdit(first) } satisfies Parameters<typeof fullDemoIntentConflict>[1];
  const input = { matchId: 'match', playIds: [], variant: existing.variant, mode: existing.mode, editConfig: fullDemoPlanEdit(second) };
  for (const status of ['queued', 'recording', 'composing'] as const) {
    assert.equal(fullDemoIntentConflict({ status }, existing, input), true);
    assert.equal(shouldReuseReelIntent({ status }, existing, input), false);
  }
  assert.equal(fullDemoIntentConflict({ status: 'ready' }, existing, input), false);
});

test('planning and upload use same-origin endpoints with complete options and provenance', async (context) => {
  const snapshot = fixture();
  const requests: { url: string; init?: RequestInit }[] = [];
  context.mock.method(globalThis, 'fetch', async (url: string, init?: RequestInit): Promise<Response> => {
    requests.push({ url, init });
    let body: unknown = { document: snapshot.document, defaults: snapshot.document.options, compatibility: 'editorial-v1' };
    if (init?.method === 'POST') body = url === '/api/editor/assets' ? { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) } : snapshot.document;
    return Response.json(body);
  });
  const job = '11111111-1111-4111-8111-111111111111';
  assert.deepEqual((await loadFullDemoPlan(job)).document, snapshot.document);
  assert.deepEqual(await saveFullDemoPlan(job, snapshot.document.options), snapshot.document);
  const provenance = { title: 'Owned clip', creator: 'Owner', source_url: 'local:owned', permission: 'Owned media', attribution: '' };
  await uploadFullDemoAsset(new File(['test'], 'clip.wav', { type: 'audio/wav' }), provenance);
  assert.equal(requests[0]?.url, `/api/demos/${job}/full-demo/plan`);
  assert.equal(requests[1]?.init?.body, JSON.stringify({ options: snapshot.document.options }));
  const body = requests[2]?.init?.body;
  assert.ok(body instanceof FormData);
  assert.ok(body.get('video') instanceof File);
  assert.equal(body.get('config'), JSON.stringify({ provenance }));
  await assert.rejects(loadFullDemoPlan('../private'));
});
