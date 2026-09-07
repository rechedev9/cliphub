import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { FULL_DEMO_EMPTY } from '../lib/full-demo.ts';
import { isFullDemoSnapshot, isFullDemoOptions, type FullDemoDocument, type FullDemoSnapshot } from '../lib/full-demo-plan.ts';
import { PRODUCE_MATCH_MISSING, PRODUCE_SHORT_TITLE } from '../lib/produce/copy.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const PRODUCE_FULL = `/clips/${JOB}/nuevo?formato=full`;
const REC_CTA = 'Crear Full Demo';
const SAVE_CTA = 'Actualizar y guardar plan';
const GAME_VOLUME = 'Volumen del juego';
const DRAFT_KEY = `cliphub.full-demo.draft.v1:${JOB}`;
const REELS_KEY = 'cliphub.reels.v1';
const INCOMPATIBLE_PLAN = 'El servidor devolvió un plan Full Demo incompatible.';
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
function snapshotWithHash(hash: string): FullDemoSnapshot {
  const document = { ...editorial(), plan_hash: hash };
  return { document, approval: { approved_plan_hash: hash, allow_safe_tail_trim: document.options.editorial.allow_safe_tail_trim, timestamp: '2026-01-01T00:00:00Z' } };
}
/** Seed localStorage once per tab, before the first page script, without re-seeding after client navigations. */
async function seedStorage(page: Page, entries: Record<string, string>): Promise<void> {
  await page.addInitScript((seed: Record<string, string>) => {
    if (sessionStorage.getItem('e2e.seeded')) return;
    for (const [key, value] of Object.entries(seed)) localStorage.setItem(key, value);
    sessionStorage.setItem('e2e.seeded', '1');
  }, entries);
}
/** The saved document identity lives in the collapsed Avanzado panel; it stays attached while hidden. */
type HeldRoute = Parameters<Parameters<Page['route']>[1]>[0];
/** Make localStorage (only) reads and writes for one key prefix throw; sessionStorage and other keys stay untouched. */
async function blockStorage(page: Page, prefix: string): Promise<void> {
  await page.addInitScript((blocked: string) => {
    const { getItem, setItem } = Storage.prototype;
    const hit = (store: Storage, key: string): boolean => store === window.localStorage && key.startsWith(blocked);
    Storage.prototype.getItem = function (key: string) { if (hit(this, key)) throw new DOMException('blocked', 'SecurityError'); return getItem.call(this, key); };
    Storage.prototype.setItem = function (key: string, value: string) { if (hit(this, key)) throw new DOMException('blocked', 'QuotaExceededError'); setItem.call(this, key, value); };
  }, prefix);
}
async function planHashLink(page: Page, hash: string): Promise<void> {
  await expect(page.getByRole('link', { name: `Ver documento del plan · ${hash.slice(0, 12)}`, includeHidden: true })).toBeAttached();
}
/** Non-empty alerts only: the App Router route announcer is an always-present empty alert. */
function shownErrors(page: Page) {
  return page.getByRole('alert').filter({ hasText: /\S/ });
}
async function storedIntents(page: Page): Promise<unknown> {
  return page.evaluate((key) => JSON.parse(localStorage.getItem(key) ?? 'null'), REELS_KEY);
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

test.describe('Full POV recovery boundaries', () => {
  for (const [name, draft, reels] of [
    ['unparsable', '{not json', '{not json'],
    ['invalid', JSON.stringify({ ...editorial().options, profile_id: 'legacy' }), JSON.stringify([{ videoId: 1, jobId: JOB }])],
  ] satisfies [string, string, string][]) {
    test(`${name} local storage falls back to the server plan and still persists a new intent`, async ({ page }) => {
      await stubParsedMatch(page, { status: 200, body: PLAN });
      await fulfillJson(page, '/renders/gameplay-pov-60', 404, {});
      let generated: unknown;
      await page.route(`**/api/demos/${JOB}/generate`, async (route) => {
        generated = route.request().postDataJSON();
        await fulfillJson(page, '/status', 200, { status: 'recording' });
        await route.fulfill({ status: 202, json: { accepted: true } });
      });
      await seedStorage(page, { [DRAFT_KEY]: draft, [REELS_KEY]: reels });
      await gotoStudio(page, PRODUCE_FULL);
      const document = editorial();
      await expect(page.getByRole('spinbutton', { name: GAME_VOLUME, exact: true })).toHaveValue(String(document.options.audio.game.gain));
      await planHashLink(page, document.plan_hash);
      await expect(shownErrors(page)).toHaveCount(0);
      await page.getByRole('button', { name: REC_CTA }).click();
      await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { approval: { approved_plan_hash: document.plan_hash } } } });
      expect(await storedIntents(page)).toMatchObject([{ videoId: `${JOB}__full-demo`, editConfig: { fullDemo: { approval: { approved_plan_hash: document.plan_hash } } } }]);
      expect(await storedIntents(page)).toHaveLength(1);
    });
  }
  test('a valid local draft overrides the server options but keeps the saved document', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    const draft = editorial().options; draft.audio.game.gain = 0;
    await seedStorage(page, { [DRAFT_KEY]: JSON.stringify(draft) });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('spinbutton', { name: GAME_VOLUME, exact: true })).toHaveValue('0');
    await planHashLink(page, editorial().plan_hash);
    await expect(page.getByRole('status').filter({ hasText: 'Guarda el plan para revisar' })).toBeVisible();
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
  });
  for (const failure of [
    { name: 'server error', status: 503, body: { code: 'service_unavailable', error: 'Planificador sin conexión' }, alert: 'Planificador sin conexión' },
    { name: 'malformed document', status: 201, body: {}, alert: INCOMPATIBLE_PLAN },
  ]) {
    test(`a ${failure.name} while saving keeps the unsaved options, blocks creation and allows a retry`, async ({ page }) => {
      await stubParsedMatch(page, { status: 200, body: PLAN });
      const original = editorial();
      const posted: unknown[] = [];
      await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
        if (route.request().method() !== 'POST') return route.fulfill({ json: { document: original, defaults: original.options, compatibility: 'editorial-v1' } });
        const raw: unknown = route.request().postDataJSON();
        posted.push(raw);
        if (posted.length === 1) return route.fulfill({ status: failure.status, json: failure.body });
        if (typeof raw !== 'object' || raw === null || !('options' in raw) || !isFullDemoOptions(raw.options)) throw new Error('Invalid options');
        return route.fulfill({ status: 201, json: { ...original, options: raw.options, plan_hash: 'b'.repeat(64) } });
      });
      await gotoStudio(page, PRODUCE_FULL);
      const create = page.getByRole('button', { name: REC_CTA });
      const volume = page.getByRole('spinbutton', { name: GAME_VOLUME, exact: true });
      await volume.fill('0');
      await page.getByRole('button', { name: SAVE_CTA }).click();
      await expect(page.getByRole('alert').filter({ hasText: failure.alert })).toBeVisible();
      await expect(volume).toHaveValue('0');
      await expect(volume).toBeEnabled();
      await expect(create).toBeDisabled();
      await planHashLink(page, original.plan_hash);
      expect(posted).toHaveLength(1);
      expect(await page.evaluate((key) => JSON.parse(localStorage.getItem(key) ?? 'null'), DRAFT_KEY)).toMatchObject({ audio: { game: { gain: 0 } } });
      await page.getByRole('button', { name: SAVE_CTA }).click();
      await expect(create).toBeEnabled();
      await expect(shownErrors(page)).toHaveCount(0);
      await planHashLink(page, 'b'.repeat(64));
      expect(posted).toHaveLength(2);
      expect(posted[1]).toMatchObject({ options: { audio: { game: { gain: 0 } } } });
    });
  }
  test('an in-flight save blocks every other submission until the server answers', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    const original = editorial();
    const held: HeldRoute[] = [];
    await page.route(`**/api/demos/${JOB}/full-demo/plan`, (route) => {
      if (route.request().method() !== 'POST') return route.fulfill({ json: { document: original, defaults: original.options, compatibility: 'editorial-v1' } });
      held.push(route);
    });
    await gotoStudio(page, PRODUCE_FULL);
    const create = page.getByRole('button', { name: REC_CTA });
    const volume = page.getByRole('spinbutton', { name: GAME_VOLUME, exact: true });
    await volume.fill('0');
    await page.getByRole('button', { name: SAVE_CTA }).click();
    const saving = page.getByRole('button', { name: 'Analizando voces y rondas…' });
    await expect(saving).toBeDisabled();
    await expect(saving).toHaveAttribute('aria-busy', 'true');
    await expect(create).toBeDisabled();
    await expect(volume).toBeDisabled();
    await expect.poll(() => held.length).toBe(1);
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect(saving).toBeDisabled();
    expect(held).toHaveLength(1);
    const raw: unknown = held[0]?.request().postDataJSON();
    if (typeof raw !== 'object' || raw === null || !('options' in raw) || !isFullDemoOptions(raw.options)) throw new Error('Invalid options');
    await held[0]?.fulfill({ status: 201, json: { ...original, options: raw.options, plan_hash: 'b'.repeat(64) } });
    await expect(create).toBeEnabled();
    await expect(volume).toBeEnabled();
    await planHashLink(page, 'b'.repeat(64));
    expect(held).toHaveLength(1);
  });
  test('a rejected creation keeps the validated plan, re-enables the retry and never navigates', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await fulfillJson(page, '/status', 200, { status: 'recording' });
    let captures = 0;
    await page.route(`**/api/demos/${JOB}/generate`, (route) => { captures += 1; return route.fulfill({ status: 202, json: { accepted: true } }); });
    const inFlight = snapshotWithHash('b'.repeat(64));
    await seedStorage(page, { [REELS_KEY]: JSON.stringify([{
      videoId: `${JOB}__full-demo`, jobId: JOB, segmentIds: [], mode: 'clean', variant: 'gameplay-pov-60', editConfig: { fullDemo: inFlight },
      title: '2 rondas - Full POV', map: 'de_inferno', score: '', createdAt: 1,
    }]) });
    await gotoStudio(page, PRODUCE_FULL);
    const cta = page.getByRole('button', { name: /^(Crear Full Demo|Poner Full Demo en cola)$/ });
    await expect(cta).toBeEnabled();
    await cta.click();
    await expect(page.getByRole('alert').filter({ hasText: 'Ya hay un Full Demo con otro plan en curso. Espera a que termine antes de cambiarlo.' })).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/clips/${JOB}/nuevo`));
    await expect(cta).toBeEnabled();
    await expect(page.getByText('Full Demo en cola', { exact: true })).toHaveCount(0);
    await planHashLink(page, editorial().plan_hash);
    expect(await storedIntents(page)).toMatchObject([{ editConfig: { fullDemo: { approval: { approved_plan_hash: 'b'.repeat(64) } } } }]);
    expect(await storedIntents(page)).toHaveLength(1);
    expect(captures).toBe(0);
  });
  // User-visible outcome only: a double click yields one capture request and one
  // stored intent. Which guard absorbed the second click (disabled button, intent
  // reuse, in-flight capture, or the recording status) is not established here.
  test('a double click on Crear Full Demo yields one capture request and one stored intent', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await fulfillJson(page, '/renders/gameplay-pov-60', 404, {});
    const captures: HeldRoute[] = [];
    let accepted = false;
    await page.route(`**/api/demos/${JOB}/status`, (route) => route.fulfill({ json: { status: accepted ? 'recording' : 'parsed' } }));
    await page.route(`**/api/demos/${JOB}/generate`, (route) => { captures.push(route); });
    await gotoStudio(page, PRODUCE_FULL);
    await page.getByRole('button', { name: REC_CTA }).dblclick();
    await expect.poll(() => captures.length).toBe(1);
    // Creation is acknowledged before the capture is accepted: the shell jobs control already shows the one queued intent.
    await expect(page).not.toHaveURL(new RegExp(`/clips/${JOB}/nuevo`));
    const jobs = page.getByRole('button', { name: /^Trabajos: / });
    await expect(jobs).toHaveAccessibleName(/^Trabajos: EN COLA, /);
    expect(await storedIntents(page)).toMatchObject([{ editConfig: { fullDemo: { approval: { approved_plan_hash: editorial().plan_hash } } } }]);
    // Release the held capture; the jobs control moving to REC shows the accepted response was processed.
    accepted = true;
    await captures[0]?.fulfill({ status: 202, json: { accepted: true } });
    await expect(jobs).toHaveAccessibleName(/^Trabajos: REC/);
    expect(captures).toHaveLength(1);
    expect(captures[0]?.request().postDataJSON()).toMatchObject({ edit: { full_demo: { approval: { approved_plan_hash: editorial().plan_hash } } } });
    expect(await storedIntents(page)).toHaveLength(1);
  });
  for (const [name, prefix, persisted] of [
    ['draft', DRAFT_KEY, true],
    ['intent', REELS_KEY, false],
  ] satisfies [string, string, boolean][]) {
    test(`unavailable ${name} storage keeps the server plan editable, saveable and recordable`, async ({ page }) => {
      await stubParsedMatch(page, { status: 200, body: PLAN });
      await fulfillJson(page, '/renders/gameplay-pov-60', 404, {});
      const original = editorial();
      await page.route(`**/api/demos/${JOB}/full-demo/plan`, async (route) => {
        if (route.request().method() !== 'POST') return route.fulfill({ json: { document: original, defaults: original.options, compatibility: 'editorial-v1' } });
        const raw: unknown = route.request().postDataJSON();
        if (typeof raw !== 'object' || raw === null || !('options' in raw) || !isFullDemoOptions(raw.options)) throw new Error('Invalid options');
        return route.fulfill({ status: 201, json: { ...original, options: raw.options, plan_hash: 'b'.repeat(64) } });
      });
      let generated: unknown;
      await page.route(`**/api/demos/${JOB}/generate`, async (route) => {
        generated = route.request().postDataJSON();
        await fulfillJson(page, '/status', 200, { status: 'recording' });
        await route.fulfill({ status: 202, json: { accepted: true } });
      });
      await blockStorage(page, prefix);
      await gotoStudio(page, PRODUCE_FULL);
      const volume = page.getByRole('spinbutton', { name: GAME_VOLUME, exact: true });
      const create = page.getByRole('button', { name: REC_CTA });
      await expect(volume).toHaveValue(String(original.options.audio.game.gain));
      await expect(create).toBeEnabled();
      await volume.fill('0');
      await expect(volume).toHaveValue('0');
      await expect(create).toBeDisabled();
      await page.getByRole('button', { name: SAVE_CTA }).click();
      await expect(create).toBeEnabled();
      await expect(shownErrors(page)).toHaveCount(0);
      await planHashLink(page, 'b'.repeat(64));
      await create.click();
      await expect.poll(() => generated).toMatchObject({ edit: { full_demo: { approval: { approved_plan_hash: 'b'.repeat(64) } } } });
      const keys: string[] = await page.evaluate(() => Object.keys(localStorage));
      expect(keys).not.toContain(prefix);
      if (persisted) expect(await storedIntents(page)).toMatchObject([{ editConfig: { fullDemo: { approval: { approved_plan_hash: 'b'.repeat(64) } } } }]);
      else expect(keys).not.toContain(REELS_KEY);
    });
  }
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
