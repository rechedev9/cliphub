import { readFile, stat, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { expect, test, type Locator, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';

const fixtureInput = process.env.CLIPHUB_PLAYBACK_TEST_MP4 ?? '';
const fixturePath = fixtureInput === '' ? '' : resolve(fixtureInput);
const JOB_ID = '79f62593-bd70-4cea-883f-16885f70d1ac';
const VARIANT = 'streamer-fullframe-nocam';

function json(body: unknown, status = 200): { status: number; contentType: string; body: string } {
  return { status, contentType: 'application/json', body: JSON.stringify(body) };
}

async function requireFixture(): Promise<void> {
  const info = await stat(fixturePath);
  expect(info.isFile()).toBe(true);
  expect(info.size).toBeGreaterThan(0);
}

async function fulfillMP4(page: Page, pattern: string): Promise<void> {
  const bytes = await readFile(fixturePath);
  await page.route(pattern, (route) => {
    const range = route.request().headers().range;
    const match = range?.match(/^bytes=(\d+)-(\d*)$/);
    if (!match) {
      return route.fulfill({
        status: 200,
        contentType: 'video/mp4',
        headers: { 'Accept-Ranges': 'bytes', 'Content-Length': String(bytes.length) },
        body: bytes,
      });
    }
    const start = Number(match[1]);
    const requestedEnd = match[2] === '' ? bytes.length - 1 : Number(match[2]);
    if (!Number.isSafeInteger(start) || !Number.isSafeInteger(requestedEnd) || start < 0 || start >= bytes.length || requestedEnd < start) {
      return route.fulfill({ status: 416, headers: { 'Content-Range': `bytes */${bytes.length}` } });
    }
    const end = Math.min(requestedEnd, bytes.length - 1);
    const body = bytes.subarray(start, end + 1);
    return route.fulfill({
      status: 206,
      contentType: 'video/mp4',
      headers: {
        'Accept-Ranges': 'bytes',
        'Content-Length': String(body.length),
        'Content-Range': `bytes ${start}-${end}/${bytes.length}`,
      },
      body,
    });
  });
}

function videoTime(video: Locator): Promise<number> {
  return video.evaluate((element) => {
    if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
    return element.currentTime;
  });
}

function videoDuration(video: Locator): Promise<number> {
  return video.evaluate((element) => {
    if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
    return element.duration;
  });
}

function decodedFrames(video: Locator): Promise<number> {
  return video.evaluate((element) => {
    if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
    return element.getVideoPlaybackQuality().totalVideoFrames;
  });
}

async function stubRenderedLibrary(page: Page): Promise<void> {
  await page.route('**/api/demos/jobs', (route) => route.fulfill(json({ jobs: [] })));
  await page.route('**/api/streams', (route) => {
    const url = new URL(route.request().url());
    if (url.pathname !== '/api/streams') return route.fallback();
    return route.fulfill(json({
      jobs: [{
        id: JOB_ID,
        status: 'rendered',
        title: 'Prueba de reproducción',
        created_at: '2026-09-05T10:00:00Z',
        rendered_outputs: [
          {
            artifact_revision: 'revision-a', variant: VARIANT, clip_id: 'clip-a', artifact_name: 'clip-a.mp4',
            title: 'Clip real A', format: 'video/mp4', aspect_ratio: '9:16', render_status: 'rendered', stale: false,
            review_required: false,
            video_url: `/api/streams/${JOB_ID}/renders/${VARIANT}/revisions/revision-a/videos/clip-a`,
          },
          {
            artifact_revision: 'revision-b', variant: VARIANT, clip_id: 'clip-b', artifact_name: 'clip-b.mp4',
            title: 'Clip real B', format: 'video/mp4', aspect_ratio: '9:16', render_status: 'rendered', stale: true,
            review_required: true, warnings: ['Revisión real de prueba'],
            video_url: `/api/streams/${JOB_ID}/renders/${VARIANT}/revisions/revision-b/videos/clip-b`,
          },
        ],
      }],
    }));
  });
  await fulfillMP4(page, `**/api/streams/${JOB_ID}/renders/${VARIANT}/revisions/*/videos/*`);
}

async function stubStreamEditor(page: Page): Promise<void> {
  const plan = {
    schema_version: '1.1',
    variant: VARIANT,
    face_crop_reviewed: true,
    gameplay_crop: { x: 0, y: 0, width: 1, height: 1 },
    clips: [
      { id: 'clip-a', start_seconds: 0, end_seconds: 1.5, title: 'Primero' },
      { id: 'clip-b', start_seconds: 1.5, end_seconds: 3, title: 'Segundo' },
    ],
    updated_at: '2026-09-05T10:00:00Z',
  };
  await page.route(`**/api/streams/${JOB_ID}`, (route) => route.fulfill(json({
    id: JOB_ID,
    status: 'ready',
    title: 'Fuente real local',
    probe: { width: 1920, height: 1080, duration_seconds: 3 },
    edit_plan: plan,
    created_at: '2026-09-05T10:00:00Z',
  })));
  await page.route(`**/api/streams/${JOB_ID}/edit-plan`, (route) => route.fulfill(json(plan)));
  await page.route(`**/api/streams/${JOB_ID}/renders/**`, (route) => route.fulfill(json({ error: 'no render' }, 404)));
  await page.route('**/api/songs', (route) => route.fulfill(json({ songs: [] })));
  await fulfillMP4(page, `**/api/streams/${JOB_ID}/source`);
}

test.describe('real Chromium media playback', () => {
  test.skip(fixturePath === '', 'set CLIPHUB_PLAYBACK_TEST_MP4 to an existing local MP4');
  test.describe.configure({ mode: 'serial' });
  test.use({ viewport: { width: 1920, height: 1080 } });

  test.beforeAll(async () => requireFixture());

  test('library plays continuously, scrubs, resumes and keeps revision-scoped state', async ({ page }, testInfo) => {
    await stubRenderedLibrary(page);
    await gotoStudio(page, '/clips?vista=clips');

    await page.getByRole('button', { name: 'Reproducir Clip real A' }).first().click();
    const dialog = page.getByRole('dialog');
    const video = dialog.locator('video');
    await expect(dialog.getByRole('heading', { name: 'Clip real A' })).toBeVisible();
    await expect(video).toHaveCount(1);
    await expect(dialog.getByRole('heading', { name: 'Clip real A' })).toBeInViewport();
    await expect(dialog.getByRole('button', { name: 'Reproducir', exact: true })).toBeInViewport();
    await video.evaluate(async (element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
      if (element.readyState < HTMLMediaElement.HAVE_METADATA) {
        await new Promise<void>((done) => element.addEventListener('loadedmetadata', () => done(), { once: true }));
      }
      if (!Number.isFinite(element.duration) || element.duration < 3) throw new Error('fixture MP4 must be at least 3 seconds');
      element.dataset.seekEvents = '0';
      element.addEventListener('seeking', () => {
        element.dataset.seekEvents = String(Number(element.dataset.seekEvents ?? '0') + 1);
      });
    });

    await dialog.getByRole('button', { name: 'Reproducir', exact: true }).click();
    await expect(dialog.getByRole('heading', { name: 'Clip real A' })).toBeInViewport();
    await expect(dialog.getByRole('button', { name: 'Pausar', exact: true })).toBeInViewport();
    const startedAt = await videoTime(video);
    await expect.poll(() => videoTime(video)).toBeGreaterThan(startedAt + 0.75);
    await expect.poll(() => decodedFrames(video)).toBeGreaterThan(0);
    expect(await video.evaluate((element) => Number(element.dataset.seekEvents ?? '0'))).toBe(0);
    const libraryScreenshot = testInfo.outputPath('library-modal.png');
    await page.screenshot({ path: libraryScreenshot, fullPage: true });

    await dialog.getByRole('button', { name: 'Pantalla completa' }).click();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement !== null)).toBe(true);
    await expect(dialog.getByRole('button', { name: 'Salir de pantalla completa' })).toBeInViewport();
    await page.keyboard.press('Escape');
    await expect.poll(() => page.evaluate(() => document.fullscreenElement === null)).toBe(true);
    await expect(dialog).toBeVisible();

    await dialog.getByRole('button', { name: 'Pausar', exact: true }).click();
    const duration = await videoDuration(video);
    const position = dialog.locator('#media-player-position');
    const step = Number(await position.getAttribute('step'));
    expect(step).toBeGreaterThan(0);
    // Any local MP4 is accepted; align fractional durations to the range's step.
    const scrubbed = Math.floor(Math.min(duration - 1, duration * 0.55) / step) * step;
    await position.fill(String(Number(scrubbed.toFixed(6))));
    await expect.poll(() => videoTime(video)).toBeGreaterThan(scrubbed - 0.3);
    const quality = await video.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
      const result = element.getVideoPlaybackQuality();
      return {
        currentTime: element.currentTime,
        duration: element.duration,
        decodedFrames: result.totalVideoFrames,
        droppedFrames: result.droppedVideoFrames,
        seekEvents: Number(element.dataset.seekEvents ?? '0'),
      };
    });
    await dialog.getByRole('combobox', { name: 'Velocidad' }).selectOption('2');
    await dialog.getByRole('button', { name: 'Reproducir', exact: true }).focus();
    await page.keyboard.press('m');
    await expect(dialog.getByRole('button', { name: 'Activar sonido' })).toBeVisible();
    await page.keyboard.press('l');
    await expect(dialog.getByRole('button', { name: 'Repetir vídeo' })).toHaveAttribute('aria-pressed', 'true');
    await dialog.getByRole('button', { name: 'Cerrar' }).click();
    await expect(dialog).toBeHidden();
    const audibleVideos = await page.locator('video').evaluateAll((elements) => elements.filter((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
      return !element.paused;
    }).length);
    expect(audibleVideos).toBe(0);

    await page.getByRole('button', { name: 'Reproducir Clip real A' }).first().click();
    await expect.poll(() => videoTime(page.getByRole('dialog').locator('video'))).toBeGreaterThan(scrubbed - 0.5);
    await expect(page.getByRole('dialog').getByRole('combobox', { name: 'Velocidad' })).toHaveValue('2');
    await expect(page.getByRole('dialog').getByRole('button', { name: 'Activar sonido' })).toBeVisible();
    await expect(page.getByRole('dialog').getByRole('button', { name: 'Repetir vídeo' })).toHaveAttribute('aria-pressed', 'true');
    await page.getByRole('dialog').getByRole('button', { name: 'Vídeo siguiente' }).click();
    await expect(page.getByRole('dialog').getByRole('heading', { name: 'Clip real B' })).toBeVisible();
    await expect(page.getByRole('dialog').getByText(/^Render desactualizado ·/)).toBeVisible();
    const persisted = await page.evaluate(() => window.localStorage.getItem('cliphub.media-playback.v1') ?? '');
    expect(persisted).toContain('revision-a');
    expect(persisted).not.toContain('jobs/');
    await writeFile(testInfo.outputPath('library-metrics.json'), JSON.stringify(quality, null, 2));
  });

  test('editor modes share one decoder and advance canvas timestamps without seek polling', async ({ page }, testInfo) => {
    await stubStreamEditor(page);
    await gotoStudio(page, `/streams/${JOB_ID}`);
    const decoder = page.locator('video[data-stream-frame="shared-decoder"]');
    const canvas = page.locator('canvas[data-stream-frame-canvas]');
    await expect(decoder).toHaveCount(1);
    await expect(canvas.first()).toBeVisible();
    await decoder.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
      element.dataset.seekEvents = '0';
      element.addEventListener('seeking', () => {
        element.dataset.seekEvents = String(Number(element.dataset.seekEvents ?? '0') + 1);
      });
    });

    await page.getByRole('button', { name: 'Vídeo original', exact: true }).click();
    const originalScreenshot = testInfo.outputPath('editor-original.png');
    await page.screenshot({ path: originalScreenshot, fullPage: true });
    await page.getByRole('button', { name: 'Reproducir vídeo original', exact: true }).click();
    const firstFrame = Number(await canvas.first().getAttribute('data-frame-seconds'));
    await expect.poll(async () => Number(await canvas.first().getAttribute('data-frame-seconds'))).toBeGreaterThan(firstFrame + 0.5);
    expect(await decoder.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
      return Number(element.dataset.seekEvents ?? '0');
    })).toBeLessThanOrEqual(1);
    const visibleTimes = await canvas.evaluateAll((elements) => elements.flatMap((element) => {
      if (!(element instanceof HTMLCanvasElement) || element.getClientRects().length === 0) return [];
      return [Number(element.dataset.frameSeconds)];
    }));
    expect(visibleTimes.length).toBeGreaterThan(0);
    expect(visibleTimes.every((seconds) => Math.abs(seconds - visibleTimes[0]) < 0.001)).toBe(true);
    await page.getByRole('button', { name: 'Pausar', exact: true }).click();
    const pausedAt = await videoTime(decoder);
    await page.waitForTimeout(500);
    expect(Math.abs(await videoTime(decoder) - pausedAt)).toBeLessThan(0.05);

    await page.getByRole('button', { name: 'Short seleccionado', exact: true }).click();
    const shortScreenshot = testInfo.outputPath('editor-short.png');
    await page.screenshot({ path: shortScreenshot, fullPage: true });
    await page.getByRole('button', { name: 'Todos los Shorts', exact: true }).click();
    await page.getByRole('slider', { name: 'Posición del vídeo original' }).fill('0');
    await page.getByRole('button', { name: 'Reproducir todos los Shorts', exact: true }).click();
    await expect.poll(() => videoTime(decoder), { timeout: 5_000 }).toBeGreaterThan(1.6);
    await expect.poll(() => videoTime(decoder), { timeout: 5_000 }).toBeGreaterThan(2.8);
    await expect(page.getByRole('button', { name: 'Reproducir todos los Shorts', exact: true })).toBeVisible();
    await expect(decoder).toHaveCount(1);
    await expect(canvas.first()).toHaveAttribute('data-frame-seconds', /\d/);
    const metrics = await decoder.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected a video element');
      const quality = element.getVideoPlaybackQuality();
      return {
        currentTime: element.currentTime,
        decodedFrames: quality.totalVideoFrames,
        droppedFrames: quality.droppedVideoFrames,
        seekEvents: Number(element.dataset.seekEvents ?? '0'),
      };
    });
    await writeFile(testInfo.outputPath('editor-metrics.json'), JSON.stringify(metrics, null, 2));
  });
});
