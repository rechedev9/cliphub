import type { Page } from '@playwright/test';
import { NAV_SECTIONS } from '../lib/nav.ts';

/** Every shell route, plus the standalone `/upload` entry, keyed by nav order. */
export const NAV_ROUTES = NAV_SECTIONS.map((section) => ({
  name: `${section.number} ${section.label}`,
  href: section.href,
}));

/** Widths the layout is contractually validated at. */
export const VALIDATION_WIDTHS = [390, 768, 1024, 1280, 1440, 1920] as const;

/** Parse tokens; the production minifier rewrites oklch/duration text. */
export type Oklch = { l: number; c: number; h: number; a: number };

const okl = (l: number, c: number, h: number, a = 1): Oklch => ({ l, c, h, a });

export const SURFACE_RAMP = [
  { token: '--surface-0', value: okl(0.128, 0.02, 264) },
  { token: '--surface-1', value: okl(0.152, 0.024, 264) },
  { token: '--surface-2', value: okl(0.188, 0.028, 264) },
  { token: '--surface-3', value: okl(0.224, 0.032, 264) },
  { token: '--surface-4', value: okl(0.262, 0.036, 264) },
  { token: '--surface-5', value: okl(0.302, 0.038, 264) },
] as const;

export const FOREGROUND_RAMP = [
  { token: '--fg-1', value: okl(0.97, 0.012, 226) },
  { token: '--fg-2', value: okl(0.795, 0.03, 250) },
  { token: '--fg-3', value: okl(0.672, 0.028, 250) },
  { token: '--fg-4', value: okl(0.545, 0.024, 250) },
] as const;

export const BORDER_TOKENS = [
  { token: '--border-subtle', value: okl(0.42, 0.024, 264, 0.5) },
  { token: '--border', value: okl(0.52, 0.028, 264, 0.62) },
  { token: '--border-strong', value: okl(0.62, 0.035, 264, 0.85) },
  { token: '--border-accent', value: okl(0.811, 0.135, 207.6, 0.45) },
] as const;

export const SIGNAL_TOKENS = [
  { token: '--primary', value: okl(0.811, 0.135, 207.6) },
  { token: '--primary-foreground', value: okl(0.173, 0.027, 233.6) },
  { token: '--ring', value: okl(0.811, 0.135, 207.6) },
  { token: '--stream', value: okl(0.657, 0.241, 6.9) },
  { token: '--stream-text', value: okl(0.72, 0.22, 6.9) },
  { token: '--success', value: okl(0.82, 0.135, 166) },
  { token: '--warning', value: okl(0.82, 0.16, 78) },
  { token: '--destructive', value: okl(0.68, 0.205, 22) },
  { token: '--destructive-solid', value: okl(0.485, 0.195, 22) },
] as const;

/** Type scale steps, with 12px as the hard floor. */
export const TYPE_SCALE = [
  { step: 'text-meta', px: 12, lineHeight: 1.25, tracking: 0.14 },
  { step: 'text-label', px: 13, lineHeight: 1.3, tracking: 0.08 },
  { step: 'text-body-sm', px: 14, lineHeight: 1.45, tracking: undefined },
  { step: 'text-body', px: 15, lineHeight: 1.6, tracking: undefined },
  { step: 'text-body-lg', px: 17, lineHeight: 1.5, tracking: undefined },
  { step: 'text-title', px: 20, lineHeight: 1.2, tracking: -0.01 },
  { step: 'text-section', px: 24, lineHeight: 1.15, tracking: -0.015 },
  { step: 'text-display-sm', px: 30, lineHeight: 1.08, tracking: -0.02 },
  { step: 'text-display', px: 40, lineHeight: 1.02, tracking: -0.025 },
  { step: 'text-hero', px: 56, lineHeight: 0.98, tracking: -0.03 },
  { step: 'text-stat', px: 28, lineHeight: 1, tracking: 0.01 },
] as const;

/** Motion vocabulary; an ad-hoc duration is a contract break. */
export const DURATION_TOKENS = [
  { token: '--dur-instant', ms: 80 },
  { token: '--dur-fast', ms: 140 },
  { token: '--dur-base', ms: 220 },
  { token: '--dur-slow', ms: 380 },
  { token: '--dur-data', ms: 600 },
] as const;

/** Parses `oklch(0.128 0.02 264 / 0.5)` and its minified `oklch(12.8% .02 264/.5)`. */
export function parseOklch(raw: string): Oklch {
  const inner = raw.trim().replace(/^oklch\(/i, '').replace(/\)$/, '');
  const [components, alpha] = inner.split('/');
  const parts = components.trim().split(/\s+/);
  if (parts.length < 3) throw new Error(`not an oklch colour: ${raw}`);
  const lightness = parts[0].endsWith('%') ? Number.parseFloat(parts[0]) / 100 : Number.parseFloat(parts[0]);
  const parsedAlpha = alpha === undefined ? 1 : Number.parseFloat(alpha);
  return {
    l: lightness,
    c: Number.parseFloat(parts[1]),
    h: Number.parseFloat(parts[2]),
    a: alpha !== undefined && alpha.trim().endsWith('%') ? parsedAlpha / 100 : parsedAlpha,
  };
}

/** The leading number of a dimensioned value, minifier-independent. */
export function parseNumber(raw: string): number {
  const value = Number.parseFloat(raw.trim());
  if (Number.isNaN(value)) throw new Error(`not a numeric CSS value: ${raw}`);
  return value;
}

/** Duration in ms; the minifier may serve `.38s` instead of `380ms`. */
export function parseMs(raw: string): number {
  const value = parseNumber(raw);
  return /\dms$/i.test(raw.trim()) ? value : value * 1000;
}

/** Drops every space so a minified value compares against an authored one. */
export function squash(raw: string): string {
  return raw.replace(/\s+/g, '');
}

/** Reads a custom property off `:root` as the browser resolves it. */
export function rootToken(page: Page, token: string): Promise<string> {
  return page.evaluate(
    (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim(),
    token,
  );
}

/** Wait until the shell has painted. Do not use networkidle: Studio polls. */
export async function gotoStudio(page: Page, href: string): Promise<void> {
  await page.goto(href, { waitUntil: 'load' });
  await page.locator('body').waitFor({ state: 'visible' });
  // App Router stages streamed chunks twice; wait for two equal DOM sizes.
  await page.waitForFunction(
    () => {
      const probe = window as unknown as { __studioDomSize?: number };
      const size = document.getElementsByTagName('*').length;
      const settled = probe.__studioDomSize === size;
      probe.__studioDomSize = size;
      return settled;
    },
    undefined,
    { polling: 150 },
  );
  // Settled markup is still not an interactive page; wait for React to attach.
  await page.waitForFunction(() => Object.keys(document.body).some((key) => key.startsWith('__react')));
  await page.evaluate(() => document.fonts.ready);
}
