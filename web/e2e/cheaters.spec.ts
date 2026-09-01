import { expect, test, type Page } from '@playwright/test';
import { VALIDATION_WIDTHS, gotoStudio } from './contract.ts';

const JOB_ID = '3f2b9c14-7d6e-4a52-9b81-0c5e8f7a1d23';

const ROSTER = {
  players: [
    {
      steamid64: '76561198000000001',
      name: 'ropz',
      team: 'CT',
      kills: 24,
      deaths: 14,
      assists: 4,
      headshots: 12,
      mvps: 3,
      rounds: 22,
    },
  ],
  match: { map: 'de_inferno', score_ct: 13, score_t: 9, rounds: 22 },
};

async function stubJobs(page: Page, jobs: Array<Record<string, string>>): Promise<void> {
  await page.route('**/api/demos/jobs', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ jobs }),
    });
  });
  await page.route('**/api/demos/*/roster', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ROSTER) });
  });
  await page.route('**/api/demos/*/anticheat', async (route) => {
    if (route.request().method() === 'POST') {
      await route.fulfill({
        status: 202,
        contentType: 'application/json',
        body: JSON.stringify({ jobId: JOB_ID, status: 'running' }),
      });
      return;
    }
    await route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ error: 'not started' }) });
  });
}

test.describe('CheaterDetect', () => {
  test('empty state is a demo drop, not a detour to Shorts', async ({ page }) => {
    await stubJobs(page, []);
    await gotoStudio(page, '/cheaters');
    await expect(page.getByRole('heading', { name: 'CHEATERDETECT' })).toBeVisible();
    await expect(page.getByText('SUELTA UN .DEM AQUÍ')).toBeVisible();
    await expect(page.locator('input[type="file"]')).toHaveCount(1);
    await expect(page.locator('[data-layout="full"]')).toBeVisible();
    await expect(page.locator('main a[href="/upload"]')).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'SUBIR UNA DEMO' })).toHaveCount(0);
  });

  test('an already imported demo still offers a compact drop', async ({ page }) => {
    await stubJobs(page, [
      {
        jobId: JOB_ID,
        status: 'scanned',
        fileName: 'match730.dem',
        createdAt: '2026-08-14T18:15:37.000Z',
      },
    ]);
    await gotoStudio(page, '/cheaters');
    await expect(page.getByRole('navigation', { name: 'Demos analizables' }).getByText('Inferno')).toBeVisible();
    await expect(page.locator('[data-layout="compact"]')).toBeVisible();
    await expect(page.locator('input[type="file"]')).toHaveCount(1);
    await expect(page.getByText('SUELTA UN .DEM AQUÍ')).toBeVisible();
  });

  test('dropping a demo adds it to the picker without opening Shorts', async ({ page }) => {
    await stubJobs(page, []);
    await page.route('**/api/demos/scan', async (route) => {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ jobId: JOB_ID }),
      });
    });
    await page.route('**/api/demos/*/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'scanned' }),
      });
    });
    await gotoStudio(page, '/cheaters');
    await page.locator('input[type="file"]').setInputFiles({
      name: 'match730.dem',
      mimeType: 'application/octet-stream',
      buffer: Buffer.from('HL2DEMO\0e2e-fixture'),
    });
    await expect(page.getByRole('navigation', { name: 'Demos analizables' }).getByText('Inferno')).toBeVisible();
    await expect(page.locator('main a[href="/upload"]')).toHaveCount(0);
    await expect(page.locator('[data-layout="compact"]')).toBeVisible();
  });
});

const TARGET_SELECTOR =
  'a[href], button, input, select, textarea, [role="button"], label[for]';

async function undersizedTargets(page: Page): Promise<string[]> {
  return page.evaluate((selector) => {
    return [...document.querySelectorAll<HTMLElement>(selector)]
      .filter((node) => {
        const box = node.getBoundingClientRect();
        if (box.width === 0 || box.height === 0) return false;
        const style = getComputedStyle(node);
        if (style.visibility === 'hidden' || style.display === 'none') return false;
        if (box.width <= 2 && box.height <= 2) return false;
        if (node.tagName === 'A' && style.display.startsWith('inline') && style.display !== 'inline-block') {
          return false;
        }
        return box.height < 40 || box.width < 40;
      })
      .slice(0, 8)
      .map((node) => {
        const box = node.getBoundingClientRect();
        const label = node.getAttribute('aria-label') ?? node.textContent?.trim().slice(0, 30) ?? '';
        return `${node.tagName.toLowerCase()} "${label}" ${Math.round(box.width)}x${Math.round(box.height)}`;
      });
  }, TARGET_SELECTOR);
}

async function pageOverflow(page: Page): Promise<{ scrollWidth: number; clientWidth: number }> {
  return page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
}

test.describe('CheaterDetect presentation contract', () => {
  test('the empty drop is an operable file input, same hook as Shorts', async ({ page }) => {
    await stubJobs(page, []);
    await gotoStudio(page, '/cheaters');
    const input = page.locator('input[type="file"]');
    await expect(input).toHaveCount(1);
    expect(await input.isEnabled()).toBe(true);
    await expect(page.locator('[data-layout="full"]')).toBeVisible();
  });

  test('empty CheaterDetect keeps every interactive target at 40px or more', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await stubJobs(page, []);
    await gotoStudio(page, '/cheaters');
    await expect.poll(() => undersizedTargets(page), { message: 'targets under 40px', timeout: 10_000 }).toEqual([]);
  });

  test('populated CheaterDetect keeps drop + picker at 40px or more', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await stubJobs(page, [
      {
        jobId: JOB_ID,
        status: 'scanned',
        fileName: 'match730.dem',
        createdAt: '2026-08-14T18:15:37.000Z',
      },
    ]);
    await gotoStudio(page, '/cheaters');
    await expect(page.locator('[data-layout="compact"]')).toBeVisible();
    await expect.poll(() => undersizedTargets(page), { message: 'targets under 40px', timeout: 10_000 }).toEqual([]);
  });

  for (const width of VALIDATION_WIDTHS) {
    test(`empty CheaterDetect does not overflow at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await stubJobs(page, []);
      await gotoStudio(page, '/cheaters');
      await expect(page.getByText('SUELTA UN .DEM AQUÍ')).toBeVisible();
      const root = await pageOverflow(page);
      expect(root.scrollWidth, `overflows by ${root.scrollWidth - root.clientWidth}px`).toBeLessThanOrEqual(
        root.clientWidth,
      );
    });

    test(`populated CheaterDetect does not overflow at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await stubJobs(page, [
        {
          jobId: JOB_ID,
          status: 'scanned',
          fileName: 'match730.dem',
          createdAt: '2026-08-14T18:15:37.000Z',
        },
      ]);
      await gotoStudio(page, '/cheaters');
      await expect(page.locator('[data-layout="compact"]')).toBeVisible();
      const root = await pageOverflow(page);
      expect(root.scrollWidth, `overflows by ${root.scrollWidth - root.clientWidth}px`).toBeLessThanOrEqual(
        root.clientWidth,
      );
    });
  }
});
