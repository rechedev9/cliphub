import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { FULL_DEMO_EMPTY } from '../lib/full-demo.ts';
import { PRODUCE_MATCH_MISSING } from '../lib/produce/copy.ts';

/** Roster picker on /clips/nueva. Network is stubbed; this is the presentation contract. */
const JOB_ID = '3f2b9c14-7d6e-4a52-9b81-0c5e8f7a1d23';

const ROSTER_PLAYERS = [
  { steamid64: '76561198000000001', name: 'ropz', team: 'CT', kills: 24, deaths: 14, assists: 4 },
  { steamid64: '76561198000000002', name: 'donk', team: 'T', kills: 31, deaths: 17, assists: 2 },
  { steamid64: '76561198000000003', name: 'zywoo', team: 'CT', kills: 22, deaths: 18, assists: 6 },
  { steamid64: '76561198000000004', name: 'niko', team: 'T', kills: 19, deaths: 19, assists: 5 },
  { steamid64: '76561198000000005', name: 'm0nesy', team: 'CT', kills: 26, deaths: 15, assists: 3 },
] as const;

/** Serves the scan → status → roster triplet the upload flow walks. */
async function stubRosterScan(page: Page): Promise<void> {
  await page.route('**/api/demos/scan', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ jobId: JOB_ID }) });
  });

  await page.route('**/api/demos/*/status', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'scanned' }) });
  });

  await page.route('**/api/demos/*/roster', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        players: ROSTER_PLAYERS.map((player, index) => ({
          ...player,
          headshots: 10 + index,
          mvps: index,
          rounds: 24,
          adr: 78.5 + index,
          hs_pct: 52.1,
          kast: 71.4,
          rating: 1.18,
        })),
        match: { map: 'de_mirage', score_team_a: 13, score_team_b: 9 },
      }),
    });
  });
}

/** Drops a demo through the real file input and waits for the picker. */
async function scanDemo(page: Page): Promise<void> {
  await stubRosterScan(page);
  await gotoStudio(page, '/clips/nueva');

  await expect(page.locator('input[type="file"]')).toBeEnabled();
  await page.locator('input[type="file"]').setInputFiles({
    name: 'match.dem',
    mimeType: 'application/octet-stream',
    buffer: Buffer.from('HL2DEMO\0e2e-fixture'),
  });

  await expect(page.locator('[data-testid="player-avatar"]').first()).toBeVisible();
}

test.describe('roster flow', () => {
  test('a scanned demo renders the picker with every player', async ({ page }) => {
    await scanDemo(page);
    await expect(page.locator('[data-testid="player-avatar"]')).toHaveCount(ROSTER_PLAYERS.length);
    for (const { name } of ROSTER_PLAYERS) {
      await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
    }
  });

  test('the picker sits in a card', async ({ page }) => {
    await scanDemo(page);
    expect(await page.locator('[data-slot="card"]').count()).toBeGreaterThan(0);
  });

  test('every roster row is a 40px-or-larger target', async ({ page }) => {
    await scanDemo(page);
    const undersized = await page.evaluate(() => {
      return [...document.querySelectorAll<HTMLElement>('button, [role="button"]')]
        .filter((node) => {
          const box = node.getBoundingClientRect();
          if (box.width === 0 || box.height === 0) return false;
          return box.height < 40 || box.width < 40;
        })
        .map((node) => {
          const box = node.getBoundingClientRect();
          const label = node.getAttribute('aria-label') ?? node.textContent?.trim().slice(0, 30) ?? '';
          return `"${label}" ${Math.round(box.width)}x${Math.round(box.height)}`;
        });
    });
    expect(undersized, `targets under 40px: ${undersized.join(' | ')}`).toEqual([]);
  });

  test('the picker does not overflow at any validation width', async ({ page }) => {
    await scanDemo(page);
    for (const width of [390, 768, 1024, 1280, 1440, 1920]) {
      await page.setViewportSize({ width, height: 900 });
      const root = await page.evaluate(() => ({
        scrollWidth: document.documentElement.scrollWidth,
        clientWidth: document.documentElement.clientWidth,
      }));
      expect(root.scrollWidth, `roster overflows at ${width}px`).toBeLessThanOrEqual(root.clientWidth);
    }
  });
});

test.describe('resume a scanned job', () => {
  test('?job= loads the existing roster into the same picker without an upload', async ({ page }) => {
    await stubRosterScan(page);
    await gotoStudio(page, `/clips/nueva?job=${JOB_ID}`);

    await expect(page.locator('[data-testid="player-avatar"]')).toHaveCount(ROSTER_PLAYERS.length);
    await expect(page.getByText('Mirage', { exact: false }).first()).toBeVisible();
    await expect(page.locator('input[type="file"]')).toHaveCount(0);
  });

  test('a job that is gone shows the not-found panel', async ({ page }) => {
    await page.route('**/api/demos/*/status', (route) =>
      route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'job not found' }) }),
    );
    await gotoStudio(page, `/clips/nueva?job=${JOB_ID}`);

    await expect(page.getByText(PRODUCE_MATCH_MISSING.title)).toBeVisible();
    await expect(page.locator('[data-testid="player-avatar"]')).toHaveCount(0);
  });

  test('a malformed job id is treated as not found, never sent upstream', async ({ page }) => {
    const upstream: string[] = [];
    await page.route('**/api/demos/**', (route) => {
      upstream.push(route.request().url());
      return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'nope' }) });
    });
    await gotoStudio(page, '/clips/nueva?job=..%2Fjobs');

    await expect(page.getByText(PRODUCE_MATCH_MISSING.title)).toBeVisible();
    // The shell's activity monitor lists jobs on every page; nothing else may be called.
    expect(upstream.filter((url) => !url.endsWith('/api/demos/jobs'))).toEqual([]);
  });

  test('a job that already has its POV redirects to its produce page', async ({ page }) => {
    await stubRosterScan(page);
    await page.route('**/api/demos/*/status', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'parsed' }) }),
    );
    await gotoStudio(page, `/clips/nueva?job=${JOB_ID}`);

    await expect(page).toHaveURL(new RegExp(`/clips/${JOB_ID}/nuevo$`));
  });

  test('a dead local service is told apart from a missing job', async ({ page }) => {
    await page.route('**/api/demos/*/status', (route) =>
      route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ code: 'service_unavailable' }) }),
    );
    await gotoStudio(page, `/clips/nueva?job=${JOB_ID}`);

    await expect(page.getByText(FULL_DEMO_EMPTY.offline.title)).toBeVisible();
  });
});

test.describe('scan failure states', () => {
  test('an empty roster is reported as a bad demo, not a spinner', async ({ page }) => {
    await stubRosterScan(page);
    await page.route('**/api/demos/*/roster', async (route) => {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ players: [] }) });
    });
    await gotoStudio(page, '/clips/nueva');

    await expect(page.locator('input[type="file"]')).toBeEnabled();
  await page.locator('input[type="file"]').setInputFiles({
      name: 'empty.dem',
      mimeType: 'application/octet-stream',
      buffer: Buffer.from('HL2DEMO\0empty'),
    });

    // State is never colour alone, so the failure has to be text
    // in a live region rather than a red edge on a still-spinning dropzone.
    await expect(page.locator('[role="alert"]').first()).toBeVisible();
    await expect(page.locator('[data-testid="player-avatar"]')).toHaveCount(0);
  });

  test('a dead local service is reported, not swallowed', async ({ page }) => {
    await page.route('**/api/demos/scan', async (route) => {
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'service_unavailable' }),
      });
    });
    await gotoStudio(page, '/clips/nueva');

    await expect(page.locator('input[type="file"]')).toBeEnabled();
  await page.locator('input[type="file"]').setInputFiles({
      name: 'offline.dem',
      mimeType: 'application/octet-stream',
      buffer: Buffer.from('HL2DEMO\0offline'),
    });

    await expect(page.locator('[role="alert"]').first()).toBeVisible();
  });
});

for (const format of ['short', 'full']) {
  test(`creation intent ${format} survives a resumed player selection`, async ({ page }) => {
    await stubRosterScan(page);
    await page.route(`**/api/demos/${JOB_ID}/parse`, (route) =>
      route.fulfill({ json: { jobId: JOB_ID } }),
    );
    await gotoStudio(page, `/clips/nueva?job=${JOB_ID}&formato=${format}`);
    const next = page.getByRole('button', { name: format === 'full' ? 'Continuar al vídeo largo' : 'Continuar al Short' });
    await expect(next).toBeEnabled();
    await expect(page.getByRole('button', { name: 'Preparar vídeo largo', exact: true })).toHaveCount(0);
    const request = page.waitForRequest((req) => req.method() === 'POST' && req.url().endsWith('/parse'));
    await next.click();
    expect((await request).postDataJSON()).toHaveProperty('steamId');
    await page.route(`**/api/demos/${JOB_ID}/status`, (route) => route.fulfill({ json: { status: 'parsed' } }));
    await page.route(`**/api/demos/${JOB_ID}/plan`, (route) => route.fulfill({ json: { demo: { map: 'de_mirage' }, target: { steamid64: '76561198000000001', name_in_demo: 'ropz' }, stats: {}, segments: [] } }));
    await expect(page).toHaveURL(new RegExp(`/clips/${JOB_ID}/nuevo${format === 'full' ? '\\?formato=full' : ''}$`));
  });
}
