import { expect, test } from '@playwright/test';
import { gotoStudio, parseMs, parseNumber, rootToken, squash } from './contract.ts';

/** design.md:106 — every depth effect must collapse with `--shell-depth`. */
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
    await gotoStudio(page, '/matches');
    expect(parseNumber(await rootToken(page, '--shell-depth'))).toBe(1);
  });

  for (const { name, attributes } of HTML_GATES) {
    test(`collapses to 0 under ${name}`, async ({ page }) => {
      await gotoStudio(page, '/matches');
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
    await gotoStudio(page, '/matches');
    expect(parseNumber(await rootToken(page, '--shell-depth'))).toBe(0);
  });

  test('collapses to 0 under forced-colors', async ({ page }) => {
    await page.emulateMedia({ forcedColors: 'active' });
    await gotoStudio(page, '/matches');
    expect(parseNumber(await rootToken(page, '--shell-depth'))).toBe(0);
  });

  test('derived effects follow the scalar rather than their own media query', async ({ page }) => {
    await gotoStudio(page, '/matches');
    // getPropertyValue substitutes --shell-depth into the calc, which is the
    // point: the derived value has to move with the scalar, not sit beside it.
    expect(squash(await rootToken(page, '--tilt-max'))).toBe('calc(1*6deg)');
    expect(squash(await rootToken(page, '--sheen-opacity'))).toBe('calc(1*.1)');

    const active = await page.evaluate(() => {
      const probe = document.createElement('div');
      probe.style.setProperty('rotate', 'var(--tilt-max)');
      document.body.append(probe);
      const value = getComputedStyle(probe).rotate;
      probe.remove();
      return value;
    });
    expect(active).toBe('6deg');

    await page.evaluate(() => document.documentElement.setAttribute('data-capture-active', 'true'));
    expect(squash(await rootToken(page, '--tilt-max'))).toBe('calc(0*6deg)');
    expect(squash(await rootToken(page, '--sheen-opacity'))).toBe('calc(0*.1)');

    const captured = await page.evaluate(() => {
      const probe = document.createElement('div');
      probe.style.setProperty('rotate', 'var(--tilt-max)');
      document.body.append(probe);
      const value = getComputedStyle(probe).rotate;
      probe.remove();
      return value;
    });
    expect(captured).toBe('0deg');
  });
});

test.describe('reduced motion', () => {
  test('keeps the spinner alive because a frozen one reads as a hung app', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await gotoStudio(page, '/matches');

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
    await gotoStudio(page, '/matches');
    for (const token of ['--dur-instant', '--dur-fast', '--dur-base', '--dur-slow', '--dur-data']) {
      expect(parseMs(await rootToken(page, token)), `${token} still animates`).toBe(1);
    }
  });
});
