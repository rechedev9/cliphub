import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import {
  HUB_EMPTY_TITLE,
  HUB_ORPHANS_TITLE,
  MATCH_ROW_UNPICKED_CTA,
  MATCH_ROW_UNPICKED_TITLE,
} from '../lib/clips/copy.ts';

/**
 * 01 Clips y vídeos hub. The suite runs without an orchestrator, so the
 * default is the offline first-run state; the stubbed test serves one parsed
 * partida through `/api/demos/jobs` + `/api/demos/{id}/roster`, one `scanned`
 * partida with no POV picked, one ready Short and one still-rendering Short
 * through the reel intent store + `/api/demos/{id}/status` +
 * `/api/demos/{id}/renders/{variant}`, plus one orphan Short whose job is gone.
 */
const JOB_ID = '3f2b9c14-7d6e-4a52-9b81-0c5e8f7a1d23';
const VIDEO_ID = `${JOB_ID}__seg-001`;
const VARIANT = 'viral-60-clean';
const RENDERING_VIDEO_ID = `${JOB_ID}__seg-002`;
const RENDERING_VARIANT = 'viral-aggressive-60';
const SCANNED_JOB_ID = '9a1c6e2f-4b3d-4a10-8f2e-1d6c7b9a0e55';
const GONE_JOB_ID = '5d7e0b3a-2c1f-4e88-9a64-7b2d1c0e9f31';
const ORPHAN_VIDEO_ID = `${GONE_JOB_ID}__seg-001`;
const ORPHAN_TITLE = 'Clutch en Overpass';
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
    (intents) => {
      window.localStorage.setItem('cliphub.reels.v1', JSON.stringify(intents));
    },
    [
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
      {
        videoId: RENDERING_VIDEO_ID,
        jobId: JOB_ID,
        segmentIds: ['seg-002'],
        mode: 'clean',
        variant: RENDERING_VARIANT,
        editConfig: SHORT_EDIT,
        title: 'Ace en humo',
        map: 'de_mirage',
        score: '13-9',
        targetName: TARGET.name,
        createdAt: 1_756_800_100_000,
      },
      {
        videoId: ORPHAN_VIDEO_ID,
        jobId: GONE_JOB_ID,
        segmentIds: ['seg-001'],
        mode: 'clean',
        variant: VARIANT,
        editConfig: SHORT_EDIT,
        title: ORPHAN_TITLE,
        map: 'de_overpass',
        score: '16-14',
        targetName: TARGET.name,
        createdAt: 1_756_700_000_000,
      },
    ],
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
          {
            jobId: SCANNED_JOB_ID,
            status: 'scanned',
            fileName: 'match731.dem',
            createdAt: '2026-09-02T10:00:00Z',
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
  await page.route(`**/api/demos/${SCANNED_JOB_ID}/roster`, (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        players: [{ steamid64: '76561198000000003', name: 'zywoo', team: 'CT', kills: 24, deaths: 12, assists: 4, rating: 1.4 }],
        match: { map: 'de_inferno', score_ct: 16, score_t: 10, rounds: 26 },
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
  await page.route(`**/api/demos/${JOB_ID}/renders/${RENDERING_VARIANT}`, (route) =>
    route.fulfill({ contentType: 'application/json', body: JSON.stringify({ status: 'rendering' }) }),
  );
  // The orphan's job was deleted: every call about it 404s and it is not in `/jobs`.
  await page.route(`**/api/demos/${GONE_JOB_ID}/**`, (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ code: 'not_found' }) }),
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
    await expect(row.getByText('Shorts · 2')).toBeVisible();

    await row.getByRole('button', { expanded: false }).first().click();
    await expect(row.getByRole('button', { expanded: true })).toBeVisible();
    // Header "Shorts · 2" and the column count must agree and say shorts, not clips.
    await expect(row.getByText('2 shorts', { exact: true })).toBeVisible();

    const publish = row.getByRole('link', { name: 'Publicar' });
    await expect(publish).toHaveAttribute('href', `/clips/${JOB_ID}/publicar/${encodeURIComponent(VIDEO_ID)}`);
    await expect(row.getByRole('button', { name: 'MP4' })).toBeEnabled();

    // The still-rendering Short has no result yet, but it must stay deletable
    // (bug: queued/rendering outputs had no delete affordance).
    await expect(row.getByRole('button', { name: 'Borrar Ace en humo' })).toBeVisible();
  });

  test('a scanned partida (no POV picked) shows the unpicked copy and stays deletable', async ({ page }) => {
    await stubParsedMatchWithReadyShort(page);
    await gotoStudio(page, '/clips');

    const row = page.locator(`#partida-${SCANNED_JOB_ID}`);
    await expect(row).toBeVisible();
    await expect(row.getByText(MATCH_ROW_UNPICKED_TITLE)).toBeVisible();

    // Not "still parsing": the spinner copy from the real parsing row must be absent.
    await expect(row.getByText(/Parseando/)).toHaveCount(0);

    const cta = row.getByRole('link', { name: MATCH_ROW_UNPICKED_CTA });
    await expect(cta).toHaveAttribute('href', `/clips/nueva?job=${SCANNED_JOB_ID}`);

    // Bug: a scanned job could never be removed. It must always be deletable.
    await expect(row.getByRole('button', { name: /^Borrar / })).toBeVisible();
  });

  test('the unpicked CTA resumes the scan and picking a player POSTs parse', async ({ page }) => {
    await stubParsedMatchWithReadyShort(page);
    await page.route(`**/api/demos/${SCANNED_JOB_ID}/status`, (route) =>
      route.fulfill({ contentType: 'application/json', body: JSON.stringify({ status: 'scanned' }) }),
    );
    await page.route(`**/api/demos/${SCANNED_JOB_ID}/parse`, (route) =>
      route.fulfill({ contentType: 'application/json', body: JSON.stringify({ jobId: SCANNED_JOB_ID }) }),
    );
    await gotoStudio(page, '/clips');

    await page.locator(`#partida-${SCANNED_JOB_ID}`).getByRole('link', { name: MATCH_ROW_UNPICKED_CTA }).click();
    await expect(page).toHaveURL(new RegExp(`/clips/nueva\\?job=${SCANNED_JOB_ID}$`));

    // No upload: the existing roster lands straight in the picker.
    await expect(page.locator('[data-testid="player-avatar"]')).toHaveCount(1);
    await expect(page.getByText('zywoo', { exact: true }).first()).toBeVisible();
    await expect(page.locator('input[type="file"]')).toHaveCount(0);

    const parsePost = page.waitForRequest(
      (request) => request.method() === 'POST' && request.url().includes(`/api/demos/${SCANNED_JOB_ID}/parse`),
    );
    await page.getByRole('button', { name: /^Parsear POV/ }).click();
    expect((await parsePost).postDataJSON()).toEqual({ steamId: '76561198000000003' });
    await expect(page.getByText('Parseando la POV…')).toBeVisible();
  });

  test('a reel whose job is gone lists under Otros clips instead of vanishing', async ({ page }) => {
    await stubParsedMatchWithReadyShort(page);
    await gotoStudio(page, '/clips');

    const others = page.locator(`section[aria-label="${HUB_ORPHANS_TITLE}"]`);
    await expect(others).toBeVisible();
    await expect(others.getByText(ORPHAN_TITLE)).toBeVisible();
    await expect(others.getByRole('button', { name: `Borrar ${ORPHAN_TITLE}` })).toBeVisible();
    // No partida row claims it.
    await expect(page.locator(`#partida-${GONE_JOB_ID}`)).toHaveCount(0);
  });
});
