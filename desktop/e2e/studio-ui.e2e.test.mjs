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
import { createRequire } from 'node:module';
import { mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
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

test('boots to Inicio, not the error screen', async () => {
  await page.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/onboarding/, {
    timeout: E2E_BOOT_DEADLINE_MS,
  });
  const url = page.url();
  assert.match(url, /^http:\/\/127\.0\.0\.1:\d+\/onboarding/, `landed on ${url}`);
  // The document titles itself with the shared web product name, while the
  // native window must keep the desktop product name (main.ts suppresses
  // page-title-updated).
  assert.equal(await page.title(), 'Inicio · ClipHub');
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

test('renderer produced no uncaught exceptions', () => {
  assert.deepEqual(pageErrors, []);
});

test('renderer console has no errors', () => {
  // Report the exact messages on failure so regressions are diagnosable from
  // CI output alone.
  assert.deepEqual(consoleErrors, []);
});
