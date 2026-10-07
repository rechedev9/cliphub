import { expect, test } from '@playwright/test';
import { gotoStudio, parseMs, parseNumber, rootToken } from './contract.ts';

/** Every depth effect must collapse with `--shell-depth`. */
const HTML_GATES = [
  { name: 'an active capture', attributes: { 'data-capture-active': 'true' } },
  { name: 'an inactive window', attributes: { 'data-window-activity': 'inactive' } },
  {
    name: 'the efficiency profile on desktop',
    attributes: { 'data-runtime': 'desktop', 'data-performance-profile': 'efficiency' },
  },
] as const;

test.describe('depth scalar', () => {
  test('is 1 in the default room', async ({ page }) => {
    await gotoStudio(page, '/clips');
    expect(parseNumber(await rootToken(page, '--shell-depth'))).toBe(1);
  });

  for (const { name, attributes } of HTML_GATES) {
    test(`collapses to 0 under ${name}`, async ({ page }) => {
      await gotoStudio(page, '/clips');
      expect(parseNumber(await rootToken(page, '--shell-depth'))).toBe(1);

      await page.evaluate((pairs) => {
        for (const [key, value] of Object.entries(pairs)) {
          document.documentElement.setAttribute(key, value);
        }
      }, attributes as Record<string, string>);

      expect(parseNumber(await rootToken(page, '--shell-depth'))).toBe(0);
    });
  }

  test('collapses to 0 under prefers-reduced-motion', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await gotoStudio(page, '/clips');
    expect(parseNumber(await rootToken(page, '--shell-depth'))).toBe(0);
  });

  test('collapses to 0 under forced-colors', async ({ page }) => {
    await page.emulateMedia({ forcedColors: 'active' });
    await gotoStudio(page, '/clips');
    expect(parseNumber(await rootToken(page, '--shell-depth'))).toBe(0);
  });
});

test.describe('reduced motion', () => {
  test('keeps the spinner alive because a frozen one reads as a hung app', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await gotoStudio(page, '/clips');

    const duration = await page.evaluate(() => {
      const probe = document.createElement('div');
      probe.className = 'animate-spin';
      document.body.append(probe);
      const style = getComputedStyle(probe);
      const value = { duration: style.animationDuration, count: style.animationIterationCount };
      probe.remove();
      return value;
    });

    expect(duration.duration).toBe('1.8s');
    expect(duration.count).toBe('infinite');
  });

  test('flattens every other transition to 1ms', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await gotoStudio(page, '/clips');
    for (const token of ['--dur-instant', '--dur-fast', '--dur-base', '--dur-slow', '--dur-data']) {
      expect(parseMs(await rootToken(page, token)), `${token} still animates`).toBe(1);
    }
  });
});
