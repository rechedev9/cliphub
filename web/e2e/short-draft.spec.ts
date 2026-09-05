import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { defaultShortSettings, shortDraftKey } from '../lib/produce/short-draft.ts';
import { PRODUCE_SHORT_DRAFT_RESET, PRODUCE_SHORT_DRAFT_RESTORED } from '../lib/produce/copy.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const HREF = `/clips/${JOB}/nuevo`;
const KEY = shortDraftKey(JOB);
const PLAN = {
  demo: { map: 'de_inferno' }, target: { steamid64: '76561198000000001', name_in_demo: 'ropz', team_at_start: 'CT' },
  stats: { total_kills_target: 24 },
  segments: [1, 2, 3].map((round) => ({ id: `r${round}`, round, tick_start: round * 100, tick_end: round * 100 + 90, kills: [{ weapon: 'ak47' }] })),
};

async function stubProducer(page: Page): Promise<void> {
  // Every request stays in this test, including background reconcile after creating.
  await page.route('**/api/**', (route) => route.fulfill({ status: 503, json: { code: 'service_unavailable' } }));
  await page.route('**/api/demos/jobs', (route) => route.fulfill({ json: { jobs: [] } }));
  await page.route('**/api/streams', (route) => route.fulfill({ json: { jobs: [] } }));
  await page.route(`**/api/demos/${JOB}/status`, (route) => route.fulfill({ json: { status: 'parsed' } }));
  await page.route(`**/api/demos/${JOB}/plan`, (route) => route.fulfill({ json: PLAN }));
  await page.route(`**/api/demos/${JOB}/recap-plan`, (route) => route.fulfill({ json: PLAN }));
  await page.route(`**/api/demos/${JOB}/roster`, (route) => route.fulfill({ json: {
    players: [{ steamid64: '76561198000000001', name: 'ropz', team: 'CT', kills: 24, deaths: 14, assists: 4 }],
  } }));
  await page.route('**/api/presets', (route) => route.fulfill({ json: { presets: [
    { name: 'viral-60-clean', label: 'Estilo limpio', default: true, width: 1080, height: 1920 },
    { name: 'viral-aggressive-60', label: 'Estilo intenso', width: 1080, height: 1920 },
  ] } }));
}

const selected = (page: Page) => page.locator('button.group\\/play[aria-pressed="true"]');
const rows = (page: Page) => page.locator('button.group\\/play');
const stored = (page: Page) => page.evaluate((key) => sessionStorage.getItem(key), KEY);

test.beforeEach(async ({ page }) => { await stubProducer(page); });

test('recovers the last selection immediately after editing and reloading', async ({ page }) => {
  await gotoStudio(page, HREF);
  await expect(selected(page)).toHaveCount(3);
  await rows(page).first().click();
  await page.reload();
  await expect(page.getByText(PRODUCE_SHORT_DRAFT_RESTORED, { exact: false })).toBeVisible();
  await expect(rows(page).first()).toHaveAttribute('aria-pressed', 'false');
  await expect(selected(page)).toHaveCount(2);
});

test('keeps an empty selection and no-music choice, then resets without losing the loaded preset', async ({ page }) => {
  await gotoStudio(page, HREF);
  await page.getByRole('button', { name: 'Limpiar', exact: true }).click();
  await page.getByRole('button', { name: 'Sin música', exact: true }).click();
  await page.reload();
  await expect(page.getByText(PRODUCE_SHORT_DRAFT_RESTORED, { exact: false })).toBeVisible();
  await expect(selected(page)).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Crear Short', exact: true })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Sin música', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: PRODUCE_SHORT_DRAFT_RESET }).click();
  await expect(selected(page)).toHaveCount(3);
  await expect(page.getByRole('button', { name: PRODUCE_SHORT_DRAFT_RESET })).toHaveCount(0);
  expect(await stored(page)).toBeNull();
  await expect(page.locator('#short-preset')).toContainText('Estilo limpio');
  await page.getByRole('button', { name: 'Sin música', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Crear Short', exact: true })).toBeEnabled();
});

test('keeps the chosen preset when leaving the screen and returning', async ({ page }) => {
  await gotoStudio(page, HREF);
  await page.locator('#short-preset').click();
  await page.getByRole('option', { name: 'Estilo intenso' }).click();
  await page.getByRole('link', { name: 'Volver', exact: true }).click();
  await expect(page).toHaveURL(/\/clips\?partida=/);
  await page.goBack();
  await expect(page.locator('#short-preset')).toContainText('Estilo intenso');
  await expect(page.getByText(PRODUCE_SHORT_DRAFT_RESTORED, { exact: false })).toBeVisible();
});

test('a removed preset falls back to a current preset before enabling creation', async ({ page }) => {
  await page.addInitScript(({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)), {
    key: KEY, value: { ...defaultShortSettings([]), version: 1, selectedIds: ['r1'], variant: 'removed', musicDecided: true },
  });
  await gotoStudio(page, HREF);
  await expect(page.locator('#short-preset')).toContainText('Estilo limpio');
  await expect(page.getByRole('button', { name: 'Crear Short', exact: true })).toBeEnabled();
});

test('resetting while presets load does not restore the discarded preset when they arrive', async ({ page }) => {
  await page.addInitScript(({ key, value }) => sessionStorage.setItem(key, JSON.stringify(value)), {
    key: KEY, value: { ...defaultShortSettings([]), version: 1, selectedIds: [], variant: 'viral-aggressive-60', musicDecided: true },
  });
  let release = () => {};
  const pending = new Promise<void>((resolve) => { release = resolve; });
  await page.route('**/api/presets', async (route) => {
    await pending;
    await route.fulfill({ json: { presets: [
      { name: 'viral-60-clean', label: 'Estilo limpio', default: true },
      { name: 'viral-aggressive-60', label: 'Estilo intenso' },
    ] } });
  });
  try {
    await gotoStudio(page, HREF);
    await page.getByRole('button', { name: PRODUCE_SHORT_DRAFT_RESET }).click();
    expect(await stored(page)).toBeNull();
  } finally {
    release();
  }
  await expect(page.locator('#short-preset')).toContainText('Estilo limpio');
  expect(await stored(page)).toBeNull();
  await page.getByRole('button', { name: 'Sin música', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Crear Short', exact: true })).toBeEnabled();
});

test('viewing the long format does not create a Short draft', async ({ page }) => {
  await gotoStudio(page, `${HREF}?formato=full`);
  await expect(page.locator('#short-preset')).toContainText('Estilo limpio');
  expect(await stored(page)).toBeNull();
  await page.getByRole('button', { name: 'Short 9:16', exact: true }).click();
  await expect(page.getByRole('button', { name: PRODUCE_SHORT_DRAFT_RESET })).toHaveCount(0);
});

test('successful creation clears the draft without a later write bringing it back', async ({ page }) => {
  await gotoStudio(page, HREF);
  await page.getByRole('button', { name: 'Sin música', exact: true }).click();
  expect(await stored(page)).not.toBeNull();
  await page.getByRole('button', { name: 'Crear Short', exact: true }).click();
  await expect(page).toHaveURL(/\/clips\?partida=/);
  expect(await stored(page)).toBeNull();
  await gotoStudio(page, HREF);
  await expect(page.locator('#short-preset')).toContainText('Estilo limpio');
  expect(await stored(page)).toBeNull();
  await expect(page.getByRole('button', { name: PRODUCE_SHORT_DRAFT_RESET })).toHaveCount(0);
});

test('a creation error preserves the draft', async ({ page }) => {
  await gotoStudio(page, HREF);
  await page.getByRole('button', { name: 'Sin música', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Crear Short', exact: true })).toBeEnabled();
  const before = await stored(page);
  await page.route(`**/api/demos/${JOB}/plan`, (route) => route.fulfill({ status: 503, json: { error: 'No se pudo leer el plan' } }));
  await page.getByRole('button', { name: 'Crear Short', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'No se pudo leer el plan' })).toBeVisible();
  expect(await stored(page)).toBe(before);
  await expect(page).toHaveURL(HREF);
});

test('blocked storage does not prevent editing or creating', async ({ page }) => {
  await page.addInitScript(() => Object.defineProperty(window, 'sessionStorage', { get() { throw new Error('Storage blocked'); } }));
  await gotoStudio(page, HREF);
  await rows(page).first().click();
  await expect(selected(page)).toHaveCount(2);
  await page.getByRole('button', { name: 'Sin música', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Crear Short', exact: true })).toBeEnabled();
});
