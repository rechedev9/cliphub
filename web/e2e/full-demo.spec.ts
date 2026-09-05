import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { FULL_DEMO_EMPTY } from '../lib/full-demo.ts';
import { isFullDemoSnapshot, isFullDemoOptions, type FullDemoDocument } from '../lib/full-demo-plan.ts';
import { PRODUCE_MATCH_MISSING, PRODUCE_SHORT_TITLE } from '../lib/produce/copy.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const PRODUCE_FULL = `/clips/${JOB}/nuevo?formato=full`;
const REC_CTA = 'Crear Full Demo';
const BRIEF_CHECKBOX = /He revisado y apruebo los ajustes/;
const PLAN = {
  demo: { map: 'de_inferno' }, target: { steamid64: '76561198000000001', name_in_demo: 'ropz', team_at_start: 'CT' },
  stats: { total_kills_target: 24 }, segments: [{ id: 'r1', round: 1, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }] }],
};
const ROSTER = { players: [{ steamid64: '76561198000000001', name: 'ropz', team: 'CT', kills: 24, deaths: 14, assists: 4 }] };
function editorial(): FullDemoDocument {
  const raw: unknown = JSON.parse(readFileSync(new URL('../lib/full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  if (!isFullDemoSnapshot(raw)) throw new Error('Invalid Go editorial fixture');
  return raw.document;
}
async function fulfillJson(page: Page, path: string, status: number, body: unknown): Promise<void> {
  await page.route(`**/api/demos/${JOB}${path}`, (route) => route.fulfill({ status, json: body }));
}
async function stubParsedMatch(page: Page, recap: { status: number; body: unknown }): Promise<void> {
  await fulfillJson(page, '/status', 200, { status: 'parsed' });
  await fulfillJson(page, '/plan', 200, PLAN);
  await fulfillJson(page, '/roster', 200, ROSTER);
  await fulfillJson(page, '/recap-plan', recap.status, recap.body);
  const document = editorial();
  await fulfillJson(page, '/full-demo/plan', 200, { document, defaults: document.options, compatibility: 'editorial-v1' });
}

test.describe('Full POV editorial constructor', () => {
  test('retries an offline editorial load without allowing unplanned defaults', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    let offline = true;
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, (route) => route.fulfill(offline
      ? { status: 503, json: { code: 'service_unavailable', error: 'Sin conexión' } }
      : { status: 200, json: { document: editorial(), defaults: editorial().options, compatibility: 'editorial-v1' } }));
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
    offline = false;
    await page.getByRole('button', { name: 'Reintentar conexión y cargar plan' }).click();
    await expect(page.getByRole('spinbutton', { name: 'Volumen del juego', exact: true })).toBeVisible();
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
    await expect(page.getByRole('button', { name: REC_CTA })).toBeEnabled();
  });
  test('previews the approved assets without queueing capture', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    let captures = 0;
    await page.route(`**/api/demos/${JOB}/generate`, (route) => { captures += 1; return route.fulfill({ status: 500 }); });
    await gotoStudio(page, PRODUCE_FULL);
    const music = editorial().options.audio.music.assets[0];
    const sponsor = editorial().options.sponsor.video;
    if (!music || !sponsor) throw new Error('The fixture requires music and sponsor assets');
    await expect(page.getByLabel('Escuchar pista 1', { exact: true })).toHaveAttribute('src', `/api/editor/assets/${music.id}/media`);
    const video = page.getByLabel('Previsualizar vídeo del sponsor', { exact: true });
    await expect(video).toHaveAttribute('src', `/api/editor/assets/${sponsor.id}/media`);
    await expect(video).toHaveAttribute('preload', 'none');
    await page.getByRole('combobox', { name: 'Audio del anuncio', exact: true }).click();
    await page.getByRole('option', { name: 'Reemplazar por narración', exact: true }).click();
    await expect.poll(() => video.evaluate((element) => element instanceof HTMLMediaElement ? element.volume : -1)).toBe(0);
    expect(captures).toBe(0);
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
  });
  test('switches formats and preserves the numbered Clips section', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, PRODUCE_FULL);
    const key = page.locator('[data-slot="sidebar-menu-button"][href="/clips"]');
    await expect(key).toContainText('01');
    await expect(page.getByRole('heading', { name: 'Full POV Chill' })).toBeVisible();
    await page.getByRole('button', { name: 'Short 9:16', exact: true }).click();
    await expect(page.getByRole('heading', { name: PRODUCE_SHORT_TITLE })).toBeVisible();
  });
  test('a missing job stays distinct from a load failure', async ({ page }) => {
    await fulfillJson(page, '/status', 404, { error: 'not found' });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('heading', { name: PRODUCE_MATCH_MISSING.title })).toBeVisible();
    await expect(page.getByRole('button', { name: REC_CTA })).toHaveCount(0);
  });
  test('a failed base plan is not a missing match', async ({ page }) => {
    await fulfillJson(page, '/status', 200, { status: 'parsed' });
    await fulfillJson(page, '/plan', 500, { error: 'upstream error' });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('heading', { name: FULL_DEMO_EMPTY.error.title })).toBeVisible();
    await expect(page.getByText(PRODUCE_MATCH_MISSING.title)).toHaveCount(0);
  });
  for (const failure of [
    { status: 503, code: 'service_unavailable', error: 'Servicio de análisis sin conexión' },
    { status: 409, code: 'full_demo_facts_insufficient', error: 'Vuelve a analizar el jugador para obtener hechos de rondas' },
  ]) {
    test(`editorial ${failure.code} blocks creation without legacy defaults`, async ({ page }) => {
      await stubParsedMatch(page, { status: 200, body: PLAN });
      await fulfillJson(page, '/full-demo/plan', failure.status, failure);
      await gotoStudio(page, PRODUCE_FULL);
      await expect(page.getByRole('alert').filter({ hasText: failure.error })).toBeVisible();
      await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
      await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
    });
  }
  test('includes zero-kill rounds independently of an unavailable legacy recap', async ({ page }) => {
    await stubParsedMatch(page, { status: 409, body: { error: 'legacy recap unavailable' } });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByText('R01', { exact: true }).filter({ visible: true })).toBeVisible();
    await expect(page.getByText('R02', { exact: true })).toBeVisible();
    await expect(page.getByText('0 kills', { exact: true })).toHaveCount(2);
    const cta = page.getByRole('button', { name: REC_CTA });
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
    await expect(cta).toBeEnabled();
    await expect(page.getByText(/Este formato necesita acceso a FACEIT/)).toHaveCount(0);
  });
  test('missing enabled music and sponsor are actionable blockers', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    const document = editorial();
    document.options.audio.music.assets = []; document.options.sponsor.video = null;
    document.blockers = [{ code: 'full_demo_asset_missing', message: 'Missing required media', round_id: undefined }];
    await fulfillJson(page, '/full-demo/plan', 200, { document, defaults: document.options, compatibility: 'editorial-v1' });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByText('Añade al menos una pista o desactiva la música.')).toBeVisible();
    await expect(page.getByText('Añade el vídeo del sponsor o desactívalo.')).toBeVisible();
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
  });
  test('changed options require a validated saved plan but no separate brief approval', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    let document = editorial();
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() === 'POST') {
        const raw: unknown = route.request().postDataJSON();
        if (typeof raw !== 'object' || raw === null || !('options' in raw) || !isFullDemoOptions(raw.options)) throw new Error('Invalid options');
        document = { ...document, options: raw.options, plan_hash: 'b'.repeat(64) };
        await route.fulfill({ status: 201, json: document });
      } else await route.fulfill({ json: { document, defaults: document.options, compatibility: 'editorial-v1' } });
    });
    await gotoStudio(page, PRODUCE_FULL);
    const create = page.getByRole('button', { name: REC_CTA });
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
    await expect(create).toBeEnabled();
    await page.getByRole('spinbutton', { name: 'Volumen del juego', exact: true }).fill('0');
    await expect(create).toBeDisabled();
    await page.getByRole('button', { name: 'Actualizar y guardar plan' }).click();
    await expect(create).toBeEnabled();
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect(create).toBeEnabled();
    await page.reload();
    await expect(page.getByRole('spinbutton', { name: 'Volumen del juego', exact: true })).toHaveValue('0');
    await expect(create).toBeEnabled();
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
  });
  test('creating binds the validated document hash through generate and persists it for Library', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    let generated: unknown;
    await fulfillJson(page, '/renders/gameplay-pov-60', 404, {});
    await page.route(`**/api/demos/${JOB}/generate`, async (route) => {
      generated = route.request().postDataJSON();
      await fulfillJson(page, '/status', 200, { status: 'recording' });
      await route.fulfill({ status: 202, json: { accepted: true } });
    });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
    await page.getByRole('button', { name: REC_CTA }).click();
    await expect.poll(() => generated).toBeDefined();
    expect(generated).toMatchObject({ preset: 'gameplay-pov-60', segment_ids: [], edit: {
      full_demo: { document: editorial(), approval: { approved_plan_hash: editorial().plan_hash } },
      intro: false, outro: false, kill_counter: false, hook_text: false, cover_strategy: 'no-cover',
    } });
    const stored: unknown = await page.evaluate(() => JSON.parse(localStorage.getItem('cliphub.reels.v1') ?? '[]'));
    expect(stored).toMatchObject([{ editConfig: { fullDemo: { document: editorial() } } }]);
  });
  test('manual options remain independent across format switches', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('spinbutton', { name: 'Volumen del juego', exact: true }).fill('0');
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
    await page.getByRole('button', { name: 'Short 9:16', exact: true }).click();
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
    await page.getByRole('button', { name: 'Vídeo largo 16:9', exact: true }).click();
    await expect(page.getByRole('spinbutton', { name: 'Volumen del juego', exact: true })).toHaveValue('0');
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toHaveCount(0);
  });
});
for (const interruption of ['empty', 'failed']) {
  test(`the Short draft survives an ${interruption} plan poll while editing a long video`, async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    let phase = 'ready';
    let interruptedReads = 0;
    await page.route(`**/api/demos/${JOB}/plan`, (route) => {
      if (phase !== 'ready') interruptedReads += 1;
      if (phase === 'failed') return route.fulfill({ status: 503, json: { code: 'service_unavailable' } });
      return route.fulfill({ json: phase === 'empty' ? { ...PLAN, segments: [] } : PLAN });
    });
    await gotoStudio(page, `/clips/${JOB}/nuevo`);
    await page.getByRole('button', { name: 'Limpiar', exact: true }).click();
    await page.getByRole('button', { name: 'Sin música', exact: true }).click();
    await page.getByRole('button', { name: 'Vídeo largo 16:9', exact: true }).click();
    phase = interruption;
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect.poll(() => interruptedReads).toBeGreaterThan(0);
    await page.getByRole('button', { name: 'Short 9:16', exact: true }).click();
    if (interruption === 'empty') {
      await expect(page.getByRole('heading', { name: 'Sin jugadas destacables' })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Crear Short', exact: true })).toHaveCount(0);
    } else {
      await expect(page.getByRole('alert').filter({ hasText: 'Seguimos mostrando los últimos datos cargados' })).toBeVisible();
    }
    phase = 'ready';
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect(page.getByRole('heading', { name: PRODUCE_SHORT_TITLE })).toBeVisible();
    await expect(page.getByText('Solo el audio de la partida.', { exact: true })).toBeVisible();
    await expect(page.getByText('Elige al menos una jugada', { exact: true })).toBeVisible();
  });
}
