import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { FULL_DEMO_EMPTY } from '../lib/full-demo.ts';
import { isFullDemoSnapshot, isFullDemoOptions, type FullDemoDocument } from '../lib/full-demo-plan.ts';
import { PRODUCE_MATCH_MISSING, PRODUCE_SHORT_TITLE } from '../lib/produce/copy.ts';
import { CUSTOM_HUD_CAPTURE_PROFILE, CUSTOM_HUD_THEMES } from '../lib/custom-hud.ts';

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
  for (const width of [390, 1024, 1440]) {
    test(`custom HUD selection, saved approval and adjacent controls at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 950 });
      await stubParsedMatch(page, { status: 200, body: PLAN });
      const longName = 'Donk' + 'W'.repeat(180);
      await fulfillJson(page, '/plan', 200, { ...PLAN, target: { ...PLAN.target, name_in_demo: longName } });
      await fulfillJson(page, '/roster', 200, { players: [{ ...ROSTER.players[0], name: longName }] });
      let document = editorial();
      let generated: unknown;
      await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
        if (route.request().method() === 'POST') {
          const body: unknown = route.request().postDataJSON();
          if (typeof body !== 'object' || body === null || !('options' in body) || !isFullDemoOptions(body.options)) throw new Error('Invalid HUD decisions');
          document = { ...document, options: body.options, plan_hash: 'b'.repeat(64) };
          await route.fulfill({ status: 201, json: document });
        } else await route.fulfill({ json: { document, defaults: document.options, compatibility: 'editorial-v1' } });
      });
      await fulfillJson(page, '/renders/gameplay-pov-60', 404, {});
      await page.route(`**/api/demos/${JOB}/generate`, async (route) => {
        generated = route.request().postDataJSON();
        await fulfillJson(page, '/status', 200, { status: 'recording' });
        await route.fulfill({ status: 202, json: { accepted: true } });
      });
      await gotoStudio(page, PRODUCE_FULL);
      await page.getByRole('checkbox', { name: 'Utilizar un custom HUD', exact: true }).check();
      await expect(page.getByRole('radio', { name: /^HUD / })).toHaveCount(10);
      for (const theme of CUSTOM_HUD_THEMES) {
        const radio = page.getByRole('radio', { name: `HUD ${theme.name}`, exact: true });
        await page.locator('label').filter({ has: radio }).click();
        await expect(radio).toBeChecked();
        await expect(page.getByRole('img', { name: `Vista previa del HUD ${theme.name}`, exact: true })).toHaveJSProperty('naturalWidth', 1920);
        const overflow = await page.evaluate(() => window.document.documentElement.scrollWidth > window.innerWidth);
        expect(overflow, `page overflow in ${theme.id}`).toBe(false);
      }
      await page.getByRole('button', { name: 'Ampliar HUD Mono', exact: true }).click();
      const expanded = page.getByRole('dialog');
      await expect(expanded).toBeVisible();
      await expanded.getByRole('button', { name: 'Jugador', exact: true }).click();
      await expect(expanded.getByTestId('custom-hud-expanded').locator('svg').last()).toHaveAttribute('viewBox', '16 926 324 134');
      await expanded.getByRole('button', { name: 'Arma', exact: true }).click();
      await expect(expanded.getByTestId('custom-hud-expanded').locator('svg').last()).toHaveAttribute('viewBox', '1540 938 364 122');
      await expanded.getByRole('button', { name: 'Marcador', exact: true }).click();
      await expect(expanded.getByTestId('custom-hud-expanded').locator('svg').last()).toHaveAttribute('viewBox', '436 12 1048 100');
      await expanded.getByRole('button', { name: 'Fondo claro', exact: true }).click();
      await expect(expanded.getByRole('button', { name: 'Fondo claro', exact: true })).toHaveAttribute('aria-pressed', 'true');
      const bounds = await expanded.boundingBox();
      expect(bounds && bounds.x >= 0 && bounds.x + bounds.width <= width).toBeTruthy();
      await expanded.getByRole('button', { name: 'Cerrar', exact: true }).click();
      await expect(page.getByRole('button', { name: 'Ampliar HUD Mono', exact: true })).toBeFocused();
      await page.getByRole('spinbutton', { name: 'Volumen del juego', exact: true }).fill('0.8');
      await page.getByRole('button', { name: 'Actualizar y guardar plan' }).click();
      await expect(page.getByRole('button', { name: REC_CTA })).toBeEnabled();
      await page.reload();
      await expect(page.getByRole('radio', { name: 'HUD Mono', exact: true })).toBeChecked();
      await expect(page.getByRole('spinbutton', { name: 'Volumen del juego', exact: true })).toHaveValue('0.8');
      await page.getByRole('button', { name: REC_CTA }).click();
      await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { document: { options: { capture: { hud_profile: CUSTOM_HUD_CAPTURE_PROFILE }, overlays: { hud_theme: 'mono' } } }, approval: { approved_plan_hash: document.plan_hash } } } });
    });
  }

  test('recommended tuning restores defaults while preserving chosen media and voices', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, PRODUCE_FULL);
    const volume = page.getByRole('spinbutton', { name: 'Volumen del juego', exact: true });
    const defaultVolume = await volume.inputValue();
    await volume.fill('0');
    const draftKey = `cliphub.full-demo.draft.v1:${JOB}`;
    const before = await page.evaluate((key) => JSON.parse(localStorage.getItem(key) ?? 'null'), draftKey);
    await page.getByRole('button', { name: 'Usar ajustes recomendados', exact: true }).click();
    await expect(volume).toHaveValue(defaultVolume);
    const after = await page.evaluate((key) => JSON.parse(localStorage.getItem(key) ?? 'null'), draftKey);
    expect(after.audio.music.assets).toEqual(before.audio.music.assets);
    expect(after.sponsor).toEqual(before.sponsor);
    expect(after.audio.voice.enabled).toBe(before.audio.voice.enabled);
    await expect(page.getByRole('spinbutton', { name: 'R1: tick final' })).toBeHidden();
  });
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
    await expect(page.getByRole('spinbutton', { name: 'Freeze antes de jugar (s)', exact: true })).toBeHidden();
    await page.getByText('Avanzado', { exact: true }).click();
    await expect(page.getByText('R01', { exact: true }).filter({ visible: true })).toBeVisible();
    await expect(page.getByText('R02', { exact: true })).toBeVisible();
    await expect(page.getByText('0 kills', { exact: true })).toHaveCount(2);
    await expect(page.getByText(/Freeze fijo: los últimos 2 segundos/)).toBeVisible();
    await expect(page.getByRole('spinbutton', { name: /Freeze|Contexto de las voces/ })).toHaveCount(0);
    await expect(page.getByRole('switch', { name: 'Conservar voces durante el freeze' })).toHaveCount(0);
    await page.locator('summary').filter({ hasText: 'R01' }).click();
    await expect(page.getByRole('spinbutton', { name: 'R1: tick inicial' })).toHaveCount(0);
    await expect(page.getByText(/Inicio fijo: tick 128/)).toBeVisible();
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

test('round effects persist through saving, reload and the approved generation request', async ({ page }) => {
  await stubParsedMatch(page, { status: 200, body: PLAN });
  let document = editorial();
  let generated: unknown;
  await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
    if (route.request().method() === 'POST') {
      const raw: unknown = route.request().postDataJSON();
      if (typeof raw !== 'object' || raw === null || !('options' in raw) || !isFullDemoOptions(raw.options)) throw new Error('Invalid transition options');
      document = { ...document, options: raw.options, plan_hash: 'c'.repeat(64) };
      return route.fulfill({ status: 201, json: document });
    }
    return route.fulfill({ json: { document, defaults: document.options, compatibility: 'editorial-v1' } });
  });
  await page.route(`**/api/demos/${JOB}/generate`, (route) => {
    generated = route.request().postDataJSON();
    return route.fulfill({ status: 202, json: { accepted: true } });
  });
  await gotoStudio(page, PRODUCE_FULL);
  await expect(page.getByRole('checkbox', { name: 'Activar efectos entre rondas' })).not.toBeChecked();
  await page.getByRole('checkbox', { name: 'Activar efectos entre rondas' }).check();
  await page.getByRole('button', { name: 'Dinámico', exact: true }).click();
  await page.getByRole('spinbutton', { name: 'Duración de la transición (fotogramas)' }).fill('10');
  await page.getByRole('combobox', { name: 'Dirección del barrido' }).click();
  await page.getByRole('option', { name: 'Abajo', exact: true }).click();
  await page.getByRole('spinbutton', { name: 'Aumento del zoom (%)' }).fill('15');
  await page.getByRole('spinbutton', { name: 'Duración del microflash (fotogramas)' }).fill('4');
  await page.getByRole('spinbutton', { name: 'Separación de color (px)' }).fill('6');
  await page.getByRole('spinbutton', { name: 'Volumen del whoosh (dB)' }).fill('-24');
  await page.getByRole('spinbutton', { name: 'Cola de voces hacia la siguiente ronda (s)' }).fill('0.9');
  await page.getByText('Ajustar mezcla del corte', { exact: true }).click();
  await page.getByRole('spinbutton', { name: 'Filtro grave de salida (Hz; 0 desactiva)' }).fill('2400');
  await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
  await page.getByRole('button', { name: /guardar plan/i }).click();
  await expect(page.getByRole('button', { name: REC_CTA })).toBeEnabled();
  expect(document.options.transitions).toMatchObject({ enabled: true, direction: 'down', duration_frames: 10, zoom_percent: 15, flash_frames: 4, rgb_pixels: 6, whoosh_gain_db: -24, comms_tail_seconds: .9, game_tail_lowpass_hz: 2400 });
  await page.reload();
  await expect(page.getByRole('checkbox', { name: 'Activar efectos entre rondas' })).toBeChecked();
  await expect(page.getByRole('spinbutton', { name: 'Duración de la transición (fotogramas)' })).toHaveValue('10');
  await expect(page.getByRole('combobox', { name: 'Dirección del barrido' })).toHaveText('Abajo');
  await page.getByRole('button', { name: REC_CTA }).click();
  await expect.poll(() => generated).toBeDefined();
  expect(generated).toMatchObject({ edit: { full_demo: { document: { options: { transitions: document.options.transitions } } } } });
});

for (const width of [390, 1024, 1440]) {
  test(`round effects remain usable at ${width}px with an unbroken player name`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await fulfillJson(page, '/plan', 200, { ...PLAN, target: { ...PLAN.target, name_in_demo: 'donk'.repeat(65) } });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('checkbox', { name: 'Activar efectos entre rondas' }).check();
    await page.getByRole('button', { name: 'Dinámico', exact: true }).click();
    await page.getByText('Ajustar mezcla del corte', { exact: true }).click();
    const rgb = page.getByRole('spinbutton', { name: 'Separación de color (px)' });
    await rgb.fill('4');
    await expect(rgb).toHaveValue('4');
    await page.getByRole('checkbox', { name: 'Microflash de brillo' }).uncheck();
    await expect(page.getByRole('checkbox', { name: 'Separación RGB' })).toBeChecked();
    await expect(page.getByRole('button', { name: /guardar plan/i })).toBeEnabled();
    const overflow = await page.evaluate(() => ({ width: document.documentElement.clientWidth, scroll: document.documentElement.scrollWidth }));
    expect(overflow.scroll).toBeLessThanOrEqual(overflow.width);
    await page.getByRole('checkbox', { name: 'Activar efectos entre rondas' }).uncheck();
    await expect(page.getByRole('spinbutton', { name: 'Separación de color (px)' })).toHaveCount(0);
  });
}
