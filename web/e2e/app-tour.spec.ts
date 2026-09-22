import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';
import { TOUR_CHAPTERS, TOUR_TITLE } from '../lib/app-tour.ts';

/** Studio preload stand-in; `acknowledged` decides whether the telemetry notice shows first. */
async function installSettingsBridge(page: Page, acknowledged: boolean): Promise<void> {
  await page.addInitScript((noticeAcknowledged: boolean) => {
    let status = { available: true, enabled: true, noticeAcknowledged, supportCode: 'CH-TEST-0001' };
    Object.defineProperty(window, 'cliphubSettings', {
      value: {
        getAppInfo: async () => ({ version: '3.0.1' }),
        getTelemetry: async () => status,
        updateTelemetry: async (enabled: boolean) => {
          status = { ...status, enabled, noticeAcknowledged: true };
          return status;
        },
      },
    });
  }, acknowledged);
}

test.describe('Studio tour', () => {
  test('stays closed in a plain browser and opens from the command strip', async ({ page }) => {
    await gotoStudio(page, '/clips');
    await expect(page.getByTestId('app-tour')).toHaveCount(0);

    await page.getByTestId('app-tour-trigger').click();
    const tour = page.getByRole('dialog', { name: TOUR_TITLE });
    await expect(tour).toBeVisible();
    await expect(tour.getByRole('heading', { name: TOUR_CHAPTERS[0].title })).toBeVisible();
    await expect(tour.getByText(`01 / ${String(TOUR_CHAPTERS.length).padStart(2, '0')}`)).toBeVisible();
  });

  test('walks every chapter in order and closes on the last one', async ({ page }) => {
    await gotoStudio(page, '/clips');
    await page.getByTestId('app-tour-trigger').click();
    const tour = page.getByRole('dialog', { name: TOUR_TITLE });

    for (const [i, chapter] of TOUR_CHAPTERS.entries()) {
      await expect(tour.getByRole('heading', { name: chapter.title })).toBeVisible();
      if (i < TOUR_CHAPTERS.length - 1) await tour.getByRole('button', { name: 'Siguiente' }).click();
    }
    await tour.getByRole('button', { name: 'Empezar' }).click();
    await expect(tour).toHaveCount(0);
  });

  test('the chapter rail jumps straight to a section and its link navigates there', async ({ page }) => {
    await gotoStudio(page, '/clips');
    await page.getByTestId('app-tour-trigger').click();
    const tour = page.getByRole('dialog', { name: TOUR_TITLE });
    const tactical = TOUR_CHAPTERS.find((chapter) => chapter.id === 'tactical');
    if (tactical?.link === undefined) throw new Error('tactical chapter lost its link');

    await tour.getByRole('navigation', { name: 'Capítulos de la guía' }).getByRole('button', { name: tactical.label }).click();
    await expect(tour.getByRole('heading', { name: tactical.title })).toBeVisible();
    await tour.getByRole('link', { name: tactical.link.label }).click();
    await expect(page).toHaveURL(/\/tactical$/);
    await expect(tour).toHaveCount(0);
  });

  test('opens by itself once inside Studio and never again after it is closed', async ({ page }) => {
    await installSettingsBridge(page, true);
    await gotoStudio(page, '/clips');
    const tour = page.getByRole('dialog', { name: TOUR_TITLE });
    await expect(tour).toBeVisible();
    await tour.getByRole('button', { name: 'Saltar guía' }).click();
    await expect(tour).toHaveCount(0);

    await gotoStudio(page, '/clips');
    await expect(page.getByTestId('app-tour-trigger')).toBeVisible();
    await expect(page.getByTestId('app-tour')).toHaveCount(0);
  });

  test('waits for the telemetry notice instead of stacking on top of it', async ({ page }) => {
    await installSettingsBridge(page, false);
    await gotoStudio(page, '/clips');
    const notice = page.getByRole('dialog', { name: 'Ayuda a detectar fallos de ClipHub' });
    await expect(notice).toBeVisible();
    await expect(page.getByTestId('app-tour')).toHaveCount(0);

    await notice.getByRole('button', { name: 'Mantener activado' }).click();
    await expect(notice).toHaveCount(0);
    await expect(page.getByRole('dialog', { name: TOUR_TITLE })).toBeVisible();
  });

  test('fits a 390px window without horizontal overflow', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await gotoStudio(page, '/clips');
    await page.getByTestId('app-tour-trigger').click();
    const tour = page.getByRole('dialog', { name: TOUR_TITLE });
    await expect(tour).toBeVisible();
    await expect(tour.getByRole('button', { name: 'Siguiente' })).toBeInViewport();
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow, 'tour overflow at 390px').toBe(0);
  });
});
