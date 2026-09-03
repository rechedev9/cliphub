import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { HUB_EMPTY_TITLE } from '../lib/clips/copy.ts';

/**
 * 01 Clips y vídeos hub. The suite runs without an orchestrator, so the
 * default is the offline first-run state; the stubbed test serves one parsed
 * partida through `/api/demos/jobs` + `/api/demos/{id}/roster` and one ready
 * Short through the reel intent store + `/api/demos/{id}/status` +
 * `/api/demos/{id}/renders/{variant}`.
 */
const JOB_ID = '3f2b9c14-7d6e-4a52-9b81-0c5e8f7a1d23';
const VIDEO_ID = `${JOB_ID}__seg-001`;
const VARIANT = 'viral-60-clean';
const TARGET = { steamid64: '76561198000000002', name: 'donk' };

const SHORT_EDIT = {
  format: 'short-9x16',
  killEffect: 'punch-in',
  transition: 'flash',
  intro: false,
  outro: false,
  hookText: false,
  killCounter: false,
  matchRecap: false,
  voiceComms: false,
  nativeHud: false,
  coverStrategy: 'generated-gameplay',
  introText: '',
  outroText: '',
} as const;

async function stubParsedMatchWithReadyShort(page: Page): Promise<void> {
  await page.addInitScript(
    (intent) => {
      window.localStorage.setItem('cliphub.reels.v1', JSON.stringify([intent]));
    },
    {
      videoId: VIDEO_ID,
      jobId: JOB_ID,
      segmentIds: ['seg-001'],
      mode: 'clean',
      variant: VARIANT,
      editConfig: SHORT_EDIT,
      title: 'Triple en pistola',
      map: 'de_mirage',
      score: '13-9',
      targetName: TARGET.name,
      createdAt: 1_756_800_000_000,
    },
  );

  await page.route('**/api/demos/jobs', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        jobs: [
          {
            jobId: JOB_ID,
            status: 'parsed',
            fileName: 'match730.dem',
            targetSteamId: TARGET.steamid64,
            createdAt: '2026-09-01T10:00:00Z',
          },
        ],
      }),
    }),
  );
  await page.route(`**/api/demos/${JOB_ID}/roster`, (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        players: [{ ...TARGET, team: 'T', kills: 31, deaths: 17, assists: 2, rating: 1.3 }],
        match: { map: 'de_mirage', score_ct: 9, score_t: 13, rounds: 22 },
      }),
    }),
  );
  await page.route(`**/api/demos/${JOB_ID}/status`, (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ status: 'done' }) }),
  );
  await page.route(`**/api/demos/${JOB_ID}/renders/${VARIANT}`, (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        status: 'ready',
        videos: ['triple-en-pistola.mp4'],
        covers: [],
        segment_ids: ['seg-001'],
        edit: {
          format: SHORT_EDIT.format,
          killEffect: SHORT_EDIT.killEffect,
          transition: SHORT_EDIT.transition,
          intro: false,
          outro: false,
          hook_text: false,
          kill_counter: false,
          match_recap: false,
          voice_comms: false,
          native_hud: false,
          cover_strategy: SHORT_EDIT.coverStrategy,
        },
      }),
    }),
  );
}

test.describe('clips hub', () => {
  test('first run shows the demo door and nothing else', async ({ page }) => {
    await gotoStudio(page, '/clips');
    const empty = page.locator(`section[aria-label="${HUB_EMPTY_TITLE}"]`);
    await expect(empty).toBeVisible();
    await expect(empty.locator('input[type="file"]')).toHaveCount(1);
    const links = empty.locator('a');
    const count = await links.count();
    expect(count).toBeLessThanOrEqual(1);
    if (count === 1) await expect(links).toHaveAttribute('href', '/clips/nueva');
    await expect(page.getByRole('tab', { name: /Partidas/ })).toHaveCount(0);
  });

  test('a parsed partida lists its Short and opens on click', async ({ page }) => {
    await stubParsedMatchWithReadyShort(page);
    await gotoStudio(page, '/clips');

    const row = page.locator(`#partida-${JOB_ID}`);
    await expect(row).toBeVisible();
    await expect(row.getByText('Mirage', { exact: true })).toBeVisible();
    await expect(row.getByText('Shorts · 1')).toBeVisible();

    await row.getByRole('button', { expanded: false }).first().click();
    await expect(row.getByRole('button', { expanded: true })).toBeVisible();

    const publish = row.getByRole('link', { name: 'Publicar' });
    await expect(publish).toHaveAttribute('href', `/clips/${JOB_ID}/publicar/${encodeURIComponent(VIDEO_ID)}`);
    await expect(row.getByRole('button', { name: 'MP4' })).toBeEnabled();
  });
});
