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

const JOB = '11111111-1111-4111-8111-111111111111';

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

const PARSED_JOB_ID = '11111111-1111-4111-8111-111111111111';

async function stubParsedInferno(page: Page): Promise<void> {
  await page.route('**/api/demos/jobs', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        jobs: [
          {
            jobId: PARSED_JOB_ID,
            status: 'parsed',
            fileName: 'inferno.dem',
            createdAt: '2026-08-25T12:00:00Z',
          },
        ],
      }),
    });
  });
  await page.route(`**/api/demos/${PARSED_JOB_ID}/roster`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        players: [
          {
            steamid64: '76561198000000001',
            name: 'player',
            team: 'CT',
            kills: 1,
            deaths: 0,
            assists: 0,
            headshots: 0,
            mvps: 0,
            rounds: 1,
            adr: 0,
            hs_pct: 0,
            kast: 0,
            rating: 0,
          },
        ],
        match: { map: 'de_inferno', score_team_a: 13, score_team_b: 9 },
      }),
    });
  });
}

async function stubNativePovPreset(page: Page): Promise<void> {
  await page.route('**/api/presets', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        presets: [
          {
            name: 'gameplay-pov-60',
            label: 'POV nativo',
            description: 'YouTube POV with native CS2 HUD',
            hud_mode: 'gameplay',
          },
        ],
      }),
    });
  });
}

test.describe('Full demo to video', () => {
  test('is a numbered production section', async ({ page }) => {
    await gotoStudio(page, '/full-demo');
    const key = page.locator('[data-slot="sidebar"] a[href="/full-demo"]');
    await expect(key).toBeVisible();
    await expect(key).toContainText('03');
    await expect(key).toContainText('Full demo to video');
  });

  test('states the locked 16:9 recap contract from shipped constants', async ({ page }) => {
    await gotoStudio(page, '/full-demo');
    await expect(page.getByRole('heading', { name: 'FULL DEMO TO VIDEO' })).toBeVisible();
    for (const row of FULL_DEMO_CONTRACT) {
      await expect(page.getByText(row.value, { exact: true })).toBeVisible();
    }
    await expect(page.getByRole('button', { name: 'ELEGIR MÚSICA' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'SIN MÚSICA' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Preset (POV nativo|Native POV)/ })).toBeVisible();
  });

  test('empty state uses the same demo drop as Subir demo', async ({ page }) => {
    await gotoStudio(page, '/full-demo');
    await expect(page.locator('p[role="alert"]')).toContainText(
      /El servicio de análisis está offline|No se pudieron cargar las demos parseadas/,
    );
    await expect(page.getByText('SUELTA UN .DEM AQUÍ')).toBeVisible();
    await expect(page.locator('input[type="file"]')).toHaveCount(1);
    await expect(page.locator('main a[href="/onboarding"]')).toHaveCount(0);
  });

  test('keeps the demo drop when a parsed match is already listed', async ({ page }) => {
    await stubParsedInferno(page);
    await gotoStudio(page, '/full-demo');
    const listed = page.locator(`main a[href="/full-demo/${PARSED_JOB_ID}"]`);
    await expect(listed).toBeVisible();
    await expect(listed).toContainText(/Inferno/i);
    await expect(page.getByText('SUELTA UN .DEM AQUÍ')).toBeVisible();
    await expect(page.locator('input[type="file"]')).toHaveCount(1);
  });

  test('a 404 job is a missing demo', async ({ page }) => {
    await page.route(`**/api/demos/${JOB}/status`, async (route) => {
      await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'not found' }) });
    });
    await gotoStudio(page, `/full-demo/${JOB}`);
    await expect(page.getByRole('heading', { name: FULL_DEMO_EMPTY.missing.title })).toBeVisible();
    await expect(page.getByText(FULL_DEMO_EMPTY.missing.description)).toBeVisible();
    await expect(page.getByText(FULL_DEMO_EMPTY.error.title)).toHaveCount(0);
  });

  test('a missing job does not offer FORJAR or a music picker', async ({ page }) => {
    await gotoStudio(page, `/full-demo/${JOB}`);
    await expect(page.getByText(/Servicio local sin conexión|Demo no encontrada/)).toBeVisible();
    await expect(page.getByRole('button', { name: 'FORJAR REEL' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'ELEGIR MÚSICA' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'AÑADIR MÚSICA' })).toHaveCount(0);
  });

  test('a 500 from /plan is a load error, not a missing demo', async ({ page }) => {
    await page.route(`**/api/demos/${JOB}/status`, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'parsed' }),
      });
    });
    await page.route(`**/api/demos/${JOB}/plan`, async (route) => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'upstream error' }),
      });
    });
    await gotoStudio(page, `/full-demo/${JOB}`);
    await expect(page.getByRole('heading', { name: FULL_DEMO_EMPTY.error.title })).toBeVisible();
    await expect(page.getByText(FULL_DEMO_EMPTY.error.description)).toBeVisible();
    await expect(page.getByText(FULL_DEMO_EMPTY.missing.title)).toHaveCount(0);
    await expect(page.getByText(FULL_DEMO_EMPTY.missing.description)).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'FORJAR REEL' })).toHaveCount(0);
  });

  test('a recap-plan 500 is an error, not a pending parse or Shorts empty state', async ({ page }) => {
    await stubParsedMatch(page, { status: 500, body: { error: 'upstream error' } });
    await gotoStudio(page, `/full-demo/${JOB}`);
    await expect(page.getByRole('heading', { name: 'INFERNO' })).toBeVisible();
    await expect(page.getByText(FULL_DEMO_RECAP_ERROR)).toBeVisible();
    await expect(page.getByText(FULL_DEMO_ROUNDS_PENDING)).toHaveCount(0);
    await expect(page.getByText('Demo no encontrada')).toHaveCount(0);
    await expect(page.getByText('Elige al menos una jugada para empezar.')).toHaveCount(0);
    await expect(page.getByText(FULL_DEMO_FORGE_HINT_ERROR)).toBeVisible();
    await expect(page.locator('[aria-label="Formato del reel"]')).toHaveText('16:9');
    await expect(page.getByRole('button', { name: 'FORJAR REEL' })).toBeDisabled();
  });

  test('a recap-plan 409 talks about rondas, not Shorts jugadas', async ({ page }) => {
    await stubParsedMatch(page, { status: 409, body: { error: 'recap plan not ready' } });
    await gotoStudio(page, `/full-demo/${JOB}`);
    await expect(page.getByRole('heading', { name: 'INFERNO' })).toBeVisible();
    await expect(page.getByText(FULL_DEMO_ROUNDS_PENDING)).toBeVisible();
    await expect(page.getByText(FULL_DEMO_RECAP_ERROR)).toHaveCount(0);
    await expect(page.getByText('Elige al menos una jugada para empezar.')).toHaveCount(0);
    await expect(page.getByText(FULL_DEMO_FORGE_HINT_EMPTY)).toBeVisible();
    await expect(page.locator('[aria-label="Formato del reel"]')).toHaveText('16:9');
    await expect(page.getByRole('button', { name: 'FORJAR REEL' })).toBeDisabled();
  });

  test('a ready recap-plan names rondas and keeps 16:9', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await gotoStudio(page, `/full-demo/${JOB}`);
    await expect(page.getByRole('heading', { name: 'INFERNO' })).toBeVisible();
    await expect(page.getByText('Elige un preset para continuar.')).toBeVisible();
    await expect(page.getByText(FULL_DEMO_RECAP_ERROR)).toHaveCount(0);
    await expect(page.getByText(FULL_DEMO_ROUNDS_PENDING)).toHaveCount(0);
    await expect(page.getByText('Elige al menos una jugada para empezar.')).toHaveCount(0);
    await expect(page.locator('[aria-label="Formato del reel"]')).toHaveText('16:9');
  });

  test('job brief and POV chip name native CS2 HUD, not Shorts full-hud copy', async ({ page }) => {
    await stubParsedMatch(page, { status: 200, body: PLAN });
    await stubNativePovPreset(page);
    await gotoStudio(page, `/full-demo/${JOB}`);
    await expect(page.getByRole('heading', { name: 'INFERNO' })).toBeVisible();
    const preset = page.getByRole('button', { name: /Preset (POV nativo|Native POV)/ });
    await preset.click();
    await expect(preset).toContainText(`HUD · ${NATIVE_HUD_LABEL}`);
    const brief = page.getByRole('region', { name: 'Brief creativo exacto' });
    await expect(brief.getByText('HUD / killfeed:', { exact: true })).toBeVisible();
    await expect(brief.getByText(NATIVE_HUD_LABEL, { exact: true })).toBeVisible();
    await expect(page.getByText('HUD completo con killfeed')).toHaveCount(0);
    await expect(page.getByText(/HUD · gameplay/i)).toHaveCount(0);
  });
});
