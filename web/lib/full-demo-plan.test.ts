import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import {
  approveFullDemo, fixedFullDemoFreeze, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoPlanEdit, isFullDemoOptions, isFullDemoSnapshot,
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

test('fixed freeze migrates old drafts without changing gameplay voice settings', () => {
  const original = fixture().document.options;
  original.editorial.freeze_seconds = 20;
  original.editorial.max_freeze_seconds = 60;
  original.editorial.keep_freeze_voice = true;
  original.editorial.voice_context_seconds = 3;
  const fixed = fixedFullDemoFreeze(original);
  assert.equal(fixed.editorial.freeze_seconds, 2);
  assert.equal(fixed.editorial.max_freeze_seconds, 2);
  assert.equal(fixed.editorial.keep_freeze_voice, false);
  assert.equal(fixed.editorial.voice_context_seconds, 0);
  assert.deepEqual(fixed.audio, original.audio);
  assert.equal(original.editorial.freeze_seconds, 20);
});

test('variable-freeze documents must be replanned even with a current planner version', () => {
  const { document } = fixture();
  document.options.editorial.keep_freeze_voice = true;
  assert.equal(fullDemoApprovalKey(document, document.options), null);
  assert.throws(() => approveFullDemo(document), /freeze fijo de 2 segundos/);
});

test('legacy snapshots remain readable but cannot be approved for a new recording', () => {
  const snapshot = fixture();
  snapshot.document.planner_version = 'full-demo-editorial-v1';
  assert.ok(isFullDemoSnapshot(snapshot));
  assert.deepEqual(coerceEditConfig(fullDemoPlanEdit(snapshot)).fullDemo, snapshot);
  assert.equal(fullDemoApprovalKey(snapshot.document, snapshot.document.options), null);
  assert.throws(() => approveFullDemo(snapshot.document), /Vuelve a preparar/);
});

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

const INCOMPATIBLE = /plan Full Demo incompatible/;
const JOB = '11111111-1111-4111-8111-111111111111';
const PROVENANCE = { title: 'Owned clip', creator: 'Owner', source_url: 'local:owned', permission: 'Owned media', attribution: '' };
// Malformed or failed wire responses reject at the guard; nothing is defaulted or
// partially accepted, and a client-side option guard never reaches the server.
for (const [name, call, response, message, requests] of [
  ['load: unknown compatibility', () => loadFullDemoPlan(JOB), { status: 200, body: { document: null, defaults: fixture().document.options, compatibility: 'editorial-v9' } }, INCOMPATIBLE, 1],
  ['load: document with invalid hash', () => loadFullDemoPlan(JOB), { status: 200, body: { document: { ...fixture().document, plan_hash: 'not-a-hash' }, defaults: fixture().document.options, compatibility: 'editorial-v1' } }, INCOMPATIBLE, 1],
  ['load: defaults missing', () => loadFullDemoPlan(JOB), { status: 200, body: { document: fixture().document, compatibility: 'editorial-v1' } }, INCOMPATIBLE, 1],
  ['load: server error message', () => loadFullDemoPlan(JOB), { status: 503, body: { code: 'service_unavailable', error: 'Sin conexión' } }, { message: 'Sin conexión' }, 1],
  ['load: server error without message', () => loadFullDemoPlan(JOB), { status: 500, body: {} }, { message: 'Solicitud fallida (500).' }, 1],
  ['save: empty document', () => saveFullDemoPlan(JOB, fixture().document.options), { status: 201, body: {} }, INCOMPATIBLE, 1],
  ['save: envelope instead of document', () => saveFullDemoPlan(JOB, fixture().document.options), { status: 201, body: { document: fixture().document, defaults: fixture().document.options, compatibility: 'editorial-v1' } }, INCOMPATIBLE, 1],
  ['save: conflict message', () => saveFullDemoPlan(JOB, fixture().document.options), { status: 409, body: { code: 'full_demo_facts_insufficient', error: 'Vuelve a analizar el jugador' } }, { message: 'Vuelve a analizar el jugador' }, 1],
  ['save: invalid options never leave the client', () => saveFullDemoPlan(JOB, { ...fixture().document.options, audio: { ...fixture().document.options.audio, game: { gain: Number.NaN, voice_priority: false } } }), { status: 201, body: fixture().document }, /Revisa los valores/, 0],
  ['upload: invalid asset id', () => uploadFullDemoAsset(new File(['x'], 'clip.wav'), PROVENANCE), { status: 200, body: { id: 'not-a-uuid', sha256: 'c'.repeat(64) } }, /Referencia de archivo inválida/, 1],
  ['upload: non-object body', () => uploadFullDemoAsset(new File(['x'], 'clip.wav'), PROVENANCE), { status: 200, body: [] }, /no certificó el archivo/, 1],
] satisfies [string, () => Promise<unknown>, { status: number; body: unknown }, RegExp | { message: string }, number][]) {
  test(`rejects ${name}`, async (context) => {
    let count = 0;
    context.mock.method(globalThis, 'fetch', async (): Promise<Response> => { count += 1; return Response.json(response.body, { status: response.status }); });
    await assert.rejects(call, message);
    assert.equal(count, requests);
  });
}

// `responseJSON` parses the body before it looks at the status, so a non-JSON body
// (proxy HTML page, empty reply) surfaces as a parse error rather than the
// status-derived "Solicitud fallida" message, whatever the status code was.
for (const [name, call, response] of [
  ['load: HTML body with 200', () => loadFullDemoPlan(JOB), () => new Response('<!doctype html><title>Proxy</title>', { status: 200, headers: { 'Content-Type': 'text/html' } })],
  ['load: HTML body with 502', () => loadFullDemoPlan(JOB), () => new Response('<html>Bad Gateway</html>', { status: 502, headers: { 'Content-Type': 'text/html' } })],
  ['save: empty body with 204', () => saveFullDemoPlan(JOB, fixture().document.options), () => new Response(null, { status: 204 })],
  ['upload: plain-text error with 413', () => uploadFullDemoAsset(new File(['x'], 'clip.wav'), PROVENANCE), () => new Response('Payload Too Large', { status: 413, headers: { 'Content-Type': 'text/plain' } })],
] satisfies [string, () => Promise<unknown>, () => Response][]) {
  test(`rejects ${name} as a parse error, not a defaulted plan`, async (context) => {
    let count = 0;
    context.mock.method(globalThis, 'fetch', async (): Promise<Response> => { count += 1; return response(); });
    await assert.rejects(call, SyntaxError);
    assert.equal(count, 1);
  });
}

test('saving normalizes a variable freeze to the fixed freeze before it reaches the wire', async (context) => {
  const snapshot = fixture();
  let sent: unknown;
  context.mock.method(globalThis, 'fetch', async (_url: string, init?: RequestInit): Promise<Response> => { sent = JSON.parse(String(init?.body)); return Response.json(snapshot.document); });
  const variable = structuredClone(snapshot.document.options);
  variable.editorial.freeze_seconds = 7; variable.editorial.keep_freeze_voice = true;
  assert.deepEqual(await saveFullDemoPlan(JOB, variable), snapshot.document);
  assert.deepEqual(sent, { options: fixedFullDemoFreeze(variable) });
  assert.equal(fullDemoOptionsKey(fixedFullDemoFreeze(variable)), fullDemoOptionsKey(snapshot.document.options));
});
