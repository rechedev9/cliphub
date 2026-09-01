// Unit tests for the pure intent coercion (corrupt-storage tolerance).
// Run: node --test reel-store.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { coerceEditConfig, coerceIntents, DEFAULT_EDIT_CONFIG } from './reel-store.ts';
import type { ReelIntent } from './reel-store.ts';

const valid: ReelIntent = {
  videoId: 'job__seg-001',
  jobId: 'job',
  segmentIds: ['seg-001'],
  mode: 'music',
  variant: 'clean-pov-60',
  editConfig: { format: 'landscape-16x9', killEffect: 'velocity', transition: 'whip', intro: true, outro: false, hookText: true, killCounter: false, matchRecap: false, voiceComms: false, nativeHud: false, coverStrategy: 'no-cover', introText: 'GG WP', outroText: '' },
  songId: 's1',
  musicVolume: 0.35,
  title: 'Ace',
  map: 'de_dust2',
  score: '13-7',
  targetName: 'zack',
  createdAt: 123,
};

test('keeps a well-formed intent verbatim', () => {
  assert.deepEqual(coerceIntents([valid]), [valid]);
});

test('non-array input → empty', () => {
  assert.deepEqual(coerceIntents(null), []);
  assert.deepEqual(coerceIntents({}), []);
  assert.deepEqual(coerceIntents('nope'), []);
});

test('drops entries missing required ids', () => {
  assert.deepEqual(coerceIntents([{ jobId: 'j', segmentIds: ['s'] }, 42, null]), []);
});

test('drops entries with no segment ids at all', () => {
  assert.deepEqual(coerceIntents([{ videoId: 'v', jobId: 'j' }, { videoId: 'v', jobId: 'j', segmentIds: [] }]), []);
});

test('keeps a Full Demo recap intent with empty segment ids', () => {
  const recap = {
    videoId: 'job__full-demo',
    jobId: 'job',
    segmentIds: [] as string[],
    mode: 'clean' as const,
    variant: 'gameplay-pov-60',
    editConfig: {
      format: 'landscape-16x9' as const,
      killEffect: 'clean' as const,
      transition: 'cut' as const,
      intro: false,
      outro: false,
      hookText: false,
      killCounter: false,
      matchRecap: true,
      voiceComms: true,
      nativeHud: true,
      coverStrategy: 'generated-gameplay' as const,
      introText: '',
      outroText: '',
    },
    title: '12 rondas - POV nativo',
    map: 'Inferno',
    score: '',
    createdAt: 9,
  };
  const kept = coerceIntents([recap, { videoId: 'shorts', jobId: 'job', segmentIds: [] }]);
  assert.equal(kept.length, 1);
  assert.equal(kept[0]?.videoId, 'job__full-demo');
  assert.deepEqual(kept[0]?.segmentIds, []);
  assert.equal(kept[0]?.editConfig.matchRecap, true);
});

test('defaults soft fields, normalizes mode, migrates missing variant', () => {
  assert.deepEqual(coerceIntents([{ videoId: 'v', jobId: 'j', segmentIds: ['s'], mode: 'weird' }]), [
    { videoId: 'v', jobId: 'j', segmentIds: ['s'], mode: 'clean', variant: 'viral-60-clean', editConfig: DEFAULT_EDIT_CONFIG, songId: undefined, musicVolume: undefined, title: 'Highlight', map: 'Unknown', score: '', targetName: undefined, createdAt: 0 },
  ]);
});

test('migrates a legacy singular segmentId into a one-element segmentIds array', () => {
  assert.deepEqual(coerceIntents([{ videoId: 'v', jobId: 'j', segmentId: 'seg-001', mode: 'clean' }]), [
    { videoId: 'v', jobId: 'j', segmentIds: ['seg-001'], mode: 'clean', variant: 'viral-60-clean', editConfig: DEFAULT_EDIT_CONFIG, songId: undefined, musicVolume: undefined, title: 'Highlight', map: 'Unknown', score: '', targetName: undefined, createdAt: 0 },
  ]);
});

test('keeps a valid music volume and drops out-of-range or mistyped ones', () => {
  const base = { videoId: 'v', jobId: 'j', segmentIds: ['s'], mode: 'music' as const, songId: 's1' };
  assert.equal(coerceIntents([{ ...base, musicVolume: 0.35 }])[0]?.musicVolume, 0.35);
  assert.equal(coerceIntents([{ ...base, musicVolume: 1 }])[0]?.musicVolume, 1);
  assert.equal(coerceIntents([{ ...base, musicVolume: 0 }])[0]?.musicVolume, undefined);
  assert.equal(coerceIntents([{ ...base, musicVolume: 1.5 }])[0]?.musicVolume, undefined);
  assert.equal(coerceIntents([{ ...base, musicVolume: '0.5' }])[0]?.musicVolume, undefined);
  assert.equal(coerceIntents([{ ...base, musicVolume: Number.NaN }])[0]?.musicVolume, undefined);
  assert.equal(coerceIntents([base])[0]?.musicVolume, undefined);
});

test('keeps a valid game volume including mute and drops out-of-range ones', () => {
  const base = { videoId: 'v', jobId: 'j', segmentIds: ['s'], mode: 'music' as const, songId: 's1' };
  assert.equal(coerceIntents([{ ...base, gameVolume: 0.2 }])[0]?.gameVolume, 0.2);
  assert.equal(coerceIntents([{ ...base, gameVolume: 0 }])[0]?.gameVolume, 0);
  assert.equal(coerceIntents([{ ...base, gameVolume: 1.5 }])[0]?.gameVolume, undefined);
  assert.equal(coerceIntents([base])[0]?.gameVolume, undefined);
});

test('ignores the legacy fake published flag instead of treating it as a real upload', () => {
  assert.deepEqual(coerceIntents([{ ...valid, published: true }]), [valid]);
});

test('keeps multiple segment ids in plan order', () => {
  const multi = { ...valid, videoId: 'job__seg-001_seg-006', segmentIds: ['seg-001', 'seg-006', 'seg-009'] };
  assert.deepEqual(coerceIntents([multi]), [multi]);
});

test('coerces edit config independently', () => {
  assert.deepEqual(
    coerceEditConfig({
      format: 'landscape-16x9',
      killEffect: 'freeze-flash',
      transition: 'dip',
      intro: true,
      outro: true,
      hookText: true,
      killCounter: true,
      matchRecap: true,
      voiceComms: true,
      nativeHud: true,
      coverStrategy: 'no-cover',
      introText: 'GG WP',
      outroText: '@handle',
    }),
    {
      format: 'landscape-16x9',
      killEffect: 'freeze-flash',
      transition: 'dip',
      intro: true,
      outro: true,
      hookText: true,
      killCounter: true,
      matchRecap: true,
      voiceComms: true,
      nativeHud: true,
      coverStrategy: 'no-cover',
      introText: 'GG WP',
      outroText: '@handle',
    },
  );
  assert.deepEqual(coerceEditConfig({ format: 'square', killEffect: 'bad', transition: 'spin', intro: 'yes' }), DEFAULT_EDIT_CONFIG);
});

test('automatic text controls preserve only explicit true values', () => {
  assert.equal(coerceEditConfig({ hookText: true, killCounter: true }).hookText, true);
  assert.equal(coerceEditConfig({ hookText: true, killCounter: true }).killCounter, true);
  assert.equal(coerceEditConfig({ matchRecap: true, voiceComms: true, nativeHud: true }).matchRecap, true);
  assert.equal(coerceEditConfig({ matchRecap: true, voiceComms: true, nativeHud: true }).voiceComms, true);
  assert.equal(coerceEditConfig({ matchRecap: true, voiceComms: true, nativeHud: true }).nativeHud, true);
  assert.equal(coerceEditConfig({ hookText: 'true', killCounter: 1 }).hookText, false);
  assert.equal(coerceEditConfig({ hookText: 'true', killCounter: 1 }).killCounter, false);
  assert.equal(coerceEditConfig({ matchRecap: 'true', voiceComms: 1, nativeHud: 'yes' }).matchRecap, false);
  assert.equal(coerceEditConfig({ matchRecap: 'true', voiceComms: 1, nativeHud: 'yes' }).voiceComms, false);
  assert.equal(coerceEditConfig({ matchRecap: 'true', voiceComms: 1, nativeHud: 'yes' }).nativeHud, false);
  assert.equal(coerceEditConfig({}).hookText, false);
  assert.equal(coerceEditConfig({}).killCounter, false);
  assert.equal(coerceEditConfig({}).matchRecap, false);
  assert.equal(coerceEditConfig({}).voiceComms, false);
  assert.equal(coerceEditConfig({}).nativeHud, false);
  assert.equal(coerceEditConfig({ voiceVolume: 0.4 }).voiceVolume, 0.4);
  assert.equal(coerceEditConfig({ voiceVolume: 0 }).voiceVolume, 0);
  assert.equal(coerceEditConfig({ voiceVolume: 1.5 }).voiceVolume, undefined);
  assert.equal(coerceEditConfig({ demoSource: 'premier' }).demoSource, 'premier');
  assert.equal(coerceEditConfig({ demoSource: 'professional' }).demoSource, 'professional');
  assert.equal(coerceEditConfig({ demoSource: 'faceit' }).demoSource, 'faceit');
  assert.equal(coerceEditConfig({ demoSource: 'esea' }).demoSource, undefined);
});

test('coerceEditConfig keeps every catalog KeyDrop style and drops unknowns', () => {
  for (const style of ['operator', 'classic', 'tigerr', 'jcorko'] as const) {
    assert.equal(coerceEditConfig({ keyDropStyle: style }).keyDropStyle, style);
  }
  assert.equal(coerceEditConfig({ keyDropStyle: 'neon' }).keyDropStyle, undefined);
});

test('truncates bookend text to the 80-char limit and drops non-string values', () => {
  const longText = 'x'.repeat(120);
  const coerced = coerceEditConfig({ format: 'short-9x16', killEffect: 'clean', transition: 'cut', intro: true, outro: true, introText: longText, outroText: 42 });
  assert.equal(coerced.introText?.length, 80);
  assert.equal(coerced.outroText, '');
});
