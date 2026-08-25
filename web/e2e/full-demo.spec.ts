import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { FULL_DEMO_CONTRACT } from '../lib/full-demo.ts';

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

test.describe('Full demo to video', () => {
  test('is a numbered production section', async ({ page }) => {
    await gotoStudio(page, '/full-demo');
    const key = page.locator('[data-slot="sidebar"] a[href="/full-demo"]');
    await expect(key).toBeVisible();
    await expect(key).toContainText('12');
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

  test('a missing job does not offer FORJAR or a music picker', async ({ page }) => {
    await gotoStudio(page, '/full-demo/11111111-1111-4111-8111-111111111111');
    await expect(page.getByText(/Servicio local sin conexión|Demo no encontrada/)).toBeVisible();
    await expect(page.getByRole('button', { name: 'FORJAR REEL' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'ELEGIR MÚSICA' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'AÑADIR MÚSICA' })).toHaveCount(0);
  });
});
