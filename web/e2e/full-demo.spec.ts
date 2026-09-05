import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import {
  FULL_DEMO_CONTRACT,
  FULL_DEMO_EMPTY,
  FULL_DEMO_FORGE_HINT_EMPTY,
  FULL_DEMO_FORGE_HINT_ERROR,
  FULL_DEMO_RECAP_ERROR,
  FULL_DEMO_ROUNDS_PENDING,
} from '../lib/full-demo.ts';
import { NATIVE_HUD_LABEL } from '../lib/preset-copy.ts';
import {
  PRODUCE_FULL_ROUNDS_NOTE,
  PRODUCE_FULL_TITLE,
  PRODUCE_MATCH_MISSING,
  PRODUCE_SHORT_TITLE,
} from '../lib/produce/copy.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const PRODUCE_FULL = `/clips/${JOB}/nuevo?formato=full`;
const REC_CTA = 'Crear vídeo largo';
const BRIEF_CHECKBOX = /He revisado y apruebo los ajustes/;

const PLAN = {
  demo: { map: 'de_inferno' },
  target: { steamid64: '76561198000000001', name_in_demo: 'ropz', team_at_start: 'CT' },
  stats: { total_kills_target: 24 },
  segments: [{ id: 'r1', round: 1, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }] }],
};

const ROSTER = {
  players: [
    {
      steamid64: '76561198000000001',
      name: 'ropz',
      team: 'CT',
      kills: 24,
      deaths: 14,
      assists: 4,
    },
  ],
};

async function fulfillJson(page: Page, path: string, status: number, body: unknown): Promise<void> {
  await page.route(`**/api/demos/${JOB}${path}`, async (route) => {
    await route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify(body),
    });
  });
}

async function stubParsedMatch(page: Page, recap: { status: number; body: unknown }): Promise<void> {
  await fulfillJson(page, '/status', 200, { status: 'parsed' });
  await fulfillJson(page, '/plan', 200, PLAN);
  await fulfillJson(page, '/roster', 200, ROSTER);
  await fulfillJson(page, '/recap-plan', recap.status, recap.body);
}

test.describe('Full POV constructor', () => {
  test('lives under the numbered 01 section', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, PRODUCE_FULL);
    const key = page.locator('[data-slot="sidebar-menu-button"][href="/clips"]');
    await expect(key).toBeVisible();
    await expect(key).toContainText('01');
    await expect(key).toContainText('Clips y vídeos');
  });

  test('the format control switches between Short and Full POV without a reload', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, PRODUCE_FULL);
    const formats = page.getByRole('group', { name: 'Tipo de vídeo' });
    await expect(formats.getByRole('button', { name: 'Vídeo largo 16:9' })).toHaveAttribute('aria-pressed', 'true');
    await expect(page.getByRole('heading', { name: PRODUCE_FULL_TITLE })).toBeVisible();
    await formats.getByRole('button', { name: 'Short 9:16' }).click();
    await expect(page).toHaveURL(new RegExp(`/clips/${JOB}/nuevo$`));
    await expect(page.getByRole('heading', { name: PRODUCE_SHORT_TITLE })).toBeVisible();
  });

  test('a 404 job is a missing partida', async ({ page }) => {
    await page.route(`**/api/demos/${JOB}/status`, async (route) => {
      await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'not found' }) });
    });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('heading', { name: PRODUCE_MATCH_MISSING.title })).toBeVisible();
    await expect(page.getByText(PRODUCE_MATCH_MISSING.description)).toBeVisible();
    await expect(page.getByText(FULL_DEMO_EMPTY.error.title)).toHaveCount(0);
    await expect(page.getByRole('button', { name: REC_CTA })).toHaveCount(0);
  });

  test('a 500 from /plan is a load error, not a missing partida', async ({ page }) => {
    await fulfillJson(page, '/status', 200, { status: 'parsed' });
    await fulfillJson(page, '/plan', 500, { error: 'upstream error' });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('heading', { name: FULL_DEMO_EMPTY.error.title })).toBeVisible();
    await expect(page.getByText(FULL_DEMO_EMPTY.error.description)).toBeVisible();
    await expect(page.getByText(PRODUCE_MATCH_MISSING.title)).toHaveCount(0);
    await expect(page.getByRole('button', { name: REC_CTA })).toHaveCount(0);
  });

  test('a recap-plan 503 is offline, not a plan error', async ({ page }) => {
    await stubParsedMatch(page, { status: 503, body: { code: 'service_unavailable', error: 'offline' } });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('heading', { name: PRODUCE_FULL_TITLE })).toBeVisible();
    await expect(page.getByText(FULL_DEMO_EMPTY.offline.title)).toBeVisible();
    await expect(page.getByText(FULL_DEMO_RECAP_ERROR)).toHaveCount(0);
    await expect(page.getByText(PRODUCE_MATCH_MISSING.title)).toHaveCount(0);
  });

  test('a recap-plan 500 is an error, not a pending parse or Shorts empty state', async ({ page }) => {
    await stubParsedMatch(page, { status: 500, body: { error: 'upstream error' } });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('heading', { name: PRODUCE_FULL_TITLE })).toBeVisible();
    await expect(page.getByText(FULL_DEMO_RECAP_ERROR)).toBeVisible();
    await expect(page.getByText(FULL_DEMO_ROUNDS_PENDING)).toHaveCount(0);
    await expect(page.getByText(FULL_DEMO_FORGE_HINT_ERROR)).toBeVisible();
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toBeDisabled();
  });

  test('a recap-plan 409 talks about rondas, not Shorts highlights', async ({ page }) => {
    await stubParsedMatch(page, { status: 409, body: { error: 'recap plan not ready' } });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('heading', { name: PRODUCE_FULL_TITLE })).toBeVisible();
    await expect(page.getByText(FULL_DEMO_ROUNDS_PENDING)).toBeVisible();
    await expect(page.getByText(FULL_DEMO_RECAP_ERROR)).toHaveCount(0);
    await expect(page.getByText(FULL_DEMO_FORGE_HINT_EMPTY)).toBeVisible();
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
  });

  test('a ready recap-plan lists every round and gates REC behind the brief', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, PRODUCE_FULL);
    await expect(page.getByRole('heading', { name: PRODUCE_FULL_TITLE })).toBeVisible();
    await expect(page.getByText(PRODUCE_FULL_ROUNDS_NOTE)).toBeVisible();
    await expect(page.getByText('R01', { exact: true }).filter({ visible: true })).toBeVisible();
    await expect(page.getByText('Incluido en tu vídeo largo')).toBeVisible();
    await expect(page.getByText(FULL_DEMO_RECAP_ERROR)).toHaveCount(0);
    await expect(page.getByText(FULL_DEMO_ROUNDS_PENDING)).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'ELEGIR MÚSICA' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Sin música' })).toHaveCount(0);

    const cta = page.getByRole('button', { name: REC_CTA });
    await expect(cta).toBeDisabled();
    const brief = page.getByRole('region', { name: /Revisar antes de crear/ });
    for (const row of FULL_DEMO_CONTRACT) {
      await expect(brief.getByText(row.value, { exact: true })).toBeVisible();
    }
    await expect(brief.getByText(NATIVE_HUD_LABEL, { exact: true })).toBeVisible();
    await expect(page.getByText(/Este formato necesita acceso a FACEIT/)).toBeVisible();
    await expect(page.getByText('HUD completo con killfeed')).toHaveCount(0);
    await page.getByRole('checkbox', { name: BRIEF_CHECKBOX }).check();
    await expect(cta).toBeEnabled();
  });

  test('the retired /matches route keeps the series return path', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    const series = '22222222-2222-4222-8222-222222222222';
    await gotoStudio(page, `/matches/${JOB}?series=${series}`);
    await expect(page).toHaveURL(`/clips/${JOB}/nuevo?series=${series}`);
    await expect(page.getByRole('heading', { name: PRODUCE_SHORT_TITLE })).toBeVisible();
  });

  test('the overlay theme is the only Full POV choice and defaults to FACEIT orange', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, PRODUCE_FULL);
    const theme = page.getByRole('combobox', { name: 'Tema de overlays FACEIT' });
    await expect(theme).toContainText('FACEIT naranja');
    await page.getByRole('checkbox', { name: BRIEF_CHECKBOX }).check();
    await theme.click();
    await page.getByRole('option', { name: /Neón violeta/ }).click();
    await expect(theme).toContainText('Neón violeta');
    // Any decision change revokes the approval.
    await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).not.toBeChecked();
    await expect(page.getByRole('button', { name: REC_CTA })).toBeDisabled();
    await expect(page.getByRole('switch')).toHaveCount(0);
  });
});


test('switching formats preserves the long video theme and approval independently', async ({ page }) => {
  await stubParsedMatch(page, { status: 200, body: PLAN });
  await gotoStudio(page, PRODUCE_FULL);
  const theme = page.getByRole('combobox', { name: 'Tema de overlays FACEIT' });
  await theme.click();
  await page.getByRole('option', { name: /Neón violeta/ }).click();
  await page.getByRole('checkbox', { name: BRIEF_CHECKBOX }).check();
  await page.getByRole('button', { name: 'Short 9:16', exact: true }).click();
  await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).not.toBeChecked();
  await page.getByRole('button', { name: 'Vídeo largo 16:9', exact: true }).click();
  await expect(theme).toContainText('Neón violeta');
  await expect(page.getByRole('checkbox', { name: BRIEF_CHECKBOX })).toBeChecked();
});

for (const format of ['short', 'full']) {
  test(`the ${format} create bar stays at the bottom of a short work area`, async ({ page }) => {
    await page.setViewportSize({ width: 1920, height: 1440 });
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, `/clips/${JOB}/nuevo?formato=${format}`);
    const action = page.getByRole('button', { name: format === 'full' ? REC_CTA : 'Crear Short', exact: true });
    await expect(action).toBeVisible();
    const work = await page.locator('.measure-work').boundingBox();
    const button = await action.boundingBox();
    expect(work).not.toBeNull();
    expect(button).not.toBeNull();
    if (work === null || button === null) return;
    expect(work.y + work.height - button.y - button.height).toBeLessThan(48);
  });
}

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
