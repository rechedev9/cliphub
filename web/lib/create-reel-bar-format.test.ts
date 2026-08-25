import test from 'node:test';
import assert from 'node:assert/strict';
import type { Preset } from './api/types.ts';
import {
  isLandscapePreset,
  lockedFormatLabel,
  resolvedReelFormat,
  selectShortsFormat,
  selectShortsPreset,
  shortsPresetsForFormat,
} from './reel-format.ts';

const VERTICAL: Preset = {
  name: 'viral-60-clean',
  label: 'Killfeed',
  description: 'test',
  hudMode: 'deathnotices',
  default: true,
  width: 1080,
  height: 1920,
};

const LANDSCAPE: Preset = {
  name: 'gameplay-pov-60',
  label: 'POV nativo',
  description: 'test',
  hudMode: 'gameplay',
  width: 1920,
  height: 1080,
};

const PRESETS = [VERTICAL, LANDSCAPE];

test('locked format label names the locked delivery', () => {
  const cases: Array<{ format: 'short-9x16' | 'landscape-16x9'; want: string }> = [
    { format: 'landscape-16x9', want: '16:9' },
    { format: 'short-9x16', want: '9:16' },
  ];
  for (const tc of cases) {
    assert.equal(lockedFormatLabel(tc.format), tc.want);
  }
});

test('landscape presets are the 1920×1080 YouTube POV cards', () => {
  const cases: Array<{ preset: Pick<Preset, 'name' | 'width' | 'height'>; want: boolean }> = [
    { preset: VERTICAL, want: false },
    { preset: LANDSCAPE, want: true },
    { preset: { name: 'gameplay-pov-60' }, want: true },
    { preset: { name: 'viral-60-clean' }, want: false },
  ];
  for (const tc of cases) {
    assert.equal(isLandscapePreset(tc.preset), tc.want, tc.preset.name);
  }
});

test('resolved reel format follows landscape format or a 16:9 preset', () => {
  const cases: Array<{ format: 'short-9x16' | 'landscape-16x9'; preset: Preset | null; want: 'short-9x16' | 'landscape-16x9' }> = [
    { format: 'short-9x16', preset: VERTICAL, want: 'short-9x16' },
    { format: 'landscape-16x9', preset: VERTICAL, want: 'landscape-16x9' },
    { format: 'short-9x16', preset: LANDSCAPE, want: 'landscape-16x9' },
    { format: 'landscape-16x9', preset: LANDSCAPE, want: 'landscape-16x9' },
    { format: 'short-9x16', preset: null, want: 'short-9x16' },
  ];
  for (const tc of cases) {
    assert.equal(resolvedReelFormat(tc.format, tc.preset), tc.want);
  }
});

test('shorts 9:16 picker hides POV nativo and 16:9 keeps every card', () => {
  assert.deepEqual(shortsPresetsForFormat(PRESETS, 'short-9x16').map((preset) => preset.name), ['viral-60-clean']);
  assert.deepEqual(shortsPresetsForFormat(PRESETS, 'landscape-16x9').map((preset) => preset.name), [
    'viral-60-clean',
    'gameplay-pov-60',
  ]);
});

test('picking POV nativo moves the Shorts constructor to 16:9', () => {
  assert.deepEqual(selectShortsPreset('gameplay-pov-60', 'short-9x16', PRESETS), {
    format: 'landscape-16x9',
    variant: 'gameplay-pov-60',
  });
  assert.deepEqual(selectShortsPreset('viral-60-clean', 'landscape-16x9', PRESETS), {
    format: 'landscape-16x9',
    variant: 'viral-60-clean',
  });
});

test('switching Shorts back to 9:16 drops a landscape-only preset', () => {
  assert.deepEqual(selectShortsFormat('short-9x16', 'gameplay-pov-60', PRESETS), {
    format: 'short-9x16',
    variant: 'viral-60-clean',
  });
  assert.deepEqual(selectShortsFormat('landscape-16x9', 'viral-60-clean', PRESETS), {
    format: 'landscape-16x9',
    variant: 'viral-60-clean',
  });
});
