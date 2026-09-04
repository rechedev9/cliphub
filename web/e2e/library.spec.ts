import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

const RECORDING_JOB_ID = '11111111-1111-4111-8111-111111111111';
const FAILED_JOB_ID = '22222222-2222-4222-8222-222222222222';

/** The Clips lens of 01: `/clips?vista=clips`. */
const CLIPS_LENS_HREF = '/clips?vista=clips';

function fullDemoIntent(jobId: string, title: string) {
  return {
    videoId: `${jobId}__full-demo`,
    jobId,
    segmentIds: [],
    mode: 'clean',
    variant: 'gameplay-pov-60',
    editConfig: {
      format: 'landscape-16x9',
      killEffect: 'clean',
      transition: 'cut',
      intro: false,
      outro: false,
      hookText: false,
      killCounter: false,
      matchRecap: true,
      voiceComms: true,
      nativeHud: true,
      coverStrategy: 'no-cover',
      introText: '',
      outroText: '',
    },
    title,
    map: 'de_ancient',
    score: '13-8',
    targetName: 'Joey-',
    createdAt: 1,
  };
}

async function seedReels(page: Parameters<typeof gotoStudio>[0], intents: object[]) {
  await page.addInitScript((value) => {
    window.localStorage.setItem('cliphub.reels.v1', JSON.stringify(value));
  }, intents);
}

test.describe('Clips lens', () => {
  test('reuses a recorded demo through generate before editing a new Full POV', async ({ page }) => {
    await seedReels(page, [fullDemoIntent(RECORDING_JOB_ID, 'Partida completa · Joey-')]);
    let jobStatus = 'recorded';
    await page.route(`**/api/demos/${RECORDING_JOB_ID}/status`, (route) =>
      route.fulfill({ contentType: 'application/json', body: JSON.stringify({ status: jobStatus }) }),
    );
    await page.route(`**/api/demos/${RECORDING_JOB_ID}/renders/gameplay-pov-60`, (route) =>
      route.fulfill({ status: 404, contentType: 'application/json', body: '{}' }),
    );

    let generateBody: Record<string, unknown> | undefined;
    await page.route(`**/api/demos/${RECORDING_JOB_ID}/generate`, async (route) => {
      generateBody = route.request().postDataJSON() as Record<string, unknown>;
      jobStatus = 'recording';
      await route.fulfill({ status: 202, contentType: 'application/json', body: JSON.stringify({ task: 'record:demo' }) });
    });

    await gotoStudio(page, CLIPS_LENS_HREF);

    await expect.poll(() => generateBody).toBeDefined();
    expect(generateBody).toMatchObject({
      preset: 'gameplay-pov-60',
      segment_ids: [],
      edit: { format: 'landscape-16x9', match_recap: true, native_hud: true },
    });
    const clips = page.getByLabel('Clips', { exact: true });
    await expect(clips.getByText('REC', { exact: true })).toBeVisible();
    await expect(clips.getByText(/Vídeo largo/).first()).toBeVisible();
    await expect(clips.getByText('RENDER', { exact: true })).toHaveCount(0);
  });

  test('does not gate the MP4 behind a cover candidate picker', async ({ page }) => {
    await gotoStudio(page, CLIPS_LENS_HREF);
    await expect(page.getByText('Portada · elige candidata')).toHaveCount(0);
    await expect(page.getByText('Confirma una portada antes de marcar el pack listo para subir.')).toHaveCount(0);
    const mp4 = page.getByRole('button', { name: 'MP4' });
    if ((await mp4.count()) > 0) {
      await expect(mp4.first()).toBeEnabled();
    }
  });

  test('shows completed capture progress as the round counter', async ({ page }) => {
    await seedReels(page, [fullDemoIntent(RECORDING_JOB_ID, 'Partida completa · Joey-')]);
    await page.route(`**/api/demos/${RECORDING_JOB_ID}/status`, (route) =>
      route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'recording',
          progress: { done: 12, total: 12, percent: 100 },
        }),
      }),
    );

    await gotoStudio(page, CLIPS_LENS_HREF);

    await expect(page.getByLabel('Clips', { exact: true }).getByText('REC R12/12')).toBeVisible();
  });

  test('sanitizes stale POV failures and hides deterministic retry', async ({ page }) => {
    await seedReels(page, [fullDemoIntent(FAILED_JOB_ID, 'Partida completa · Joey-')]);
    await page.route(`**/api/demos/${FAILED_JOB_ID}/status`, (route) =>
      route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          status: 'failed',
          failure_reason:
            '[zackvideo] capture_failed: capture POV verification failed: observer target remained unknown during seg-012; check CS2 console log at C:\\Games\\CS2\\console.log',
        }),
      }),
    );
    await page.route(`**/api/demos/${FAILED_JOB_ID}/renders/gameplay-pov-60`, (route) =>
      route.fulfill({ status: 404, contentType: 'application/json', body: '{}' }),
    );

    await gotoStudio(page, CLIPS_LENS_HREF);

    const clips = page.getByLabel('Clips', { exact: true });
    await expect(clips.getByText('FALLÓ', { exact: true })).toBeVisible();
    await expect(clips.getByRole('button', { name: /Reintentar/i })).toHaveCount(0);
    await expect(page.getByText(/seg-012|console\.log|C:\\Games/i)).toHaveCount(0);
  });
});
