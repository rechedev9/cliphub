// Release evals for TickCut Studio 2.4.8. This launches the real Electron
// application and drives the renderer with Playwright. Expensive/external
// stream stages use controlled same-origin responses.

import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { after, before, test } from 'node:test';
import { E2E_BOOT_DEADLINE_MS } from '../scripts/e2e-boot-budget.mjs';
import { createE2EProfile } from '../scripts/e2e-profile.mjs';

const require = createRequire(import.meta.url);
const { _electron } = require('playwright-core');
const desktopRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const artifactsDir = join(desktopRoot, 'e2e', 'artifacts', 'release-2.4.8');
const bootstrapPath = join(desktopRoot, 'e2e', 'isolated-userdata.cjs');
/** @type {import('playwright-core').ElectronApplication} */
let app;
/** @type {import('playwright-core').Page} */
let page;
let origin;
const pageErrors = [];
const profile = createE2EProfile('release-2.4.8');

before(async () => {
  mkdirSync(artifactsDir, { recursive: true });
  app = await _electron.launch({
    executablePath: require('electron'),
    args: [bootstrapPath],
    cwd: desktopRoot,
    env: profile.environment(),
  });
  page = await app.firstWindow();
  page.on('pageerror', (error) => pageErrors.push({ url: page.url(), message: String(error), stack: error.stack }));
  await page.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/matches/, {
    timeout: E2E_BOOT_DEADLINE_MS,
  });
  origin = new URL(page.url()).origin;
});

after(async () => {
  if (page && !page.isClosed()) {
    // The release evaluation seeds fake reel intents below. Remove them even
    // when an assertion fails so a later Electron suite never rehydrates test
    // IDs against a real empty orchestrator.
    await page.evaluate(() => window.localStorage.removeItem('tickcut.reels.v1')).catch(() => {});
    await page.screenshot({ path: join(artifactsDir, 'final-state.png'), fullPage: true }).catch(() => {});
  }
  await app?.close().catch(() => {});
  const studioLog = join(profile.root, 'studio.log');
  if (existsSync(studioLog)) {
    copyFileSync(studioLog, join(artifactsDir, 'studio.log'));
  }
  profile.dispose();
});

async function goto(pathname) {
  await page.goto(`${origin}${pathname}`);
  await page.waitForLoadState('domcontentloaded');
}

async function screenshot(name) {
  await page.screenshot({ path: join(artifactsDir, name), fullPage: true });
}

test('installed release exposes version-only desktop settings with no MCP surface', async () => {
  await goto('/settings');
  await page.getByText('Versión', { exact: true }).waitFor();
  assert.equal(await page.getByText(/Consulta la versión instalada de TickCut Studio\./).isVisible(), true);
  const bridgeShape = await page.evaluate(() => ({
    hasRetiredMCPConfig: typeof window.tickcutSettings?.getMCPConfig === 'function',
  }));
  assert.deepEqual(bridgeShape, { hasRetiredMCPConfig: false });
  const body = await page.locator('body').innerText();
  assert.match(body, /09\s+AJUSTES/);
  assert.doesNotMatch(body, /servidor MCP|configuraci[oó]n MCP/i);
  await screenshot('settings-version-only.png');

  const installedExecutable = join(process.env.LOCALAPPDATA, 'Programs', 'TickCut Studio', 'TickCut Studio.exe');
  assert.equal(existsSync(installedExecutable), true, `installed executable missing: ${installedExecutable}`);
  const installedUserData = mkdtempSync(join(tmpdir(), 'tickcut-installed-e2e-'));
  const installedApp = await _electron.launch({
    executablePath: installedExecutable,
    args: [`--user-data-dir=${installedUserData}`],
  });
  try {
    const installedPage = await installedApp.firstWindow();
    await installedPage.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/matches/, {
      timeout: E2E_BOOT_DEADLINE_MS,
    });
    const installedOrigin = new URL(installedPage.url()).origin;
    await installedPage.getByRole('link', { name: 'Ajustes' }).click();
    await installedPage.waitForURL(`${installedOrigin}/settings`);
    const installedInfo = installedPage.locator('[aria-labelledby="studio-info-title"]');
    await installedInfo.getByText('Versión', { exact: true }).waitFor();
    assert.equal(await installedPage.getByText(/Consulta la versión instalada de TickCut Studio\./).isVisible(), true);
    const installedBridgeShape = await installedPage.evaluate(() => ({
      hasRetiredMCPConfig: typeof window.tickcutSettings?.getMCPConfig === 'function',
    }));
    assert.deepEqual(installedBridgeShape, { hasRetiredMCPConfig: false });
    assert.doesNotMatch(await installedPage.locator('body').innerText(), /servidor MCP|configuraci[oó]n MCP/i);
    assert.equal(await installedInfo.getByText('2.4.8', { exact: true }).isVisible(), true);
    await installedPage.screenshot({ path: join(artifactsDir, 'installed-settings-2.4.8.png'), fullPage: true });
  } finally {
    await installedApp.close();
    rmSync(installedUserData, { force: true, recursive: true });
  }
});

test('demo reel requires the exact creative brief and keeps publication metadata isolated', async () => {
  await goto('/matches/m-inferno');
  await page.getByRole('heading', { name: 'Inferno' }).waitFor();
  const matchText = await page.locator('body').innerText();
  assert.doesNotMatch(matchText, /AHORA MISMO/i);
  assert.match(matchText, /HACE \d+ D/);

  await page.locator('button:has(.lucide-crosshair)').first().click();
  await page.getByText('Brief creativo exacto', { exact: true }).waitFor();
  const forge = page.getByRole('button', { name: 'FORJAR REEL' });
  const approval = page.getByLabel(/Apruebo todas estas decisiones/);
  assert.equal(await forge.isDisabled(), true);
  assert.equal(await approval.isEnabled(), true);
  const brief = page.locator('[aria-labelledby="creative-brief-title"]');
  for (const label of ['Formato', 'HUD / killfeed', 'Efecto de kill', 'Transición', 'Título / contador', 'Intro', 'Outro', 'Música', 'Portada']) {
    assert.equal(await brief.getByText(`${label}:`, { exact: true }).isVisible(), true, `missing ${label}`);
  }
  await approval.check();
  assert.equal(await forge.isEnabled(), true);

  await page.setViewportSize({ width: 960, height: 900 });
  const forgeBox = await forge.boundingBox();
  assert.ok(forgeBox, 'the forge CTA has no layout box at 960px');
  await screenshot('demo-brief-responsive-960.png');
  await page.setViewportSize({ width: 1280, height: 900 });

  await page.getByRole('button', { name: '16:9' }).click();
  assert.equal(await approval.isChecked(), false, 'changing the brief must revoke approval');
  assert.equal(await forge.isDisabled(), true);
  await approval.check();

  const alphaJob = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
  const betaJob = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
  const defaultEdit = {
    format: 'short-9x16', killEffect: 'punch-in', transition: 'flash', intro: false,
    outro: false, hookText: false, killCounter: false, coverStrategy: 'generated-gameplay',
    introText: '', outroText: '',
  };
  const intents = [
    { videoId: `${alphaJob}__seg-alpha`, jobId: alphaJob, segmentIds: ['seg-alpha'], mode: 'clean', variant: 'viral-60-clean', editConfig: defaultEdit, title: 'Reel Alpha', map: 'Inferno', score: '13-2', createdAt: Date.now() },
    { videoId: `${betaJob}__seg-beta`, jobId: betaJob, segmentIds: ['seg-beta'], mode: 'clean', variant: 'viral-60-clean', editConfig: defaultEdit, title: 'Reel Beta', map: 'Mirage', score: '13-9', createdAt: Date.now() - 1_000 },
  ];
  const publishPayload = (title) => ({
    schema_version: '1.0',
    metadata: { title, description: `${title} description`, tags: ['CS2', 'Shorts'] },
    recommendations: [1, 2, 3].map((index) => ({
      title: `${title} option ${index}`,
      description: `${title} description ${index}`,
      keywords: [title, 'CS2'],
      tags: ['CS2', 'Shorts'],
      score: 90 - index,
      rationale: `Factual option ${index}`,
    })),
    keywords: [title, 'CS2'],
    tags: ['CS2', 'Shorts'],
    schedule: {
      time_zone: 'Europe/Madrid',
      generated_at: '2026-07-20T12:00:00Z',
      days: [{
        date: '2026-07-21', weekday: 'martes',
        slots: [{ publish_at: '2026-07-21T18:00:00Z', local_time: '20:00', source: 'baseline', confidence: 0.7, score: 0.8, rationale: 'Eval slot' }],
      }],
      sources: [{ title: 'YouTube help', url: 'https://support.google.com/youtube/' }],
      caveat: 'Eval schedule only.',
    },
    trends: { available: false, terms: [], sources: [], reason: 'No trends in eval.' },
    studio_url: 'https://studio.youtube.com/',
  });
  await page.route('**/api/demos/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const jobId = [alphaJob, betaJob].find((id) => path.includes(id)) ?? null;
    if (jobId && path.endsWith('/status')) return route.fulfill({ json: { status: 'done' } });
    if (jobId && path.endsWith('/renders/viral-60-clean')) {
      const stem = jobId === alphaJob ? 'alpha' : 'beta';
      return route.fulfill({ json: { status: 'ready', videos: [`${stem}.mp4`], covers: [`${stem}.jpg`] } });
    }
    if (jobId && path.endsWith('/publish-assistant')) {
      return route.fulfill({ json: publishPayload(jobId === alphaJob ? 'Reel Alpha' : 'Reel Beta') });
    }
    return route.fulfill({ status: 404, json: { error: `unhandled demo eval route ${path}` } });
  });
  await page.evaluate((value) => window.localStorage.setItem('tickcut.reels.v1', JSON.stringify(value)), intents);
  await goto('/videos');
  await page.getByText('Reel Alpha', { exact: true }).waitFor();
  await page.getByText('Reel Beta', { exact: true }).waitFor();

  const alphaCard = page.locator('article').filter({ hasText: 'Reel Alpha' });
  await alphaCard.getByRole('button', { name: 'PREPARAR PUBLICACIÓN' }).click();
  const publishDialog = page.getByRole('dialog');
  const title = publishDialog.getByRole('textbox', { name: 'Título', exact: true });
  await title.waitFor();
  assert.equal(await title.inputValue(), 'Reel Alpha');
  await title.fill('NO DEBE HEREDARSE');
  await page.keyboard.press('Escape');

  const betaCard = page.locator('article').filter({ hasText: 'Reel Beta' });
  await betaCard.getByRole('button', { name: 'PREPARAR PUBLICACIÓN' }).click();
  const secondTitle = page.getByRole('dialog').getByRole('textbox', { name: 'Título', exact: true });
  await secondTitle.waitFor();
  assert.equal(await secondTitle.inputValue(), 'Reel Beta');
  assert.notEqual(await secondTitle.inputValue(), 'NO DEBE HEREDARSE');
  await screenshot('publication-metadata-isolated.png');
  await page.keyboard.press('Escape');
  // The real client keeps rehydrated intents in memory, but this durable store
  // is what a later Electron launch reads. Clearing it prevents test-only job
  // IDs leaking into the shared isolated profile.
  await page.evaluate(() => window.localStorage.removeItem('tickcut.reels.v1'));
  await page.unroute('**/api/demos/**');
});

test('player selection is reversible and does not parse until CONTINUAR', async () => {
  const jobId = '11111111-1111-4111-8111-111111111111';
  let parseRequests = 0;
  const roster = {
    players: [
      { steamid64: '76561198000000001', name: 'Alpha', team: 'CT', kills: 24, deaths: 12, assists: 4, headshots: 12, mvps: 3, rounds: 24, adr: 98, hs_pct: 50, kast: 75, rating: 1.34, rounds_2k: 2, rounds_3k: 1, rounds_4k: 0, rounds_5k: 0 },
      { steamid64: '76561198000000002', name: 'Bravo', team: 'T', kills: 18, deaths: 15, assists: 6, headshots: 9, mvps: 1, rounds: 24, adr: 82, hs_pct: 50, kast: 70, rating: 1.08, rounds_2k: 1, rounds_3k: 0, rounds_4k: 0, rounds_5k: 0 },
    ],
    match: { map: 'de_mirage', score_ct: 13, score_t: 9, rounds: 22 },
  };
  await page.route('**/api/demos/scan', async (route) => route.fulfill({ json: { jobId } }));
  await page.route(`**/api/demos/${jobId}/status`, async (route) => route.fulfill({ json: { status: 'scanned' } }));
  await page.route(`**/api/demos/${jobId}/roster`, async (route) => route.fulfill({ json: roster }));
  await page.route(`**/api/demos/${jobId}/parse`, async (route) => {
    parseRequests += 1;
    await route.fulfill({ json: { status: 'queued' } });
  });

  await goto('/upload');
  await page.locator('input[type="file"]').setInputFiles({
    name: 'qa-player-picker.dem',
    mimeType: 'application/octet-stream',
    buffer: Buffer.from('HL2DEMO release eval'),
  });
  await page.getByRole('button', { name: /Bravo/ }).waitFor();
  await page.getByRole('button', { name: /Bravo/ }).click();
  assert.equal(parseRequests, 0);
  const continueButton = page.getByRole('button', { name: 'CONTINUAR' });
  assert.equal(await continueButton.isVisible(), true);
  assert.equal(await page.getByRole('button', { name: /Bravo/ }).getAttribute('aria-pressed'), 'true');
  assert.equal(await page.getByRole('button', { name: /Alpha/ }).getAttribute('aria-pressed'), 'false');
  await screenshot('player-picker-explicit-continue.png');

  await page.unroute('**/api/demos/scan');
  await page.unroute(`**/api/demos/${jobId}/status`);
  await page.unroute(`**/api/demos/${jobId}/roster`);
  await page.unroute(`**/api/demos/${jobId}/parse`);
});

test('stream editor validates, recovers, previews, reports progress, switches layout, and exposes delivery', async () => {
  const jobId = 'eval-stream-2210';
  const basePlan = {
    schema_version: 'stream-edit-v1',
    variant: 'streamer-vertical-stack-40-60',
    face_crop: { x: 0.72, y: 0.03, width: 0.24, height: 0.30 },
    face_crop_reviewed: false,
    gameplay_crop: { x: 0, y: 0, width: 1, height: 1 },
    clips: [{ id: 'clip-1', start_seconds: 0, end_seconds: 15.112, title: '' }],
    music: {},
    effects: { grade: true },
  };
  let persistedPlan = basePlan;
  const job = {
    id: jobId,
    status: 'ready',
    title: 'vaya saco..',
    source_url: 'https://clips.twitch.tv/CulturedStormyKittenAllenHuhu-sMpul952n8rWF81N',
    probe: { width: 1920, height: 1080, duration_seconds: 15.112, audio_codec: 'aac' },
    edit_plan: persistedPlan,
    created_at: '2026-07-20T10:00:00Z',
    updated_at: '2026-07-20T10:05:00Z',
  };
  let autosaveFails = false;
  const rendered = {
    status: 'rendered',
    published: true,
    videos: [{ clip_id: 'clip-1', title: 'vaya saco..', key: 'final.mp4', duration_seconds: 15.112 }],
    delivery: [
      { name: 'final.mp4', kind: 'video', key: 'shortslistosparasubir/final.mp4' },
      { name: 'cover.jpg', kind: 'cover', key: 'shortslistosparasubir/cover.jpg' },
      { name: 'edit-plan.json', kind: 'plan', key: 'shortslistosparasubir/edit-plan.json' },
      { name: 'manifest.json', kind: 'manifest', key: 'shortslistosparasubir/manifest.json' },
    ],
  };

  await page.route('**/api/streams**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();
    if (path === '/api/streams' && method === 'GET') {
      return route.fulfill({ json: { jobs: [{ ...job, edit_plan: persistedPlan }] } });
    }
    if (path === `/api/streams/${jobId}/source`) {
      return route.fulfill({ status: 200, contentType: 'video/mp4', body: Buffer.from('invalid-media-for-actionable-preview-error') });
    }
    if (path === `/api/streams/${jobId}/edit-plan` && method === 'GET') return route.fulfill({ json: persistedPlan });
    if (path === `/api/streams/${jobId}/edit-plan` && method === 'PUT') {
      if (autosaveFails) return route.fulfill({ status: 503, json: { code: 'service_unavailable', error: 'eval autosave offline' } });
      persistedPlan = {
        ...JSON.parse(request.postData() || '{}'),
        updated_at: '2026-07-20T10:06:00Z',
      };
      return route.fulfill({ json: persistedPlan });
    }
    if (path === `/api/streams/${jobId}/renders/streamer-fullframe-nocam` && method === 'POST') {
      return route.fulfill({ json: { status: 'queued', videos: [] } });
    }
    if (path === `/api/streams/${jobId}/renders/streamer-fullframe-nocam` && method === 'GET') {
      return route.fulfill({ json: rendered });
    }
    return route.fulfill({ status: 404, json: { error: `unhandled eval route ${method} ${path}` } });
  });

  await page.evaluate((id) => window.localStorage.removeItem(`tickcut.stream-draft.${id}`), jobId);
  await goto('/streams');
  await page.getByRole('button', { name: 'TRAER CLIP' }).click();
  const urlAlert = page.getByRole('alert').filter({ hasText: 'Pega una URL de clip o VOD' });
  await urlAlert.waitFor();
  assert.equal(await page.locator('#stream-url').getAttribute('aria-invalid'), 'true');
  assert.equal(await page.getByText('CONTINUAR BORRADOR', { exact: true }).isVisible(), true);
  await page.getByText('CONTINUAR BORRADOR', { exact: true }).click();

  const endInput = page.getByLabel('Fin (s)').first();
  const titleInput = page.getByLabel('Título (opcional)').last();
  await endInput.waitFor();
  assert.equal(Number(await endInput.inputValue()), 15.112);
  assert.equal(await titleInput.inputValue(), 'vaya saco..');

  await page.getByRole('button', { name: 'CREAR SHORTS' }).click();
  await page.getByText(/Confirma manualmente el recorte de facecam/).waitFor();
  assert.equal(await page.getByText(/podría coincidir con el radar/).isVisible(), true);

  await endInput.fill('20');
  const rangeAlert = page.getByRole('alert').filter({ hasText: 'el fin supera la duración del vídeo (15.11 s)' });
  await rangeAlert.waitFor();
  assert.doesNotMatch(await rangeAlert.innerText(), new RegExp(jobId));
  await endInput.fill('15.112');

  const previewAlert = page.getByText(/No se pudo decodificar o leer el MP4|No se pudo decodificar la pista de audio/).first();
  await previewAlert.waitFor({ timeout: 10_000 });
  assert.equal(await page.getByRole('button', { name: 'REINTENTAR VISTA PREVIA' }).isVisible(), true);

  autosaveFails = true;
  await titleInput.fill('Borrador QA recuperado');
  await page.waitForTimeout(800);
  await goto('/matches');
  await goto('/streams');
  await page.getByText('CONTINUAR BORRADOR', { exact: true }).click();
  const recoveredTitle = page.getByLabel('Título (opcional)').last();
  await recoveredTitle.waitFor();
  assert.equal(await recoveredTitle.inputValue(), 'Borrador QA recuperado');

  await page.getByRole('button', { name: /Full-frame/ }).click();
  await page.getByText(/No hace falta recorte de facecam/).waitFor();
  assert.equal(await page.getByText(/Recorte de facecam: arrastra/).count(), 0);
  autosaveFails = false;
  await page.getByRole('button', { name: 'CREAR SHORTS' }).click();
  await page.getByRole('heading', { name: 'Paquete shortslistosparasubir' }).waitFor({ timeout: 10_000 });
  for (const artifact of ['final.mp4', 'cover.jpg', 'edit-plan.json', 'manifest.json']) {
    assert.equal(await page.getByText(artifact, { exact: true }).isVisible(), true, `missing delivery ${artifact}`);
  }
  await screenshot('stream-editor-complete-eval.png');
  await page.unroute('**/api/streams**');
});

test('release eval produced no uncaught renderer exceptions', () => {
  assert.deepEqual(pageErrors, []);
});
