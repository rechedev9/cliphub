import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { FULL_DEMO_EDIT } from '../lib/full-demo.ts';
import { buildEditRequest } from '../lib/api/edit-request.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const VIDEO = `${JOB}__demo-compilation`;
const VARIANT = 'gameplay-pov-60';
const titles = [
  ['Bajas destacadas', 'donk Drops 34 KILLS on FACEIT! POV with COMMS (Mirage)'],
  ['POV clásico', 'donk POV with COMMS on FACEIT (Mirage)'],
  ['Sesión de juego', 'donk Plays FACEIT! POV with COMMS (Mirage)'],
  ['Mapa protagonista', 'Mirage FACEIT — donk POV with COMMS | CS2'],
  ['Duelo de la demo', 'donk vs w0nderful on FACEIT! POV with COMMS (Mirage)'],
];

for (const width of [390, 1440]) {
  test(`long-video publication templates can be selected, edited and copied at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 });
    await page.addInitScript(({ job, video, variant, edit }) => {
      localStorage.setItem('cliphub.reels.v1', JSON.stringify([{
        videoId: video, jobId: job, segmentIds: ['demo-compilation'], mode: 'clean', variant,
        editConfig: edit,
        title: 'donk en Mirage', map: 'de_mirage', score: '13-9', targetName: 'donk', createdAt: Date.now(),
      }]));
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: {
        writeText: async (text: string) => { localStorage.setItem('test.copied', text); },
      } });
    }, { job: JOB, video: VIDEO, variant: VARIANT, edit: FULL_DEMO_EDIT });
    await page.route('**/api/streams', (route) => route.fulfill({ json: { jobs: [] } }));
    await page.route('**/api/demos/batch-status?*', (route) => route.fulfill({ status: 404, json: {} }));
    await page.route(`**/api/demos/${JOB}/plan`, (route) => route.fulfill({ json: {
      demo: { map: 'de_mirage' }, target: { steamid64: '76561198000000001', name_in_demo: 'donk' }, segments: [], stats: { total_kills_target: 34 },
    } }));
    await page.route('**/api/demos/jobs', (route) => route.fulfill({ json: { jobs: [{ jobId: JOB, status: 'done', createdAt: '2026-09-01T10:00:00Z' }] } }));
    await page.route(`**/api/demos/${JOB}/status`, (route) => route.fulfill({ json: { status: 'done' } }));
    await page.route(`**/api/demos/${JOB}/roster`, (route) => route.fulfill({ json: {
      players: [{ steamid64: '76561198000000001', name: 'donk', team: 'T', kills: 34 }],
      match: { map: 'de_mirage', score_ct: 9, score_t: 13, rounds: 22 },
    } }));
    await page.route(`**/api/demos/${JOB}/renders/${VARIANT}`, (route) => route.fulfill({ json: {
      status: 'ready', videos: ['demo-compilation.mp4'], covers: [], segment_ids: ['demo-compilation'],
      edit: buildEditRequest(FULL_DEMO_EDIT),
    } }));
    const recommendations = titles.map(([template, title]) => ({
      template, title, description: 'donk POV with COMMS on Mirage (FACEIT).\nKills in this video: 34\n#CS2 #POV',
      tags: ['CS2', 'donk', 'Mirage', 'FACEIT'], keywords: ['donk', 'Mirage'], score: 0, rationale: 'Datos del render terminado.',
    }));
    await page.route('**/publish-assistant?*', (route) => route.fulfill({ json: {
      schema_version: '1.0', studio_url: 'https://studio.youtube.com/', metadata: recommendations[0], recommendations,
      tags: recommendations[0].tags, keywords: recommendations[0].keywords,
      schedule: { time_zone: 'Europe/Madrid', generated_at: '2026-09-16T10:00:00Z', days: [{
        date: '2026-09-17', weekday: 'jueves', slots: [{ publish_at: '2026-09-17T18:00:00Z', local_time: '20:00', source: 'baseline', confidence: 0.5, score: 0.5, rationale: 'Referencia.' }],
      }], sources: [], caveat: 'Referencia horaria.' },
      trends: { available: false, terms: [], reason: 'Plantillas basadas en el vídeo y la demo.' },
    } }));
    await gotoStudio(page, `/clips/${JOB}/publicar/${encodeURIComponent(VIDEO)}`);
    await expect(page.getByRole('heading', { name: 'Plantillas para vídeo largo' })).toBeVisible();
    await expect(page.getByLabel('Título', { exact: true })).toHaveValue(titles[0][1]);
    await page.getByRole('button', { name: `Usar título recomendado: ${titles[4][1]}` }).click();
    await expect(page.getByLabel('Título', { exact: true })).toHaveValue(titles[4][1]);
    await expect(page.getByLabel('Descripción', { exact: true })).not.toHaveValue(/#Shorts/);
    await page.getByLabel('Título', { exact: true }).fill('Mi POV de Mirage');
    await page.getByLabel('Descripción', { exact: true }).fill('Descripción revisada');
    await page.getByLabel('Etiquetas, separadas por comas').fill('CS2, Mirage, mi canal');
    await page.getByRole('button', { name: 'Copiar todo', exact: true }).click();
    await expect.poll(() => page.evaluate(() => localStorage.getItem('test.copied'))).toBe('Mi POV de Mirage\n\nDescripción revisada\n\nCS2, Mirage, mi canal');
    await expect(page.getByRole('button', { name: 'Descargar MP4' })).toBeEnabled();
    const overflow = await page.evaluate(() => ({
      width: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth,
      elements: [...document.querySelectorAll('main *')].filter((element) => element.getBoundingClientRect().right > document.documentElement.clientWidth).slice(0, 8).map((element) => ({ tag: element.tagName, class: element.className, text: element.textContent?.slice(0, 80) })),
    }));
    expect(overflow.scroll, JSON.stringify(overflow)).toBeLessThanOrEqual(overflow.width);
    await page.screenshot({ path: `test-results/long-video-publish-${width}.png`, fullPage: true });
  });
}

test('failed long videos show an error instead of waiting copy', async ({ page }) => {
  await page.addInitScript(({ job, video, variant, edit }) => {
    localStorage.setItem('cliphub.reels.v1', JSON.stringify([{
      videoId: video, jobId: job, segmentIds: ['demo-compilation'], mode: 'clean', variant,
      editConfig: edit,
      title: 'donk en Mirage', map: 'de_mirage', score: '13-9', targetName: 'donk', createdAt: Date.now(),
    }]));
  }, { job: JOB, video: VIDEO, variant: VARIANT, edit: FULL_DEMO_EDIT });
  await page.route('**/api/streams', (route) => route.fulfill({ json: { jobs: [] } }));
  await page.route('**/api/demos/batch-status?*', (route) => route.fulfill({ json: {
    items: [{
      job_id: JOB, variant: VARIANT,
      job: { status: 'failed', failure_reason: 'ffmpeg exited 1' },
      render: { status: 'failed', error: 'ffmpeg exited 1' },
    }],
  } }));
  await page.route(`**/api/demos/${JOB}/plan`, (route) => route.fulfill({ json: {
    demo: { map: 'de_mirage' }, target: { steamid64: '76561198000000001', name_in_demo: 'donk' }, segments: [], stats: { total_kills_target: 34 },
  } }));
  await page.route('**/api/demos/jobs', (route) => route.fulfill({ json: { jobs: [{ jobId: JOB, status: 'failed', createdAt: '2026-09-01T10:00:00Z' }] } }));
  await page.route(`**/api/demos/${JOB}/status`, (route) => route.fulfill({ json: { status: 'failed', failure_reason: 'ffmpeg exited 1' } }));
  await page.route(`**/api/demos/${JOB}/roster`, (route) => route.fulfill({ json: {
    players: [{ steamid64: '76561198000000001', name: 'donk', team: 'T', kills: 34 }],
    match: { map: 'de_mirage', score_ct: 9, score_t: 13, rounds: 22 },
  } }));
  await page.route(`**/api/demos/${JOB}/renders/${VARIANT}`, (route) => route.fulfill({ json: {
    status: 'failed', videos: [], covers: [], segment_ids: ['demo-compilation'], error: 'ffmpeg exited 1',
  } }));
  await gotoStudio(page, `/clips/${JOB}/publicar/${encodeURIComponent(VIDEO)}`);
  await expect(page.getByRole('alert').getByText('No se pudo preparar la publicación porque el vídeo falló. Reintenta el render desde Clips.')).toBeVisible();
  await expect(page.getByText('La preparación para YouTube estará disponible cuando el vídeo esté listo.')).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Plantillas para vídeo largo' })).toHaveCount(0);
});
