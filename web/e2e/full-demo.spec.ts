import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { currentFullDemoOptions, isFullDemoOptions, isFullDemoSnapshot, type FullDemoDocument, type FullDemoOptions } from '../lib/full-demo-plan.ts';

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
    test(`Focus portrait uploads, persists and reaches generation at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      await stubParsedMatch(page);
      const portrait = { id: '22222222-2222-4222-8222-222222222222', sha256: 'a'.repeat(64) };
      const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=', 'base64');
      await page.route('**/api/full-demo/overlay-images', async (route) => {
        expect(route.request().headers()['content-type']).toContain('multipart/form-data');
        await route.fulfill({ status: 201, json: portrait });
      });
      await page.route(`**/api/full-demo/overlay-images/${portrait.id}`, (route) => route.fulfill({ contentType: 'image/png', body: png }));
      let generated: unknown;
      await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
        if (route.request().method() !== 'POST') return route.fallback();
        const body = route.request().postDataJSON();
        expect(body.options.overlays).toMatchObject({ hud_theme: 'focus', hud_portrait: portrait });
        await route.fulfill({ status: 201, json: { ...editorial(), options: body.options, plan_hash: 'b'.repeat(64) } });
      });
      await page.route(`**/api/demos/${JOB}/generate`, async (route) => { generated = route.request().postDataJSON(); await route.fulfill({ status: 202, json: { accepted: true } }); });
      await gotoStudio(page, PRODUCE_FULL);
      await page.getByRole('combobox', { name: 'Diseño', exact: true }).click();
      await page.getByRole('option', { name: 'Focus', exact: true }).click();
      await page.getByLabel('Retrato del jugador (opcional)', { exact: true }).setInputFiles({ name: 'portrait.png', mimeType: 'image/png', buffer: png });
      await expect(page.getByRole('button', { name: 'Quitar retrato', exact: true })).toBeEnabled();
      await expect(page.getByRole('img', { name: 'Retrato del jugador', exact: true })).toBeVisible();
      await expect.poll(() => page.evaluate((key) => JSON.parse(localStorage.getItem(key) ?? '{}').overlays?.hud_portrait, DRAFT_KEY)).toEqual(portrait);
      await page.reload();
      await expect(page.getByRole('button', { name: 'Quitar retrato', exact: true })).toBeVisible();
      await page.getByRole('button', { name: 'Ampliar HUD Focus', exact: true }).click();
      await expect(page.getByRole('dialog').getByRole('img', { name: 'Retrato del jugador', exact: true })).toBeVisible();
      await page.screenshot({ path: testInfo.outputPath('focus-preview.png'), animations: 'disabled' });
      await page.keyboard.press('Escape');
      expect(await page.evaluate(() => window.document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await page.getByRole('button', { name: 'Quitar retrato', exact: true }).click();
      await expect(page.getByRole('img', { name: 'Retrato del jugador', exact: true })).toHaveCount(0);
      await page.getByLabel('Retrato del jugador (opcional)', { exact: true }).setInputFiles({ name: 'portrait.png', mimeType: 'image/png', buffer: png });
      await expect(page.getByRole('button', { name: 'Quitar retrato', exact: true })).toBeEnabled();
      await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
      await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { document: { options: { overlays: { hud_theme: 'focus', hud_portrait: portrait } } } } } });
    });
  }

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

  for (const width of [390, 1440]) {
    test(`bumper MP4 states, hover, persistence and generation at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 1000 });
      const document = editorial();
      delete document.options.bumpers;
      document.options.sponsor.enabled = false;
      document.options.sponsor.video = null;
      await stubParsedMatch(page, document);
      const intro = { id: 'dddddddd-dddd-4ddd-8ddd-dddddddddddd', sha256: 'd'.repeat(64) };
      const outro = { id: 'eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee', sha256: 'e'.repeat(64) };
      const clip = readFileSync(new URL('./fixtures/stream-source.mp4', import.meta.url));
      await page.route('**/api/editor/assets/*/media', (route) => route.fulfill({ contentType: 'video/mp4', body: clip }));
      let finishUpload: (() => void) | undefined;
      let fail = false;
      let uploaded = 0;
      await page.route('**/api/editor/assets', async (route) => {
        uploaded++;
        expect(route.request().headers()['content-type']).toContain('multipart/form-data');
        const body = route.request().postDataBuffer()?.toString() ?? '';
        expect(body).toContain('No declarado');
        if (uploaded === 1) await new Promise<void>((resolve) => { finishUpload = resolve; });
        await route.fulfill(fail ? { status: 422, json: { error: 'El archivo no contiene vídeo válido.' } }
          : { status: 201, json: body.includes('outro.mp4') ? outro : intro });
      });
      await page.route(`**/api/editor/assets/${intro.id}`, (route) => route.fulfill({ json: { file_name: 'intro.mp4' } }));
      await page.route(`**/api/editor/assets/${outro.id}`, (route) => route.fulfill({ json: { file_name: 'outro.mp4' } }));
      let generated: unknown;
      await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
        if (route.request().method() !== 'POST') return route.fallback();
        const body = route.request().postDataJSON();
        expect(body.options.bumpers).toEqual({ intro: { enabled: true, video: intro }, outro: { enabled: true, video: outro } });
        expect(body.options.transitions.enabled).toBe(false);
        await route.fulfill({ status: 201, json: { ...document, options: body.options, plan_hash: 'b'.repeat(64) } });
      });
      await page.route(`**/api/demos/${JOB}/generate`, async (route) => {
        generated = route.request().postDataJSON();
        await route.fulfill({ status: 202, json: { accepted: true } });
      });
      await gotoStudio(page, PRODUCE_FULL);
      const card = page.locator('[data-bumper="intro"]').locator('..');
      const button = page.getByRole('button', { name: 'Subir MP4 de intro', exact: true });
      const input = page.getByLabel('Archivo MP4 de intro', { exact: true });
      const mp4 = { name: 'intro.mp4', mimeType: 'video/mp4', buffer: clip };
      await expect(button).toBeVisible();
      await expect(page.getByRole('checkbox', { name: 'Incluir intro', exact: true })).toHaveCount(0);
      await card.screenshot({ path: testInfo.outputPath('01-empty.png') });
      await button.hover();
      const glow = button.locator('span[aria-hidden]');
      await expect(glow).toHaveCSS('opacity', '1');
      expect(await button.evaluate((el) => el.style.getPropertyValue('--pointer-x'))).not.toBe('');
      await card.screenshot({ path: testInfo.outputPath('02-hover.png') });
      // Cancelling the native picker leaves both slots empty.
      const chooser = page.waitForEvent('filechooser');
      await button.click();
      await (await chooser).setFiles([]);
      await expect(button).toBeEnabled();
      expect(uploaded).toBe(0);
      await input.setInputFiles(mp4);
      await expect.poll(() => Boolean(finishUpload)).toBe(true);
      await expect(page.getByRole('button', { name: 'Subiendo intro…', exact: true })).toBeDisabled();
      await expect(page.getByRole('button', { name: 'Crear Full Demo', exact: true })).toBeDisabled();
      await card.screenshot({ path: testInfo.outputPath('03-uploading.png') });
      finishUpload!();
      await expect(page.getByRole('button', { name: 'Cambiar MP4 de intro', exact: true })).toBeEnabled();
      await expect(page.getByText('intro.mp4', { exact: true })).toBeVisible();
      await expect.poll(() => page.getByLabel('Previsualizar intro', { exact: true }).evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
      await card.screenshot({ path: testInfo.outputPath('04-intro.png') });
      fail = true;
      await input.setInputFiles(mp4);
      await expect(card.getByRole('alert')).toBeVisible();
      await expect(page.getByText('intro.mp4', { exact: true })).toBeVisible();
      await card.screenshot({ path: testInfo.outputPath('05-error-keeps-clip.png') });
      fail = false;
      await input.setInputFiles(mp4);
      await expect(card.getByRole('alert')).toHaveCount(0);
      await page.getByLabel('Archivo MP4 de outro', { exact: true }).setInputFiles({ ...mp4, name: 'outro.mp4' });
      await expect(page.getByRole('button', { name: 'Cambiar MP4 de outro', exact: true })).toBeEnabled();
      await page.getByRole('checkbox', { name: 'Activar efectos entre rondas', exact: true }).uncheck();
      await expect.poll(() => page.evaluate((key) => JSON.parse(localStorage.getItem(key) ?? '{}').bumpers?.outro.video, DRAFT_KEY)).toEqual(outro);
      await page.reload();
      await expect(page.getByText('intro.mp4', { exact: true })).toBeVisible();
      await expect(page.getByText('outro.mp4', { exact: true })).toBeVisible();
      await card.screenshot({ path: testInfo.outputPath('06-both-restored.png') });
      expect(await page.evaluate(() => window.document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
      await page.getByRole('button', { name: 'Quitar intro', exact: true }).click();
      await expect(button).toBeVisible();
      await expect(page.getByText('outro.mp4', { exact: true })).toBeVisible();
      await card.screenshot({ path: testInfo.outputPath('07-intro-removed.png') });
      await input.setInputFiles(mp4);
      await expect(page.getByRole('button', { name: 'Cambiar MP4 de intro', exact: true })).toBeEnabled();
      await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
      await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { document: { options: {
        bumpers: { intro: { enabled: true, video: intro }, outro: { enabled: true, video: outro } }, transitions: { enabled: false },
      } } } } });
    });
  }

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

  test('reopening round-boundary keeps the certified round already chosen', async ({ page }) => {
    const document = editorial();
    document.options.sponsor.enabled = true;
    document.options.sponsor.placement_policy = 'first-two-rounds';
    document.options.sponsor.after_round_id = 'round-002';
    await stubParsedMatch(page, document);
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('combobox', { name: 'Colocación', exact: true }).click();
    await page.getByRole('option', { name: 'Después de una ronda concreta', exact: true }).click();
    await expect(page.getByRole('combobox', { name: 'Insertar después de', exact: true })).toHaveText('Ronda 2');
  });

  test('leaving Full Demo while preparing sponsor rounds does not claim missing rounds', async ({ page }) => {
    const defaults = editorial().options;
    defaults.sponsor.enabled = false;
    defaults.sponsor.video = null;
    defaults.sponsor.placement_policy = 'first-two-rounds';
    defaults.sponsor.after_round_id = '';
    let held = false;
    await stubParsedMatch(page, null, defaults);
    await page.route('**/api/editor/assets', async (route) => {
      await route.fulfill({ status: 201, json: { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) } });
    });
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      held = true;
      await new Promise((resolve) => setTimeout(resolve, 500));
      const body: unknown = route.request().postDataJSON();
      if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid options');
      try { await route.fulfill({ status: 201, json: { ...editorial(), options: body.options, plan_hash: 'e'.repeat(64) } }); } catch { /* Navigation aborts the held request. */ }
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
    await expect.poll(() => held).toBe(true);
    await page.getByRole('button', { name: 'Short 9:16', exact: true }).click();
    await page.waitForTimeout(700);
    await page.getByRole('button', { name: 'Vídeo largo 16:9', exact: true }).click();
    await expect(page.getByRole('alert').filter({ hasText: 'No hay una ronda certificada disponible para el sponsor.' })).toHaveCount(0);
  });

  test('game and voice volume sliders travel with the plan options', async ({ page }) => {
    const document = editorial();
    const planned: FullDemoOptions[] = [];
    await stubParsedMatch(page, document);
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
      if (route.request().method() !== 'POST') return route.fallback();
      const body: unknown = route.request().postDataJSON();
      if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid options');
      planned.push(body.options);
      await route.fulfill({ status: 201, json: { ...document, options: body.options, plan_hash: 'e'.repeat(64) } });
    });
    await page.route(`**/api/demos/${JOB}/generate`, async (route) => { await route.fulfill({ status: 202, json: { accepted: true } }); });
    await gotoStudio(page, PRODUCE_FULL);
    // The stored plan hydrates options asynchronously; edit only once it has landed.
    await expect(page.getByRole('status').filter({ hasText: 'Voces: disponibles.' })).toBeVisible();
    const gameSlider = page.getByRole('slider', { name: /^Juego/ });
    const voiceSlider = page.getByRole('slider', { name: /^Voces/ });
    await expect(gameSlider).toHaveValue('100');
    await expect(voiceSlider).toHaveValue('85');
    await gameSlider.fill('60');
    await voiceSlider.fill('120');
    await expect(page.getByText('Juego · 60%')).toBeVisible();
    await expect(page.getByText('Voces · 120%')).toBeVisible();
    await page.getByRole('checkbox', { name: 'Incluir voces del equipo', exact: true }).uncheck();
    await expect(voiceSlider).toBeDisabled();
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect.poll(() => planned.length).toBe(1);
    expect(planned[0]?.audio.game.gain).toBeCloseTo(0.6);
    expect(planned[0]?.audio.voice.gain).toBeCloseTo(1.2);
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

  test('leaving Full Demo after create starts does not enqueue or toast', async ({ page }) => {
    const document = editorial();
    let statusHeld = false;
    let generated = 0;
    await stubParsedMatch(page, document);
    await page.route(`**/api/demos/${JOB}/generate`, (route) => { generated += 1; return route.fulfill({ status: 202, json: { accepted: true } }); });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('button', { name: 'Crear Full Demo', exact: true })).toBeEnabled();
    await page.route(`**/api/demos/${JOB}/status`, async (route) => {
      statusHeld = true;
      await new Promise((resolve) => setTimeout(resolve, 500));
      return route.fallback();
    });
    await page.getByRole('button', { name: 'Crear Full Demo', exact: true }).click();
    await expect.poll(() => statusHeld).toBe(true);
    await page.getByRole('button', { name: 'Short 9:16', exact: true }).click();
    await page.waitForTimeout(700);
    expect(generated).toBe(0);
    await expect(page.getByText('Full Demo en cola', { exact: true })).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Prepara tu Short', exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Vídeo largo 16:9', exact: true }).click();
    const create = page.getByRole('button', { name: 'Crear Full Demo', exact: true });
    await expect(create).toBeEnabled();
    await expect(page.getByText('Preparando Full Demo…', { exact: true })).toHaveCount(0);
  });
});
