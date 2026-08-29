import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import {
  MATCH_PLAYS_ANALYZING_DESCRIPTION,
  MATCH_PLAYS_ANALYZING_TITLE,
  MATCH_PLAYS_EMPTY_TITLE,
} from '../lib/match-plays-empty.ts';
import { FULL_DEMO_EDIT } from '../lib/full-demo.ts';

const JOB = '11111111-1111-4111-8111-111111111111';

async function fulfillJson(page: Page, path: string, status: number, body: unknown): Promise<void> {
  await page.route(`**/api/demos/${JOB}${path}`, async (route) => {
    await route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify(body),
    });
  });
}

test.describe('Matches Shorts constructor empty states', () => {
  test('pending parse shows analyzing, not Sin jugadas destacables', async ({ page }) => {
    await fulfillJson(page, '/status', 200, { status: 'parsing' });
    await fulfillJson(page, '/roster', 200, {
      players: [
        {
          steamid64: '76561198000000001',
          name: 'ropz',
          team: 'CT',
          kills: 10,
          deaths: 8,
          assists: 2,
        },
      ],
      match: { map: 'de_inferno', score_team_a: 7, score_team_b: 5 },
    });
    await gotoStudio(page, `/matches/${JOB}`);
    await expect(page.getByRole('heading', { name: MATCH_PLAYS_ANALYZING_TITLE })).toBeVisible();
    await expect(page.getByText(MATCH_PLAYS_ANALYZING_DESCRIPTION)).toBeVisible();
    await expect(page.getByText(MATCH_PLAYS_EMPTY_TITLE)).toHaveCount(0);
    await expect(page.getByRole('link', { name: /Vídeo completo 16:9/i })).toBeVisible();
  });

  test('plan-ready with zero plays keeps Sin jugadas destacables', async ({ page }) => {
    await fulfillJson(page, '/status', 200, { status: 'parsed' });
    await fulfillJson(page, '/plan', 200, {
      demo: { map: 'de_inferno' },
      target: { steamid64: '76561198000000001', name_in_demo: 'ropz', team_at_start: 'CT' },
      stats: { total_kills_target: 0 },
      segments: [],
    });
    await fulfillJson(page, '/roster', 200, {
      players: [
        {
          steamid64: '76561198000000001',
          name: 'ropz',
          team: 'CT',
          kills: 0,
          deaths: 10,
          assists: 0,
        },
      ],
    });
    await gotoStudio(page, `/matches/${JOB}`);
    await expect(page.getByRole('heading', { name: MATCH_PLAYS_EMPTY_TITLE })).toBeVisible();
    await expect(page.getByText(MATCH_PLAYS_ANALYZING_TITLE)).toHaveCount(0);
  });
});

test.describe('Library Full Demo identity', () => {
  test('marks landscape recap cards as PARTIDA COMPLETA', async ({ page }) => {
    await page.addInitScript((value) => {
      window.localStorage.setItem('cliphub.reels.v1', JSON.stringify(value));
    }, [
      {
        videoId: `${JOB}__full-demo`,
        jobId: JOB,
        segmentIds: [],
        mode: 'clean',
        variant: 'gameplay-pov-60',
        editConfig: FULL_DEMO_EDIT,
        title: '24 rondas - POV nativo',
        map: 'de_inferno',
        score: '',
        targetName: 'ropz',
        createdAt: Date.now(),
      },
    ]);
    await page.route(`**/api/demos/${JOB}/status`, (route) =>
      route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({ status: 'recording', progress: { done: 1, total: 10, percent: 10 } }),
      }),
    );
    await gotoStudio(page, '/videos');
    await expect(page.getByRole('heading', { name: 'BIBLIOTECA' })).toBeVisible();
    await expect(page.getByLabel('Vídeos').getByText('PARTIDA COMPLETA')).toBeVisible();
    await expect(page.getByLabel('Vídeos').getByText('16:9', { exact: true })).toBeVisible();
  });
});
