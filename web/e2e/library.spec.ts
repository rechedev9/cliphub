import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

const RECORDING_JOB_ID = '11111111-1111-4111-8111-111111111111';
const FAILED_JOB_ID = '22222222-2222-4222-8222-222222222222';

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

test.describe('Biblioteca', () => {
  test('does not gate the MP4 behind a cover candidate picker', async ({ page }) => {
    await gotoStudio(page, '/videos');
    await expect(page.getByText('Portada · elige candidata')).toHaveCount(0);
    await expect(page.getByText('Confirma una portada antes de marcar el pack listo para subir.')).toHaveCount(0);
    const mp4 = page.getByRole('button', { name: 'MP4' });
    if ((await mp4.count()) > 0) {
      await expect(mp4.first()).toBeEnabled();
    }
  });

  test('shows completed capture progress as local validation', async ({ page }) => {
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

    await gotoStudio(page, '/videos');

    await expect(page.getByText('Validando captura local')).toBeVisible();
    await expect(page.getByLabel('Vídeos').getByText('100%')).toBeVisible();
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

    await gotoStudio(page, '/videos');

    await expect(page.getByText(/ClipHub perdió el POV al terminar una ronda/)).toBeVisible();
    await expect(
      page.getByText('Elimina esta tarjeta y vuelve a preparar la demo para regenerar el plan de rondas.'),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: /Reintentar/i })).toHaveCount(0);
    await expect(page.getByText(/seg-012|console\.log|C:\\Games/i)).toHaveCount(0);
  });
});
