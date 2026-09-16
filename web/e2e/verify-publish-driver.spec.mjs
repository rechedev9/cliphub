import { mkdirSync } from 'node:fs';
import { expect, test } from '@playwright/test';
import { drivePublicarVideoLargo, findLongVideoPublish, waitForPublishSettled } from '../../.cursor/skills/verify-cliphub/control-cliphub.mjs';
import { gotoStudio } from './contract.ts';
import { FULL_DEMO_EDIT } from '../lib/full-demo.ts';
import { DEFAULT_EDIT_CONFIG } from '../lib/api/reel-store.ts';
import { buildEditRequest } from '../lib/api/edit-request.ts';

const JOBS = [1, 2, 3].map((n) => `${n}1111111-1111-4111-8111-111111111111`);
const FULL_VARIANT = 'gameplay-pov-60';
const SHORT_VARIANT = 'viral-60-clean';
const LONG_VIDEO = `${JOBS[2]}__demo-compilation`;

// Fixture API exercises the real hub's exclusive accordion and column markup.
// This is a regression test for the driver, not evidence of a live render.
async function readyHub(page) {
  const intents = JOBS.map((jobId, index) => ({
    videoId: `${jobId}__seg-001`, jobId, segmentIds: ['seg-001'], mode: 'clean',
    variant: SHORT_VARIANT, editConfig: DEFAULT_EDIT_CONFIG, title: `Short ${index}`,
    map: 'de_mirage', score: '13-9', targetName: 'donk', createdAt: Date.parse(`2026-09-${16-index}T10:00:00Z`),
  }));
  intents.push({ ...intents[2], videoId: LONG_VIDEO, segmentIds: ['demo-compilation'],
    variant: FULL_VARIANT, editConfig: FULL_DEMO_EDIT, title: 'POV largo' });
  await page.addInitScript((seed) => {
    localStorage.setItem('cliphub.reels.v1', JSON.stringify(seed));
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: {
      writeText: async (text) => { localStorage.setItem('test.driver.copied', text); },
    } });
  }, intents);
  const render = (intent) => ({ status: 'ready', videos: [`${intent.segmentIds[0]}.mp4`], covers: [],
    segment_ids: intent.segmentIds, edit: buildEditRequest(intent.editConfig) });
  await page.route('**/api/streams', (route) => route.fulfill({ json: { jobs: [] } }));
  await page.route('**/api/demos/jobs', (route) => route.fulfill({ json: { jobs: JOBS.map((jobId, index) => ({
    jobId, status: 'done', targetSteamId: '76561198000000001', createdAt: new Date(intents[index].createdAt).toISOString(),
  })) } }));
  await page.route('**/api/demos/batch-status?*', (route) => route.fulfill({ json: {
    items: intents.map((intent) => ({ job_id: intent.jobId, variant: intent.variant, job: { status: 'done' }, render: render(intent) })),
  } }));
  for (const job of JOBS) {
    await page.route(`**/api/demos/${job}/status`, (route) => route.fulfill({ json: { status: 'done' } }));
    await page.route(`**/api/demos/${job}/plan`, (route) => route.fulfill({ json: {
      demo: { map: 'de_mirage' }, target: { steamid64: '76561198000000001', name_in_demo: 'donk' },
      segments: [], stats: { total_kills_target: 34 },
    } }));
    await page.route(`**/api/demos/${job}/roster`, (route) => route.fulfill({ json: {
      players: [{ steamid64: '76561198000000001', name: 'donk', team: 'T', kills: 34 }],
      match: { map: 'de_mirage', score_ct: 9, score_t: 13, rounds: 22 },
    } }));
  }
  for (const intent of intents) {
    await page.route(`**/api/demos/${intent.jobId}/renders/${intent.variant}`, (route) => route.fulfill({ json: render(intent) }));
  }
}

test('driver reaches a long video in the third partida and waits for its metadata', async ({ page }, testInfo) => {
  await readyHub(page);
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  let requested = false;
  const recommendations = ['POV', 'Sesión', 'Mapa'].map((template) => ({
    template, title: `donk ${template} (Mirage)`, description: 'POV de la demo',
    tags: ['CS2', 'Mirage'], keywords: ['donk', 'Mirage'], score: 0, rationale: 'Datos del vídeo.',
  }));
  await page.route('**/publish-assistant?*', async (route) => {
    requested = true;
    await gate;
    await route.fulfill({ json: {
      schema_version: '1.0', studio_url: 'https://studio.youtube.com/', recommendations,
      metadata: recommendations[0], tags: recommendations[0].tags, keywords: recommendations[0].keywords,
      schedule: { time_zone: 'Europe/Madrid', generated_at: '2026-09-16T10:00:00Z', days: [{
        date: '2026-09-17', weekday: 'jueves', slots: [{ publish_at: '2026-09-17T18:00:00Z', local_time: '20:00', source: 'baseline', confidence: 0.5, score: 0.5, rationale: 'Referencia.' }],
      }], sources: [], caveat: 'Referencia horaria.' },
      trends: { available: false, terms: [] },
    } });
  });
  await gotoStudio(page, '/clips');
  await expect(page.locator('article[id^="partida-"]')).toHaveCount(3);
  // Establish that both formats are ready before starting with every row shut.
  const third = page.locator(`#partida-${JOBS[2]}`);
  await third.getByRole('button', { expanded: false }).click();
  await expect(third.getByRole('link', { name: 'Publicar', exact: true })).toHaveCount(2);
  await third.getByRole('button', { expanded: true }).click();
  await expect(page.locator('button[aria-expanded="true"]')).toHaveCount(0);

  const evidenceDir = testInfo.outputPath('driver');
  mkdirSync(evidenceDir, { recursive: true });
  const drive = drivePublicarVideoLargo(page, new URL(page.url()).origin, evidenceDir);
  void drive.catch(() => {}); // Keep a failed driver observable while the gate is held.
  try {
    await expect.poll(() => requested).toBe(true);
    await expect(page.getByRole('status').filter({ hasText: 'Preparando metadatos y horario' })).toBeVisible();
    const earlyResult = await Promise.race([drive.then(() => 'finished'), new Promise((resolve) => setTimeout(() => resolve('pending'), 100))]);
    expect(earlyResult).toBe('pending');
  } finally {
    release();
  }
  const report = await drive;
  expect(report.url).toContain(`/publicar/${encodeURIComponent(LONG_VIDEO)}`);
  expect(report.templates_reachable).toBe(true);
  expect(report.steps.find((step) => step.id === 'publicar-hub').result).toMatchObject({ expanded_rows: 3, inspected_rows: JOBS.map((job, index) => ({ id: `partida-${job}`, opened: true, long_publish: index === 2 ? 1 : 0 })), long_publish_links: 1 });
  expect(report.steps.find((step) => step.id === 'publicar-templates').result.edited).toBe(true);
  await expect.poll(() => page.evaluate(() => localStorage.getItem('test.driver.copied'))).toBe('POV editado en verify\n\nDescripción editada en verify\n\nCS2, verify');
});

test('driver skips disabled partidas and does not mistake a Short for a long video', async ({ page }) => {
  await page.setContent(`<article id="partida-disabled"><button aria-expanded="false" aria-disabled="true">Parsing</button></article>
    <article id="partida-short"><button aria-expanded="true">Ready</button><div>
      <div><div><span>Shorts</span></div><a href="/short">Publicar</a></div>
      <div><div><span>Vídeos largos · 16:9</span></div><p>No long videos</p></div>
    </div></article>`);
  const found = await findLongVideoPublish(page);
  expect(found).toMatchObject({ door: null, expandedRows: 1, inspected: [
    { id: 'partida-disabled', opened: false, long_publish: 0 },
    { id: 'partida-short', opened: true, long_publish: 0 },
  ], longPublishCount: 0, publishCount: 1 });
});

for (const [name, html] of [
  ['missing clip', '<h1>Clip no encontrado</h1>'],
  ['failed render', '<div role="alert">No se pudo preparar la publicación porque el vídeo falló. Reintenta el render desde Clips.</div>'],
  ['assistant error', '<div role="alert">No se pudo preparar la publicación. El MP4 sigue disponible para descargar.</div>'],
  ['waiting render', '<p>La preparación para YouTube estará disponible cuando el vídeo esté listo y su revisión resuelta.</p>'],
]) {
  test(`settled publication accepts ${name}`, async ({ page }) => {
    await page.setContent(html);
    await waitForPublishSettled(page);
  });
}
