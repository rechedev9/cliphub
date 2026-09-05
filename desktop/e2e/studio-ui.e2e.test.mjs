// ClipHub Studio UI e2e: launches the real Electron app (dev layout, same
// build-resources the installer bundles), waits for the full boot sequence
// (orchestrator + Next server + window navigation), and exercises the shell UI
// through Playwright's Electron driver.
//
// Prerequisites: `pnpm run build` (dist/main.js) and `pnpm run assemble`
// (build-resources/). The app allocates its own loopback ports, and the
// isolated-userdata.cjs bootstrap gives the suite its own userData (and thus
// its own single-instance lock), so it runs even while a real ClipHub
// Studio instance is open.
//
// Run: pnpm run test:e2e:ui

import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import { mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { after, before, test } from 'node:test';
import { E2E_BOOT_DEADLINE_MS } from '../scripts/e2e-boot-budget.mjs';
import { createE2EProfile } from '../scripts/e2e-profile.mjs';

const require = createRequire(import.meta.url);
const { _electron } = require('playwright-core');

const desktopRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const artifactsDir = join(desktopRoot, 'e2e', 'artifacts');
// Bootstrap that isolates userData so the app's single-instance lock does not
// collide with a running Studio (see isolated-userdata.cjs).
const bootstrapPath = join(desktopRoot, 'e2e', 'isolated-userdata.cjs');

// First boot provisions runtime tools (HLAE unpack, ffmpeg download) into
// userData, which can take minutes; a warm profile boots in seconds. The
// deadline covers the cold case without hanging forever on a real failure.
/** @type {import('playwright-core').ElectronApplication} */
let app;
/** @type {import('playwright-core').Page} */
let page;
const pageErrors = [];
const consoleErrors = [];
const profile = createE2EProfile('studio-ui');
const playbackFixtureInput = process.env.CLIPHUB_PLAYBACK_TEST_MP4 ?? '';
const playbackFixture = playbackFixtureInput === '' ? '' : resolve(playbackFixtureInput);
const playbackFixtureSHA256 = '878b98f746a01ce7058868eb06041c25c94b03d423221d67346b299e7a560d55';

async function pollValue(description, read, accept, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let latest;
  while (Date.now() < deadline) {
    latest = await read();
    if (accept(latest)) return latest;
    await new Promise((resolvePoll) => setTimeout(resolvePoll, 400));
  }
  assert.fail(`${description} timed out; latest=${JSON.stringify(latest)}`);
}

before(async () => {
  mkdirSync(artifactsDir, { recursive: true });
  app = await _electron.launch({
    executablePath: require('electron'),
    args: [bootstrapPath],
    cwd: desktopRoot,
    env: profile.environment(),
  });
  page = await app.firstWindow();
  page.on('pageerror', (err) => pageErrors.push(String(err)));
  page.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });
});

after(async () => {
  if (page && !page.isClosed()) {
    await page.screenshot({ path: join(artifactsDir, 'final-state.png') }).catch(() => {});
  }
  await app?.close().catch(() => {});
  profile.dispose();
});

test('boots to the clips hub, not the error screen', async () => {
  await page.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/clips(?:\?.*)?$/, {
    timeout: E2E_BOOT_DEADLINE_MS,
  });
  const url = page.url();
  assert.match(url, /^http:\/\/127\.0\.0\.1:\d+\/clips(?:\?.*)?$/, `landed on ${url}`);
  // The document titles itself with the shared web product name, while the
  // native window must keep the desktop product name (main.ts suppresses
  // page-title-updated).
  assert.equal(await page.title(), 'Clips y vídeos · ClipHub');
  const nativeTitle = await app.evaluate(({ BrowserWindow }) => {
    const win = BrowserWindow.getAllWindows()[0];
    return win ? win.getTitle() : null;
  });
  assert.equal(nativeTitle, 'ClipHub Studio');
  await page.screenshot({ path: join(artifactsDir, 'matches.png') });
});

test('renders real shell content', async () => {
  // The error screen is a data: URL; reaching here means we are on the web
  // origin. Still assert the page painted something meaningful.
  await page.waitForLoadState('domcontentloaded');
  const text = await page.evaluate(() => document.body.innerText);
  assert.ok(text.trim().length > 0, 'body rendered no text');
  assert.ok(
    !text.includes('no pudo arrancar'),
    'window shows the boot error screen',
  );
});

test('suspends visual work when unfocused and survives native minimization', async () => {
  try {
    await page.evaluate(() => {
      window.dispatchEvent(new Event('blur'));
    });
    await page.waitForFunction(() => document.documentElement.dataset.windowActivity === 'inactive');

    await page.evaluate(() => {
      window.dispatchEvent(new Event('focus'));
    });
    await page.waitForFunction(() => document.documentElement.dataset.windowActivity === 'active');

    await app.evaluate(({ BrowserWindow }) => {
      BrowserWindow.getAllWindows()[0]?.minimize();
    });
    const minimized = await app.evaluate(({ BrowserWindow }) => {
      return BrowserWindow.getAllWindows()[0]?.isMinimized() ?? false;
    });
    assert.equal(minimized, true);
  } finally {
    await app.evaluate(({ BrowserWindow }) => {
      const win = BrowserWindow.getAllWindows()[0];
      win?.restore();
      win?.show();
      win?.focus();
    });
  }
  await page.waitForFunction(() => document.documentElement.dataset.windowActivity === 'active');
});

test('web -> orchestrator proxy answers from inside the app', async () => {
  const status = await page.evaluate(async () => {
    const res = await fetch('/api/demos/jobs');
    return res.status;
  });
  assert.equal(status, 200);
});

test('clipboard writes require a focused, user-activated Studio action', async () => {
  const originalClipboard = await app.evaluate(({ clipboard }) => clipboard.readText());
  const marker = `cliphub-e2e-${Date.now()}`;
  try {
    // Chromium's transient activation can survive the preceding focus/restore
    // test for roughly five seconds. Let that standard window expire before
    // proving the bridge rejects a passive renderer call.
    await new Promise((resolve) => setTimeout(resolve, 5_500));
    const passiveBridgeWrite = await app.evaluate(async ({ BrowserWindow }) => {
      const win = BrowserWindow.getAllWindows()[0];
      if (!win) return true;
      return win.webContents.executeJavaScript(
        `window.cliphubClipboard.writeText('passive-bridge-write-must-fail').then(() => true, () => false)`,
        false,
      );
    });
    assert.equal(passiveBridgeWrite, false, 'clipboard bridge ignored user activation');

    const passiveNavigatorWrite = await page.evaluate(async () => {
      try {
        await navigator.clipboard.writeText('passive-write-must-fail');
        return true;
      } catch {
        return false;
      }
    });
    assert.equal(passiveNavigatorWrite, false, 'native clipboard permission was left open');

    await page.evaluate((value) => {
      const button = document.createElement('button');
      button.id = 'e2e-copy-button';
      button.type = 'button';
      button.textContent = 'Copy QA marker';
      button.style.position = 'fixed';
      button.style.top = '16px';
      button.style.right = '16px';
      button.style.zIndex = '2147483647';
      button.addEventListener('click', () => {
        void window.cliphubClipboard.writeText(value).then(
          () => { document.documentElement.dataset.e2eClipboard = 'written'; },
          () => { document.documentElement.dataset.e2eClipboard = 'denied'; },
        );
      });
      document.body.append(button);
    }, marker);
    await page.locator('#e2e-copy-button').click();
    await page.waitForFunction(() => document.documentElement.dataset.e2eClipboard !== undefined);
    assert.equal(
      await page.evaluate(() => document.documentElement.dataset.e2eClipboard),
      'written',
    );
    assert.equal(await app.evaluate(({ clipboard }) => clipboard.readText()), marker);
  } finally {
    await app.evaluate(({ clipboard }, value) => clipboard.writeText(value), originalClipboard);
  }
});

test('real Electron playback imports, previews, renders and plays one immutable Short', {
  skip: playbackFixture === '' ? 'set CLIPHUB_PLAYBACK_TEST_MP4 to the approved local MP4 fixture' : false,
  timeout: 240_000,
}, async () => {
  const fixtureInfo = statSync(playbackFixture);
  assert.equal(fixtureInfo.isFile(), true);
  assert.ok(fixtureInfo.size > 0, 'playback fixture is empty');
  assert.equal(
    createHash('sha256').update(readFileSync(playbackFixture)).digest('hex'),
    playbackFixtureSHA256,
    'playback fixture changed',
  );

  const origin = new URL(page.url()).origin;
  const mediaResponses = [];
  const recordMediaResponse = (response) => {
    const url = response.url();
    if (url.includes('/api/streams/') && (url.includes('/source') || url.includes('/revisions/'))) {
      mediaResponses.push({ status: response.status(), url });
    }
  };
  page.on('response', recordMediaResponse);
  try {
    await page.goto(`${origin}/streams`);
    await page.getByLabel('Nombre del proyecto', { exact: true }).fill('Electron playback E2E');
    await page.locator('input[type="file"][accept*="video/mp4"]').setInputFiles(playbackFixture);
    await page.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/streams\/[0-9a-f-]+$/i, { timeout: 30_000 });
    const jobId = new URL(page.url()).pathname.split('/').at(-1);
    assert.match(jobId ?? '', /^[0-9a-f]{8}-[0-9a-f-]{27}$/i);

    await page.getByLabel('Fin (s)').fill('2');
    await page.getByRole('button', { name: 'Añadir este momento', exact: true }).click();
    await page.getByLabel('Título del corte 1').fill('Electron playback Short');

    const decoder = page.locator('video[data-stream-frame="shared-decoder"]');
    await decoder.waitFor({ state: 'attached', timeout: 30_000 });
    assert.equal(await decoder.count(), 1, 'stream editor created more than one decoder');
    assert.equal(await page.locator('audio').count(), 0, 'stream editor created a duplicate audio element');
    await page.getByRole('button', { name: 'Vídeo original', exact: true }).click();
    const sourceStart = await decoder.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected shared video decoder');
      return element.currentTime;
    });
    await page.getByRole('button', { name: 'Reproducir vídeo original', exact: true }).click();
    await page.waitForFunction((start) => {
      const element = document.querySelector('video[data-stream-frame="shared-decoder"]');
      return element instanceof HTMLVideoElement && element.currentTime > start + 0.5;
    }, sourceStart, { timeout: 15_000 });
    assert.ok(mediaResponses.some((entry) => entry.status === 206 && entry.url.endsWith('/source')), 'source playback did not use Range HTTP');
    const sourceMetrics = await decoder.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected shared video decoder');
      const quality = element.getVideoPlaybackQuality();
      return {
        currentTime: element.currentTime,
        decodedFrames: quality.totalVideoFrames,
        droppedFrames: quality.droppedVideoFrames,
      };
    });
    await page.screenshot({ path: join(artifactsDir, 'electron-playback-editor.png') });

    const playbackInfo = await pollValue(
      'native playback diagnostics',
      () => page.evaluate(async () => {
        const bridge = window.cliphubSettings;
        if (!bridge || typeof bridge.getPlaybackInfo !== 'function') throw new Error('playback IPC bridge is unavailable');
        return bridge.getPlaybackInfo();
      }),
      (info) => info?.state === 'ready',
      15_000,
    );
    assert.equal(playbackInfo.available, true);
    assert.equal(playbackInfo.state, 'ready');
    assert.equal(playbackInfo.scope, 'global');
    assert.equal(typeof playbackInfo.chromiumVersion, 'string');

    await page.evaluate(() => window.dispatchEvent(new Event('blur')));
    await page.waitForFunction(() => document.documentElement.dataset.windowActivity === 'inactive');
    await page.waitForFunction(() => {
      const element = document.querySelector('video[data-stream-frame="shared-decoder"]');
      return element instanceof HTMLVideoElement && element.paused;
    });
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await page.waitForFunction(() => document.documentElement.dataset.windowActivity === 'active');

    await page.getByRole('button', { name: 'Continuar al aspecto →', exact: true }).click();
    await page.getByRole('button', { name: /Solo juego/ }).click();
    await page.getByRole('button', { name: 'Revisar Shorts →', exact: true }).click();
    await page.getByRole('button', { name: 'Exportar 1 Short →', exact: true }).click();
    await pollValue(
      'one-clip stream render',
      async () => {
        const state = await page.evaluate(async ({ id }) => {
          const response = await fetch(`/api/streams/${id}/renders/streamer-fullframe-nocam`, { cache: 'no-store' });
          return response.ok ? response.json() : { status: `http-${response.status}` };
        }, { id: jobId });
        if (state?.status === 'failed') throw new Error(`stream render failed: ${state.error ?? 'unknown error'}`);
        return state;
      },
      (state) => state?.status === 'rendered' && Array.isArray(state.videos) && state.videos.length === 1,
      180_000,
    );
    const catalogJob = await pollValue(
      'published stream catalog output',
      () => page.evaluate(async ({ id }) => {
        const response = await fetch('/api/streams', { cache: 'no-store' });
        if (!response.ok) throw new Error(`stream catalog failed (${response.status})`);
        const body = await response.json();
        const jobs = Array.isArray(body) ? body : body?.jobs;
        return Array.isArray(jobs) ? jobs.find((candidate) => candidate?.id === id) : undefined;
      }, { id: jobId }),
      (job) => Array.isArray(job?.rendered_outputs) && job.rendered_outputs.length === 1,
      30_000,
    );
    assert.equal(catalogJob?.rendered_outputs?.length, 1, 'real proxy dropped the published output catalog');
    assert.equal(catalogJob.rendered_outputs[0]?.render_status, 'rendered');
    assert.equal(catalogJob.rendered_outputs[0]?.aspect_ratio, '9:16');
    assert.match(catalogJob.rendered_outputs[0]?.video_url ?? '', /\/revisions\/[0-9a-f-]+\/videos\/[^/?]+$/i);
    writeFileSync(join(artifactsDir, 'electron-playback-catalog.json'), JSON.stringify(catalogJob, null, 2));

    await page.goto(`${origin}/clips?vista=clips`);
    await page.getByRole('button', { name: 'Reproducir Electron playback Short' }).first().click();
    const dialog = page.getByRole('dialog');
    const libraryVideo = dialog.locator('video');
    await libraryVideo.waitFor({ state: 'visible' });
    assert.equal(await dialog.locator('video').count(), 1, 'Library created more than one video element');
    assert.equal(await dialog.locator('audio').count(), 0, 'Library created a duplicate audio element');
    const libraryStart = await libraryVideo.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected Library video');
      return element.currentTime;
    });
    await dialog.getByRole('button', { name: 'Reproducir', exact: true }).click();
    await page.waitForFunction((start) => {
      const element = document.querySelector('[role="dialog"] video');
      return element instanceof HTMLVideoElement && element.currentTime > start + 0.5;
    }, libraryStart, { timeout: 15_000 });
    assert.ok(mediaResponses.some((entry) => entry.status === 206 && entry.url.includes('/revisions/')), 'Library playback did not use immutable Range HTTP');
    const libraryMetrics = await libraryVideo.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected Library video');
      const quality = element.getVideoPlaybackQuality();
      return {
        currentTime: element.currentTime,
        decodedFrames: quality.totalVideoFrames,
        droppedFrames: quality.droppedVideoFrames,
      };
    });

    await dialog.getByRole('button', { name: 'Pantalla completa' }).click();
    await page.waitForFunction(() => document.fullscreenElement !== null);
    await page.keyboard.press('Escape');
    await page.waitForFunction(() => document.fullscreenElement === null);
    assert.equal(await dialog.isVisible(), true, 'Escape closed the player while exiting native fullscreen');

    const playingAfterFullscreen = await libraryVideo.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected Library video');
      return !element.paused;
    });
    if (playingAfterFullscreen) await dialog.getByRole('button', { name: 'Pausar', exact: true }).click();
    await dialog.getByRole('button', { name: 'Reproducir', exact: true }).click();
    await page.waitForFunction(() => {
      const element = document.querySelector('[role="dialog"] video');
      return element instanceof HTMLVideoElement && !element.paused;
    });
    await page.evaluate(() => window.dispatchEvent(new Event('blur')));
    await page.waitForFunction(() => {
      const element = document.querySelector('[role="dialog"] video');
      return document.documentElement.dataset.windowActivity === 'inactive'
        && element instanceof HTMLVideoElement && element.paused;
    });
    await page.screenshot({ path: join(artifactsDir, 'electron-playback-library.png') });
    writeFileSync(join(artifactsDir, 'electron-playback-metrics.json'), JSON.stringify({
      fixture: { bytes: fixtureInfo.size, sha256: playbackFixtureSHA256 },
      source: sourceMetrics,
      library: libraryMetrics,
      playbackInfo,
      mediaResponses: mediaResponses.map((entry) => ({ status: entry.status, path: new URL(entry.url).pathname })),
    }, null, 2));
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  } finally {
    page.off('response', recordMediaResponse);
  }
});

test('renderer produced no uncaught exceptions', () => {
  assert.deepEqual(pageErrors, []);
});

test('renderer console has no errors', () => {
  // Report the exact messages on failure so regressions are diagnosable from
  // CI output alone.
  assert.deepEqual(consoleErrors, []);
});
