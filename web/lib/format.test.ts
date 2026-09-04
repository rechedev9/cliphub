// Unit tests for the visible-string formatters on the matches screens
// (Spanish NEON HUD skin). Run: node --test "lib/**/*.test.ts"
import test from 'node:test';
import assert from 'node:assert/strict';
import { timeAgo, matchDateLabel, playsSelectionLabel, formatKd, ratingClass, ratingBarClass, ratingBarPct, prettyMapName, formatShortDate } from './format.ts';
import type { Play } from './api/types.ts';

function play(overrides: Partial<Play>): Play {
  return {
    id: 'seg-001',
    matchId: 'job',
    label: '1K · Ronda 1',
    kind: 'highlight',
    round: 1,
    kills: 1,
    ...overrides,
  };
}

test('timeAgo: under a minute reads "ahora mismo"', () => {
  assert.equal(timeAgo(Date.now() - 5_000), 'ahora mismo');
});

test('timeAgo: minutes read "hace N min"', () => {
  assert.equal(timeAgo(Date.now() - 5 * 60_000), 'hace 5 min');
});

test('timeAgo: hours read "hace N h"', () => {
  assert.equal(timeAgo(Date.now() - 2 * 3_600_000), 'hace 2 h');
});

test('timeAgo: days read "hace N d"', () => {
  assert.equal(timeAgo(Date.now() - 3 * 86_400_000), 'hace 3 d');
});

test('uploaded demos show an import date instead of a fabricated recent play time', () => {
  const label = matchDateLabel({ playedAt: '2020-01-02T00:00:00Z', source: 'upload' });
  assert.match(label, /^importada el /);
  assert.doesNotMatch(label, /ahora mismo/);
});

test('playsSelectionLabel: empty selection is null', () => {
  assert.equal(playsSelectionLabel([]), null);
});

test('playsSelectionLabel: a single pick reuses its own label', () => {
  assert.equal(playsSelectionLabel([play({ label: '3K · Ronda 6', round: 6, kills: 3 })]), '3K · Ronda 6');
});

test('playsSelectionLabel: 2+ picks summarize count and sorted distinct rounds in Spanish', () => {
  const picks = [
    play({ id: 'a', label: '2K · Ronda 9', round: 9 }),
    play({ id: 'b', label: '1K · Ronda 1', round: 1 }),
    play({ id: 'c', label: '3K · Ronda 6', round: 6 }),
  ];
  assert.equal(playsSelectionLabel(picks), '3 jugadas · Rondas 1, 6, 9');
});

test('playsSelectionLabel: duplicate rounds collapse in the summary', () => {
  const picks = [
    play({ id: 'a', label: '1K · Ronda 6', round: 6 }),
    play({ id: 'b', label: '2K · Ronda 6', round: 6 }),
  ];
  assert.equal(playsSelectionLabel(picks), '2 jugadas · Rondas 6');
});

test('formatKd renders two decimals', () => {
  assert.equal(formatKd(2.2), '2.20');
});

test('ratingBarPct scales against a 2.0 ceiling', () => {
  assert.equal(ratingBarPct(1.42), 71);
  assert.equal(ratingBarPct(0), 0);
});

test('ratingBarPct clamps an above-ceiling rating to 100', () => {
  assert.equal(ratingBarPct(2.5), 100);
});

test('ratingBarPct clamps a negative rating to 0', () => {
  assert.equal(ratingBarPct(-1), 0);
});

test('prettyMapName strips the de_/cs_ prefix and capitalizes', () => {
  assert.equal(prettyMapName('de_dust2'), 'Dust2');
  assert.equal(prettyMapName('cs_office'), 'Office');
});

test('prettyMapName passes through an unprefixed name, capitalizing it', () => {
  assert.equal(prettyMapName('ancient'), 'Ancient');
  assert.equal(prettyMapName(''), '');
});

test('formatShortDate uses a calendar day, not a relative phrase', () => {
  const cases: Array<{ iso: string; wantDay: string; wantMonth: string }> = [
    { iso: '2026-08-17T12:00:00Z', wantDay: '17', wantMonth: 'ago' },
    { iso: '2026-01-03T12:00:00Z', wantDay: '3', wantMonth: 'ene' },
    { iso: '2024-12-25T12:00:00Z', wantDay: '25', wantMonth: 'dic' },
  ];
  for (const tc of cases) {
    const got = formatShortDate(tc.iso);
    assert.match(got, new RegExp(tc.wantDay), tc.iso);
    assert.match(got, new RegExp(tc.wantMonth, 'i'), tc.iso);
    assert.doesNotMatch(got, /hace /);
  }
});

test('formatShortDate rejects unparseable input', () => {
  assert.equal(formatShortDate('not-a-date'), '—');
});

test('ratingBarClass matches ratingClass band boundaries', () => {
  assert.equal(ratingBarClass(1.15), 'bg-success');
  assert.equal(ratingBarClass(0.95), 'bg-fg-1');
  assert.equal(ratingBarClass(0.8), 'bg-warning');
  assert.equal(ratingBarClass(0.79), 'bg-destructive');
});

test('no rating band reaches outside the token ramp', () => {
  for (const rating of [1.4, 1.15, 1.0, 0.95, 0.85, 0.8, 0.5]) {
    assert.match(ratingClass(rating), /^text-(success|warning|destructive|fg-1)$/);
    assert.match(ratingBarClass(rating), /^bg-(success|warning|destructive|fg-1)$/);
  }
});
