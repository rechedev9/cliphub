import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import {
  approveFullDemo, bumperSummary, currentFullDemoOptions, fixedFullDemoFreeze, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoOverlaySource, fullDemoPlanEdit, isFullDemoOptions, isFullDemoSnapshot,
  loadFullDemoPlan, localFileProvenance, saveFullDemoPlan, uploadFullDemoAsset, uploadFullDemoBumper, type FullDemoOptions, type FullDemoSnapshot,
} from './full-demo-plan.ts';
import { buildEditRequest, editConfigsEqual } from './api/edit-request.ts';
import { coerceEditConfig, coerceIntents } from './api/reel-store.ts';
import { parseEffectiveEditConfig } from './api/render-hydration.ts';
import { fullDemoIntentConflict, shouldReuseReelIntent } from './api/reel-identity.ts';
import { CUSTOM_HUD_THEMES, CUSTOM_HUD_CAPTURE_PROFILE, NATIVE_HUD_CAPTURE_PROFILE } from './custom-hud.ts';
import { fullDemoTransitionPreset } from './full-demo-transitions.ts';

// Serialized by the real Go planner in the synthetic FFmpeg canary. This is
// editorial evidence only: no HLAE capture attestation or production gate bypass.
function fixture(): FullDemoSnapshot {
  const value: unknown = JSON.parse(readFileSync(new URL('./full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(value));
  value.document.options = currentFullDemoOptions(value.document.options);
  return value;
}

test('old drafts normalize removed choices into the automatic Full Demo contract', () => {
  const raw: unknown = JSON.parse(readFileSync(new URL('./full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(raw));
  const normalized = currentFullDemoOptions(raw.document.options);
  assert.deepEqual(normalized.capture.crosshair, { mode: 'observed', code: '', allow_capture_default: false });
  // The fixture predates custom HUDs; its native capture stays a native HUD.
  assert.equal(normalized.capture.hud_profile, NATIVE_HUD_CAPTURE_PROFILE);
  assert.equal(normalized.audio.music.enabled, false);
  assert.deepEqual(normalized.audio.music, {
    enabled: false, assets: [], reference_level: 'track-lufs-minus-16-v1', bed_gain_db: -21, loop_policy: 'ordered-loop',
    ducking: { enabled: true, game_contribution: 0, attack_ms: 20, release_ms: 800, threshold: .025, ratio: 8 },
  });
  assert.deepEqual(normalized.editorial.manual_ranges, []);
  assert.deepEqual(normalized.overlays, { roster: true, scoreboard: true, theme: 'neon-violet', source: 'demo', mode: 'generated' });
  assert.equal(normalized.transitions?.enabled, true);
  assert.equal(fullDemoApprovalKey(raw.document, normalized), null);
});

test('an approved Go document that omits retired overlay assets stays current', () => {
  const snapshot = fixture();
  const { team1_image: _team1, team2_image: _team2, scoreboard_image: _scoreboard, ...overlays } = snapshot.document.options.overlays;
  snapshot.document.options = { ...snapshot.document.options, overlays };
  assert.ok(isFullDemoOptions(snapshot.document.options));
  const current = currentFullDemoOptions(snapshot.document.options);
  assert.equal(fullDemoOptionsKey(current), fullDemoOptionsKey(snapshot.document.options));
  assert.equal(fullDemoApprovalKey(snapshot.document, current), snapshot.document.plan_hash);
});

test('the authoritative Go defaults need no client-side replan', () => {
  const defaults: unknown = JSON.parse(readFileSync(new URL('./full-demo-go-defaults.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoOptions(defaults));
  assert.deepEqual(currentFullDemoOptions(defaults), defaults);
});

// Regression: a FACEIT demo rendered with custom HUD 07 (Circuit) produced
// demo-facts-only overlays because only overlays.source drove the layout.
test('the demo origin owns the overlay format regardless of the custom HUD', () => {
  const cases: Array<[FullDemoOptions['source_kind'], FullDemoOptions['overlays']['source'], string | undefined, string | undefined]> = [
    ['faceit', 'demo', 'circuit', 'faceit'], ['faceit', 'demo', undefined, 'faceit'], ['faceit', 'faceit', 'circuit', 'faceit'],
    ['premier', 'faceit', 'arena', 'premier'], ['professional', 'demo', undefined, 'professional'],
    ['demo', 'faceit', undefined, 'faceit'], ['demo', 'demo', 'circuit', undefined],
  ];
  for (const [sourceKind, overlaySource, hud, want] of cases) {
    const snapshot = fixture();
    const options = snapshot.document.options;
    options.source_kind = sourceKind;
    options.overlays.source = overlaySource;
    if (hud) { options.overlays.hud_theme = hud; options.capture.hud_profile = CUSTOM_HUD_CAPTURE_PROFILE; }
    assert.ok(isFullDemoOptions(options));
    assert.equal(fullDemoOverlaySource(options), want, `${sourceKind}/${overlaySource}/${hud}`);
    const edit = fullDemoPlanEdit(snapshot);
    assert.equal(edit.demoSource, want);
    assert.equal(buildEditRequest(edit).demo_source, want);
    assert.equal(parseEffectiveEditConfig(buildEditRequest(edit))?.demoSource, want);
  }
});

test('all eleven custom HUDs survive approval, persistence and the render request', () => {
  assert.equal(CUSTOM_HUD_THEMES.length, 11);
  for (const theme of CUSTOM_HUD_THEMES) {
    const snapshot = fixture();
    const options = snapshot.document.options;
    options.capture.hud_profile = CUSTOM_HUD_CAPTURE_PROFILE;
    options.overlays.hud_theme = theme.id;
    options.overlays.mode = 'generated';
    options.transitions = fullDemoTransitionPreset();
    assert.ok(isFullDemoOptions(options));
    assert.ok(isFullDemoSnapshot(snapshot));
    const edit = fullDemoPlanEdit(snapshot);
    assert.deepEqual(coerceEditConfig(JSON.parse(JSON.stringify(edit))), edit);
    assert.deepEqual(parseEffectiveEditConfig(buildEditRequest(edit)), edit);
    const changed = structuredClone(options);
    changed.overlays.hud_theme = theme.id === 'arena' ? 'apex' : 'arena';
    assert.equal(fullDemoApprovalKey(snapshot.document, changed), null);
  }
  for (const [profile, theme] of [['native', 'arena'], ['broadcast-clean', undefined], ['broadcast-clean', 'unknown'], ['native', null], ['native', '']]) {
    const options = fixture().document.options;
    assert.equal(isFullDemoOptions({ ...options, capture: { ...options.capture, hud_profile: profile }, overlays: { ...options.overlays, hud_theme: theme } }), false);
  }
});

test('the custom HUD is optional and TrueView is an explicit capture choice', () => {
  const options = fixture().document.options;
  assert.equal(options.overlays.hud_theme, undefined);
  assert.equal(options.capture.hud_profile, NATIVE_HUD_CAPTURE_PROFILE);
  assert.ok(isFullDemoOptions(options));
  // Plain "native" keeps spectator panels; new plans use the clean native HUD.
  assert.equal(currentFullDemoOptions({ ...options, capture: { ...options.capture, hud_profile: 'native' } }).capture.hud_profile, NATIVE_HUD_CAPTURE_PROFILE);
  // A broadcast capture without a theme is not a native choice.
  const orphan = currentFullDemoOptions({ ...options, capture: { ...options.capture, hud_profile: CUSTOM_HUD_CAPTURE_PROFILE } });
  assert.equal(orphan.overlays.hud_theme, CUSTOM_HUD_THEMES[0]?.id);

  // Go omits trueview when off: false must not become a key that dirties the plan.
  assert.equal('trueview' in currentFullDemoOptions({ ...options, capture: { ...options.capture, trueview: false } }).capture, false);
  const trueView = currentFullDemoOptions({ ...options, capture: { ...options.capture, trueview: true } });
  assert.equal(trueView.capture.trueview, true);
  assert.ok(isFullDemoOptions(trueView));
  assert.notEqual(fullDemoOptionsKey(trueView), fullDemoOptionsKey(options));
  assert.equal(isFullDemoOptions({ ...options, capture: { ...options.capture, trueview: 1 } }), false);
});

test('Focus portrait survives draft migration and render persistence and invalidates approval when changed', () => {
  const snapshot = fixture();
  snapshot.document.options = currentFullDemoOptions(snapshot.document.options);
  snapshot.document.options.capture.hud_profile = CUSTOM_HUD_CAPTURE_PROFILE;
  snapshot.document.options.overlays.hud_theme = 'focus';
  const portrait = { id: '22222222-2222-4222-8222-222222222222', sha256: 'a'.repeat(64) };
  snapshot.document.options.overlays.hud_portrait = portrait;
  const options = snapshot.document.options;
  assert.ok(isFullDemoOptions(options));
  assert.deepEqual(currentFullDemoOptions(options).overlays.hud_portrait, portrait);
  const edit = fullDemoPlanEdit(snapshot);
  assert.deepEqual(parseEffectiveEditConfig(buildEditRequest(edit)), edit);
  assert.deepEqual(coerceEditConfig(JSON.parse(JSON.stringify(edit))), edit);
  const changed = structuredClone(options);
  changed.overlays.hud_portrait = { ...portrait, sha256: 'b'.repeat(64) };
  assert.equal(fullDemoApprovalKey(snapshot.document, changed), null);
  changed.overlays.hud_theme = 'arena';
  assert.equal(isFullDemoOptions(changed), false);
  assert.equal(currentFullDemoOptions(changed).overlays.hud_portrait, undefined);
});

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

test('HUD/cosmetic upgrade preserves the saved document and requires a fresh plan', () => {
  const snapshot = fixture();
  snapshot.document.options.overlays.hud_theme = 'apex';
  snapshot.document.options.capture.hud_profile = 'broadcast-clean';
  const original = JSON.stringify(snapshot);
  assert.ok(isFullDemoSnapshot(snapshot));
  const draft = currentFullDemoOptions(snapshot.document.options);
  assert.equal(draft.capture.hud_profile, CUSTOM_HUD_CAPTURE_PROFILE);
  assert.equal(draft.overlays.hud_theme, 'apex');
  assert.equal(draft.overlays.theme, 'neon-violet');
  assert.equal(draft.overlays.roster, true);
  assert.equal(JSON.stringify(snapshot), original);
  assert.equal(fullDemoApprovalKey(snapshot.document, draft), null);
  assert.throws(() => approveFullDemo(snapshot.document), /Vuelve a preparar/);
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
  options.overlays.roster = false; options.overlays.scoreboard = false;
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
  ['cover', (o: FullDemoOptions): void => { o.outputs.cover_policy = 'generated-gameplay'; }],
  ['transitions disabled', (o: FullDemoOptions): void => { o.transitions = { ...fullDemoTransitionPreset(), enabled: false }; }],
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

test('intro, sponsor and outro bumpers are optional, preserved verbatim and part of the approval key', () => {
  const { document } = fixture();
  const options = structuredClone(document.options);
  assert.equal(options.bumpers, undefined);
  assert.equal(bumperSummary(options), 'Desactivados');
  assert.equal(fullDemoOptionsKey(currentFullDemoOptions(options)), fullDemoOptionsKey(options), 'absent bumpers must not be defaulted in');
  const ref = { id: 'dddddddd-dddd-4ddd-8ddd-dddddddddddd', sha256: 'e'.repeat(64) };
  const withBumpers = { ...options, bumpers: { intro: { enabled: true, video: ref }, outro: { enabled: true, video: null } } };
  assert.ok(isFullDemoOptions(withBumpers));
  assert.equal(bumperSummary(withBumpers), 'Intro y outro');
  assert.deepEqual(currentFullDemoOptions(withBumpers).bumpers, withBumpers.bumpers);
  assert.equal(fullDemoApprovalKey(document, withBumpers), null, 'enabling a bumper changes the approved plan');
  assert.equal(bumperSummary({ bumpers: { intro: { enabled: false, video: null }, outro: { enabled: true, video: ref } } }), 'Outro');
  const withSponsor = { ...withBumpers, bumpers: { ...withBumpers.bumpers, sponsor: { enabled: true, video: ref } } };
  assert.ok(isFullDemoOptions(withSponsor));
  assert.equal(bumperSummary(withSponsor), 'Intro, sponsor y outro');
  assert.deepEqual(currentFullDemoOptions(withSponsor).bumpers, withSponsor.bumpers);
  assert.equal(bumperSummary({ bumpers: { intro: { enabled: false, video: null }, outro: { enabled: false, video: null }, sponsor: { enabled: true, video: ref } } }), 'Sponsor');
});

test('an old draft or stored plan with the retired sponsor group stays readable but never reaches the wire', () => {
  const { document } = fixture();
  const video = { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) };
  const legacy = { ...document.options, sponsor: { enabled: true, video, placement_policy: 'round-boundary', after_round_id: 'round-001' } };
  assert.ok(isFullDemoOptions(legacy));
  assert.ok(isFullDemoSnapshot({ ...fixture(), document: { ...document, options: legacy, sponsor_placement: { boundary: 'round-001' } } }));
  const migrated = currentFullDemoOptions(legacy);
  assert.equal(Object.hasOwn(migrated, 'sponsor'), false);
  assert.deepEqual(migrated.bumpers, { intro: { enabled: false, video: null }, outro: { enabled: false, video: null }, sponsor: { enabled: true, video } });
  assert.equal(fullDemoApprovalKey({ ...document, options: legacy }, migrated), null, 'a legacy sponsor plan must be prepared again');
  for (const sponsor of [{ enabled: true, video: null }, { enabled: false, video }]) {
    assert.equal(fullDemoOptionsKey(currentFullDemoOptions({ ...document.options, sponsor })), fullDemoOptionsKey(document.options), 'an unused legacy sponsor is dropped without adding bumpers');
  }
});

for (const [name, value] of [
  ['missing option', { ...fixture().document.options, outputs: undefined }],
  ['bumper without slots', { ...fixture().document.options, bumpers: { intro: { enabled: true, video: null } } }],
  ['bumper with a policy', { ...fixture().document.options, bumpers: { intro: { enabled: true, video: null, placement: 'start' }, outro: { enabled: false, video: null } } }],
  ['sponsor bumper with a placement', { ...fixture().document.options, bumpers: { intro: { enabled: false, video: null }, outro: { enabled: false, video: null }, sponsor: { enabled: true, video: null, after_round_id: 'round-001' } } }],
  ['bumper with a bad ref', { ...fixture().document.options, bumpers: { intro: { enabled: true, video: { id: 'x', sha256: 'y' } }, outro: { enabled: false, video: null } } }],
  ['null boolean', { ...fixture().document.options, capture: { ...fixture().document.options.capture, xray: null } }],
  ['unknown key', { ...fixture().document.options, pipeline: 'new' }],
  ['invalid profile', { ...fixture().document.options, profile_id: 'legacy' }],
  ['nonfinite gain', { ...fixture().document.options, audio: { ...fixture().document.options.audio, game: { gain: Number.NaN, voice_priority: false } } }],
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
  assert.equal(requests[1]?.init?.body, JSON.stringify({ options: currentFullDemoOptions(snapshot.document.options) }));
  const body = requests[2]?.init?.body;
  assert.ok(body instanceof FormData);
  assert.ok(body.get('video') instanceof File);
  assert.equal(body.get('config'), JSON.stringify({ provenance }));
  await assert.rejects(loadFullDemoPlan('../private'));
});

const INCOMPATIBLE = /plan de vídeo largo incompatible/;
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
  ['save: invalid options never leave the client', () => saveFullDemoPlan(JOB, { ...fixture().document.options, profile_id: 'invalid' } as unknown as FullDemoOptions), { status: 201, body: fixture().document }, /Revisa los valores/, 0],
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
  assert.deepEqual(sent, { options: currentFullDemoOptions(variable) });
  assert.equal(fullDemoOptionsKey(currentFullDemoOptions(variable)), fullDemoOptionsKey(snapshot.document.options));
});

test('bumper upload accepts MP4 directly without inventing ownership and rejects invalid files locally', async (t) => {
  let calls = 0;
  const ref = { id: 'dddddddd-dddd-4ddd-8ddd-dddddddddddd', sha256: 'd'.repeat(64) };
  t.mock.method(globalThis, 'fetch', async (_url: string, init?: RequestInit) => {
    calls++;
    assert.ok(init?.body instanceof FormData);
    const data = JSON.parse(String(init.body.get('config')));
    assert.deepEqual(data.provenance, {
      title: 'Intro #1.MP4', creator: 'No declarado', source_url: 'local:Intro%20%231.MP4',
      permission: 'Archivo local aportado para esta edición; licencia no declarada.', attribution: '',
    });
    return Response.json(ref);
  });
  assert.deepEqual(await uploadFullDemoBumper(new File(['test'], 'Intro #1.MP4')), ref);
  for (const file of [new File(['test'], 'clip.mov'), new File(['test'], 'clip.mp4', { type: 'audio/mp4' }), new File([], 'empty.mp4')]) {
    await assert.rejects(uploadFullDemoBumper(file), /MP4/);
  }
  const oversized = new File(['test'], 'huge.mp4');
  Object.defineProperty(oversized, 'size', { value: 2 * 1024 ** 3 });
  await assert.rejects(uploadFullDemoBumper(oversized), /2 GB/);
  assert.equal(calls, 1);
});

test('a local file declaration bounds its title in UTF-8 bytes like the server', () => {
  const long = localFileProvenance(new File(['test'], `${'ñ'.repeat(150)}.mp4`));
  assert.equal(long.title, 'ñ'.repeat(100));
  assert.equal(long.source_url, `local:${encodeURIComponent('ñ'.repeat(100))}`);
});
