import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { currentFullDemoOptions, isFullDemoOptions, isFullDemoSnapshot, type FullDemoDocument } from '../lib/full-demo-plan.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const PRODUCE_FULL = `/clips/${JOB}/nuevo?formato=full`;
const DRAFT_KEY = `cliphub.full-demo.draft.v1:${JOB}`;
const PLAN = {
  demo: { map: 'de_inferno' }, target: { steamid64: '76561198000000001', name_in_demo: 'ropz', team_at_start: 'CT' },
  stats: { total_kills_target: 24 }, segments: [{ id: 'r1', round: 1, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }] }],
};
const ROSTER = { players: [{ steamid64: '76561198000000001', name: 'ropz', team: 'CT', kills: 24, deaths: 14, assists: 4 }] };

function editorial(legacy = false): FullDemoDocument {
  const raw: unknown = JSON.parse(readFileSync(new URL('../lib/full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  if (!isFullDemoSnapshot(raw)) throw new Error('Invalid Full Demo fixture');
  if (!legacy) raw.document.options = currentFullDemoOptions(raw.document.options);
  return raw.document;
}

async function seedStorage(page: Page, entries: Record<string, string>): Promise<void> {
  await page.addInitScript((seed: Record<string, string>) => {
    if (sessionStorage.getItem('e2e.seeded')) return;
    for (const [key, value] of Object.entries(seed)) localStorage.setItem(key, value);
    sessionStorage.setItem('e2e.seeded', '1');
  }, entries);
}

async function fulfillJson(page: Page, path: string, status: number, body: unknown): Promise<void> {
  await page.route(`**/api/demos/${JOB}${path}`, (route) => route.fulfill({ status, json: body }));
}

async function stubParsedMatch(page: Page, document: FullDemoDocument | null = editorial(), defaults = editorial().options): Promise<void> {
  await fulfillJson(page, '/status', 200, { status: 'parsed' });
  await fulfillJson(page, '/plan', 200, PLAN);
  await fulfillJson(page, '/roster', 200, ROSTER);
  await fulfillJson(page, '/recap-plan', 200, PLAN);
  await fulfillJson(page, '/full-demo/plan', 200, { document, defaults: document?.options ?? defaults, compatibility: document ? 'editorial-v1' : 'legacy-until-planned-and-approved' });
}

test.describe('Full POV simplified constructor', () => {
  for (const width of [390, 1024, 1440]) {
    test(`keeps useful choices usable at ${width}px with an unbroken player name`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await stubParsedMatch(page);
      const longName = `Donk${'W'.repeat(180)}`;
      await fulfillJson(page, '/plan', 200, { ...PLAN, target: { ...PLAN.target, name_in_demo: longName } });
      await fulfillJson(page, '/roster', 200, { players: [{ ...ROSTER.players[0], name: longName }] });
      await gotoStudio(page, PRODUCE_FULL);

      await expect(page.getByRole('combobox', { name: 'Diseño', exact: true })).toBeVisible();
      await page.getByRole('combobox', { name: 'Diseño', exact: true }).click();
      await page.getByRole('option', { name: 'Mono', exact: true }).click();
      await expect(page.getByRole('img', { name: 'Vista previa del HUD Mono', exact: true })).toBeVisible();
      await expect(page.getByRole('checkbox', { name: 'Activar efectos entre rondas', exact: true })).toBeChecked();
      await expect(page.getByRole('combobox', { name: 'Origen de la demo', exact: true })).toBeVisible();
      await expect(page.getByText('Jugadores y marcador en neón violeta.', { exact: true })).toBeVisible();
      await expect(page.getByRole('checkbox', { name: 'Utilizar un custom HUD', exact: true })).toHaveCount(0);
      await expect(page.getByText('Música de fondo', { exact: true })).toHaveCount(0);
      await expect(page.getByText('Ajustar mezcla del corte', { exact: true })).toHaveCount(0);
      await expect(page.getByText('Usar ajustes recomendados', { exact: true })).toHaveCount(0);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
      expect(overflow).toBe(false);
    });
  }

  test('normalizes an old local draft before direct planning and generation', async ({ page }) => {
    const legacy = editorial(true);
    legacy.options.overlays.roster = false;
    legacy.options.overlays.scoreboard = false;
    await seedStorage(page, { [DRAFT_KEY]: JSON.stringify(legacy.options) });
    await stubParsedMatch(page, editorial(true));
    let generated: unknown;
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      const body: unknown = route.request().postDataJSON();
      if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid options');
      expect(body.options).toMatchObject({ capture: { crosshair: { mode: 'observed', code: '', allow_capture_default: false } }, audio: { music: { enabled: false, assets: [] } }, overlays: { roster: true, scoreboard: true, theme: 'neon-violet', mode: 'generated' } });
      await route.fulfill({ status: 201, json: { ...editorial(), options: body.options, plan_hash: 'b'.repeat(64) } });
    });
    await page.route(`**/api/demos/${JOB}/generate`, async (route) => { generated = route.request().postDataJSON(); await route.fulfill({ status: 202, json: { accepted: true } }); });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { approval: { approved_plan_hash: 'b'.repeat(64) } } } });
  });

  test('sponsor starts optional and can be enabled before its asset is chosen', async ({ page }) => {
    const document = editorial();
    document.options.sponsor.enabled = false;
    document.options.sponsor.video = null;
    await stubParsedMatch(page, document);
    await gotoStudio(page, PRODUCE_FULL);
    const sponsor = page.getByRole('checkbox', { name: 'Incluir sponsor', exact: true });
    await expect(sponsor).not.toBeChecked();
    await sponsor.check();
    await expect(sponsor).toBeChecked();
    await expect(page.getByText('Añade el vídeo del sponsor o desactívalo.', { exact: true })).toBeVisible();
  });

  test('uploads an opted-in sponsor asset with its provenance', async ({ page }) => {
    const document = editorial();
    document.options.sponsor.enabled = false;
    document.options.sponsor.video = null;
    let uploaded = 0;
    await stubParsedMatch(page, document);
    await page.route('**/api/editor/assets', async (route) => {
      uploaded += 1;
      expect(route.request().method()).toBe('POST');
      await route.fulfill({ status: 201, json: { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) } });
    });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('checkbox', { name: 'Incluir sponsor', exact: true }).check();
    await page.getByText('Añadir vídeo del sponsor y permisos', { exact: true }).click();
    await page.getByLabel('Archivo local', { exact: true }).setInputFiles({ name: 'sponsor.mp4', mimeType: 'video/mp4', buffer: Buffer.from('sponsor') });
    await page.getByLabel('Título', { exact: true }).fill('Patrocinador');
    await page.getByLabel('Autor o titular', { exact: true }).fill('Titular');
    await page.getByLabel('Fuente (https://… o local:archivo-propio)', { exact: true }).fill('local:sponsor.mp4');
    await page.getByLabel('Licencia o permiso de uso', { exact: true }).fill('Autorizado');
    await page.getByRole('button', { name: 'Añadir archivo', exact: true }).click();
    await expect.poll(() => uploaded).toBe(1);
    await expect(page.getByText('Vídeo: Archivo pendiente de revisar en el plan', { exact: true })).toBeVisible();
  });

  test('prepares canonical sponsor boundaries from an empty plan, then creates the selected boundary', async ({ page }) => {
    const defaults = editorial().options;
    defaults.sponsor.enabled = false;
    defaults.sponsor.video = null;
    defaults.sponsor.placement_policy = 'first-two-rounds';
    defaults.sponsor.after_round_id = '';
    const prepared = editorial();
    prepared.options.sponsor.enabled = true;
    prepared.options.sponsor.video = { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) };
    let plans = 0;
    let generated: unknown;
    await stubParsedMatch(page, null, defaults);
    await page.route('**/api/editor/assets', async (route) => {
      expect(route.request().method()).toBe('POST');
      await route.fulfill({ status: 201, json: { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) } });
    });
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      plans += 1;
      const body: unknown = route.request().postDataJSON();
      if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid options');
      if (plans === 1) {
        expect(body.options.sponsor).toMatchObject({
          enabled: true,
          video: { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) },
          placement_policy: 'first-two-rounds',
        });
      } else {
        expect(body.options.sponsor).toMatchObject({
          enabled: true,
          video: { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) },
          placement_policy: 'round-boundary',
          after_round_id: 'round-002',
        });
      }
      await route.fulfill({ status: 201, json: { ...prepared, options: body.options, plan_hash: plans === 1 ? 'a'.repeat(64) : 'b'.repeat(64) } });
    });
    await page.route(`**/api/demos/${JOB}/generate`, async (route) => {
      generated = route.request().postDataJSON();
      await route.fulfill({ status: 202, json: { accepted: true } });
    });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('checkbox', { name: 'Incluir sponsor', exact: true }).check();
    await page.getByText('Añadir vídeo del sponsor y permisos', { exact: true }).click();
    await page.getByLabel('Archivo local', { exact: true }).setInputFiles({ name: 'sponsor.mp4', mimeType: 'video/mp4', buffer: Buffer.from('sponsor') });
    await page.getByLabel('Título', { exact: true }).fill('Patrocinador');
    await page.getByLabel('Autor o titular', { exact: true }).fill('Titular');
    await page.getByLabel('Fuente (https://… o local:archivo-propio)', { exact: true }).fill('local:sponsor.mp4');
    await page.getByLabel('Licencia o permiso de uso', { exact: true }).fill('Autorizado');
    await page.getByRole('button', { name: 'Añadir archivo', exact: true }).click();
    await page.getByRole('combobox', { name: 'Colocación', exact: true }).click();
    await page.getByRole('option', { name: 'Después de una ronda concreta', exact: true }).click();
    const boundary = page.getByRole('combobox', { name: 'Insertar después de', exact: true });
    await expect(boundary).toBeVisible();
    await expect.poll(() => plans).toBe(1);
    expect(generated).toBeUndefined();
    await boundary.click();
    await page.getByRole('option', { name: 'Ronda 2', exact: true }).click();
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { approval: { approved_plan_hash: 'b'.repeat(64) } } } });
    expect(plans).toBe(2);
  });

  test('storage failures keep the durable Full Demo creation flow available', async ({ page }) => {
    const document = editorial();
    let generated = 0;
    await page.addInitScript(() => Object.defineProperty(window, 'localStorage', {
      configurable: true,
      get() { throw new Error('Storage blocked'); },
    }));
    await stubParsedMatch(page, document);
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      const body: unknown = route.request().postDataJSON();
      if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid options');
      await route.fulfill({ status: 201, json: { ...document, options: body.options, plan_hash: 'e'.repeat(64) } });
    });
    await page.route(`**/api/demos/${JOB}/generate`, async (route) => { generated += 1; await route.fulfill({ status: 202, json: { accepted: true } }); });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('checkbox', { name: 'Incluir voces del equipo', exact: true }).uncheck();
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect.poll(() => generated).toBe(1);
  });

  test('an unavailable or incompatible plan keeps creation disabled until a valid retry', async ({ page }) => {
    let loads = 0;
    await stubParsedMatch(page);
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, (route) => {
      if (route.request().method() !== 'GET') return route.fallback();
      loads += 1;
      if (loads === 1) return route.fulfill({ status: 503, json: { error: 'Servicio no disponible' } });
      if (loads === 2) return route.fulfill({ status: 200, json: { document: {}, defaults: {}, compatibility: 'editorial-v1' } });
      return route.fallback();
    });
    await gotoStudio(page, PRODUCE_FULL);
    const retry = page.getByRole('button', { name: 'Reintentar conexión', exact: true });
    const create = page.getByRole('button', { name: 'Crear Full Demo', exact: true });
    await expect(retry).toBeVisible();
    await expect(create).toBeDisabled();
    await retry.click();
    await expect(retry).toBeVisible();
    await expect(create).toBeDisabled();
    await retry.click();
    await expect(page.getByRole('combobox', { name: 'Diseño', exact: true })).toBeVisible();
    await expect(create).toBeEnabled();
  });

  test('direct creation preserves planning blockers and never enqueues a stale plan', async ({ page }) => {
    const document = editorial();
    let generated = 0;
    await stubParsedMatch(page, document);
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      const body: unknown = route.request().postDataJSON();
      if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid options');
      await route.fulfill({ status: 201, json: { ...document, options: body.options, plan_hash: 'c'.repeat(64), blockers: [{ code: 'crosshair_missing', message: 'No se encontró el crosshair del jugador.' }] } });
    });
    await page.route(`**/api/demos/${JOB}/generate`, (route) => { generated += 1; return route.fulfill({ status: 202, json: { accepted: true } }); });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('checkbox', { name: 'Incluir voces del equipo', exact: true }).uncheck();
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect(page.getByRole('alert').filter({ hasText: 'No se encontró el crosshair del jugador.' })).toBeVisible();
    await expect(page.getByRole('alert').filter({ hasText: 'El plan tiene bloqueos o está incompleto.' })).toBeVisible();
    expect(generated).toBe(0);
  });

  test('a dirty choice prepares a new approval instead of reusing the older hash', async ({ page }) => {
    const document = editorial();
    document.plan_hash = 'a'.repeat(64);
    let planned = 0;
    let generated: unknown;
    await stubParsedMatch(page, document);
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      planned += 1;
      const body: unknown = route.request().postDataJSON();
      if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid options');
      await route.fulfill({ status: 201, json: { ...document, options: body.options, plan_hash: 'f'.repeat(64) } });
    });
    await page.route(`**/api/demos/${JOB}/generate`, async (route) => { generated = route.request().postDataJSON(); await route.fulfill({ status: 202, json: { accepted: true } }); });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('checkbox', { name: 'Incluir voces del equipo', exact: true }).uncheck();
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { approval: { approved_plan_hash: 'f'.repeat(64) } } } });
    expect(planned).toBe(1);
  });

  test('creates immediately from a current approved plan', async ({ page }) => {
    const document = editorial();
    let planned = 0;
    let generated: unknown;
    await stubParsedMatch(page, document);
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, (route) => { if (route.request().method() === 'POST') planned += 1; return route.fallback(); });
    await page.route(`**/api/demos/${JOB}/generate`, async (route) => { generated = route.request().postDataJSON(); await route.fulfill({ status: 202, json: { accepted: true } }); });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { approval: { approved_plan_hash: document.plan_hash } } } });
    expect(planned).toBe(0);
  });

  test('leaving Full Demo while planning cancels it and recovers creation on return', async ({ page }) => {
    const document = editorial();
    let held = false;
    let generated = 0;
    await stubParsedMatch(page, document);
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      held = true;
      await new Promise((resolve) => setTimeout(resolve, 500));
      const body: unknown = route.request().postDataJSON();
      if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid options');
      try { await route.fulfill({ status: 201, json: { ...document, options: body.options, plan_hash: 'd'.repeat(64) } }); } catch { /* Navigation aborts the held request. */ }
    });
    await page.route(`**/api/demos/${JOB}/generate`, (route) => { generated += 1; return route.fulfill({ status: 202, json: { accepted: true } }); });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('checkbox', { name: 'Incluir voces del equipo', exact: true }).uncheck();
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect.poll(() => held).toBe(true);
    await page.getByRole('button', { name: 'Short 9:16', exact: true }).click();
    await page.waitForTimeout(700);
    expect(generated).toBe(0);
    await page.getByRole('button', { name: 'Vídeo largo 16:9', exact: true }).click();
    const create = page.getByRole('button', { name: 'Crear Full Demo', exact: true });
    await expect(create).toBeEnabled();
    await expect(page.getByText('Preparando Full Demo…', { exact: true })).toHaveCount(0);
    await create.click();
    await expect.poll(() => generated).toBeGreaterThan(0);
  });
});
