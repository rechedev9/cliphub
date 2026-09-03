import test from 'node:test';
import assert from 'node:assert/strict';
import {
  applyEffectiveRenderMusic,
  clearVideoArtifactUrls,
  hydrateVideoFromIntent,
  parseEffectiveEditConfig,
  parseEffectiveRenderMusic,
} from './render-hydration.ts';
import { DEFAULT_EDIT_CONFIG, type ReelIntent } from './reel-store.ts';
import { buildEditRequest } from './edit-request.ts';
import type { EditConfig, Video } from './types.ts';

function intent(): ReelIntent {
  return {
    videoId: 'job__seg-001',
    jobId: 'job',
    segmentIds: ['seg-001'],
    mode: 'clean',
    variant: 'viral-60-clean',
    editConfig: DEFAULT_EDIT_CONFIG,
    title: 'Ace',
    map: 'de_dust2',
    score: '13-7',
    createdAt: 1,
  };
}

test('effective music parser distinguishes explicit clean from legacy unknown', () => {
  assert.deepEqual(parseEffectiveRenderMusic({ key: '', volume: 0 }), { mode: 'clean' });
  assert.deepEqual(
    parseEffectiveRenderMusic({ key: 'phonk-01', volume: 0.35 }),
    { mode: 'music', songId: 'phonk-01', musicVolume: 0.35 },
  );
  assert.deepEqual(
    parseEffectiveRenderMusic({ key: 'phonk-01', volume: 0.35, game_volume: 0.2 }),
    { mode: 'music', songId: 'phonk-01', musicVolume: 0.35, gameVolume: 0.2 },
  );
  assert.equal(parseEffectiveRenderMusic(undefined), undefined);
  assert.equal(parseEffectiveRenderMusic({ key: 'phonk-01', volume: 0 }), undefined);
  assert.equal(parseEffectiveRenderMusic({ key: '', volume: 1 }), undefined);
});

test('effective edit parser reads the Go mixed wire fields', () => {
  const expected = {
    format: 'short-9x16' as const,
    killEffect: 'freeze-flash' as const,
    transition: 'dip' as const,
    coverStrategy: 'generated-gameplay' as const,
    intro: true,
    outro: false,
    hookText: true,
    killCounter: false,
    matchRecap: false,
      voiceComms: true,
      voiceVolume: 0.4,
      nativeHud: false,
    introText: 'Watch this',
  };
  // Live orchestrator wire: killEffect camelCase + snake_case booleans.
  assert.deepEqual(
    parseEffectiveEditConfig({
      format: 'short-9x16',
      killEffect: 'freeze-flash',
      transition: 'dip',
      cover_strategy: 'generated-gameplay',
      intro: true,
      outro: false,
      hook_text: true,
      kill_counter: false,
      voice_comms: true,
      voice_volume: 0.4,
      intro_text: 'Watch this',
    }),
    expected,
  );
  // Legacy / alternate spelling still accepted.
  assert.deepEqual(
    parseEffectiveEditConfig({
      format: 'short-9x16',
      kill_effect: 'freeze-flash',
      transition: 'dip',
      cover_strategy: 'generated-gameplay',
      intro: true,
      outro: false,
      hook_text: true,
      kill_counter: false,
      voice_comms: true,
      voice_volume: 0.4,
      intro_text: 'Watch this',
    }),
    expected,
  );
  assert.equal(
    parseEffectiveEditConfig({
      format: 'short-9x16',
      transition: 'dip',
      cover_strategy: 'generated-gameplay',
      intro: true,
      outro: false,
      hook_text: true,
      kill_counter: false,
    }),
    undefined,
  );
  const withTigerr = parseEffectiveEditConfig({
    format: 'short-9x16',
    killEffect: 'freeze-flash',
    transition: 'dip',
    cover_strategy: 'generated-gameplay',
    intro: true,
    outro: false,
    hook_text: true,
    kill_counter: false,
    keydrop_style: 'tigerr',
  });
  const withJcorko = parseEffectiveEditConfig({
    format: 'short-9x16',
    killEffect: 'freeze-flash',
    transition: 'dip',
    cover_strategy: 'generated-gameplay',
    intro: true,
    outro: false,
    hook_text: true,
    kill_counter: false,
    keydrop_style: 'jcorko',
  });
  assert.equal(withTigerr?.keyDropStyle, 'tigerr');
  assert.equal(withTigerr?.keyDropFamily, 'KEYDROP');
  assert.equal(withJcorko?.keyDropStyle, 'jcorko');
  const withSkins = parseEffectiveEditConfig({
    format: 'short-9x16',
    killEffect: 'freeze-flash',
    transition: 'dip',
    cover_strategy: 'generated-gameplay',
    intro: true,
    outro: false,
    hook_text: true,
    kill_counter: false,
    keydrop_family: 'CSGOSKINS',
    keydrop_style: 'classic',
  });
  assert.equal(withSkins?.keyDropFamily, 'CSGOSKINS');
  assert.equal(withSkins?.keyDropStyle, 'classic');
  assert.equal(
    parseEffectiveEditConfig({
      format: 'short-9x16',
      killEffect: 'freeze-flash',
      transition: 'dip',
      cover_strategy: 'generated-gameplay',
      intro: true,
      outro: false,
      hook_text: true,
      kill_counter: false,
      keydrop_family: 'CSGOSKINS',
      keydrop_style: 'tigerr',
    })?.keyDropStyle,
    undefined,
  );
  assert.equal(
    parseEffectiveEditConfig({
      format: 'short-9x16',
      killEffect: 'freeze-flash',
      transition: 'dip',
      cover_strategy: 'generated-gameplay',
      intro: true,
      outro: false,
      hook_text: true,
      kill_counter: false,
      keydrop_style: 'neon',
    })?.keyDropStyle,
    undefined,
  );
  assert.equal(
    parseEffectiveEditConfig({
      format: 'landscape-16x9',
      killEffect: 'clean',
      transition: 'cut',
      cover_strategy: 'generated-gameplay',
      intro: false,
      outro: false,
      hook_text: false,
      kill_counter: false,
      match_recap: true,
      demo_source: 'premier',
    })?.demoSource,
    'premier',
  );
  assert.equal(
    parseEffectiveEditConfig({
      format: 'landscape-16x9',
      killEffect: 'clean',
      transition: 'cut',
      cover_strategy: 'generated-gameplay',
      intro: false,
      outro: false,
      hook_text: false,
      kill_counter: false,
      match_recap: true,
      demo_source: 'esea',
    })?.demoSource,
    undefined,
  );
});

test('a fully populated edit survives the wire round trip through buildEditRequest and the parser', () => {
  // Anything dropped here becomes a mismatch redrive (or a silently different
  // reel) the next time the Library adopts the server's effective edit.
  const full: EditConfig = {
    format: 'short-9x16',
    killEffect: 'punch-in',
    transition: 'whip',
    coverStrategy: 'generated-gameplay',
    intro: true,
    outro: true,
    hookText: true,
    killCounter: true,
    matchRecap: false,
    voiceComms: true,
    voiceVolume: 0.6,
    nativeHud: false,
    introText: 'GO',
    outroText: 'GG',
    keyDropFamily: 'KEYDROP',
    keyDropStyle: 'tigerr',
    keyDropCode: 'ZACK',
    keyDropPositionY: 0.72,
    keyDropStartSeconds: 2.5,
    keyDropEndSeconds: 11,
    demoSource: 'faceit',
    overlayTheme: 'neon-violet',
  };
  assert.deepEqual(parseEffectiveEditConfig(buildEditRequest(full)), full);
});

test('effective render music replaces stale cross-tab intent fields', () => {
  const reel = intent();
  assert.equal(
    applyEffectiveRenderMusic(reel, {
      mode: 'music',
      songId: 'phonk-01',
      musicVolume: 0.35,
    }),
    true,
  );
  assert.equal(reel.mode, 'music');
  assert.equal(reel.songId, 'phonk-01');
  assert.equal(reel.musicVolume, 0.35);

  assert.equal(applyEffectiveRenderMusic(reel, { mode: 'clean' }), true);
  assert.equal(reel.mode, 'clean');
  assert.equal(reel.songId, undefined);
  assert.equal(reel.musicVolume, undefined);
});

test('live video hydration replaces stale edit and music fields', () => {
  const reel = intent();
  reel.mode = 'music';
  reel.songId = 'phonk-01';
  reel.musicVolume = 0.35;
  reel.editConfig = {
    ...DEFAULT_EDIT_CONFIG,
    transition: 'whip',
  };
  const stale: Video = {
    id: reel.videoId,
    title: reel.title,
    map: reel.map,
    score: reel.score,
    mode: 'clean',
    editConfig: DEFAULT_EDIT_CONFIG,
    status: 'ready',
    createdAt: reel.createdAt,
  };

  const hydrated = hydrateVideoFromIntent(stale, reel);
  assert.equal(hydrated.mode, 'music');
  assert.equal(hydrated.songId, 'phonk-01');
  assert.equal(hydrated.musicVolume, 0.35);
  assert.equal(hydrated.editConfig?.transition, 'whip');
});

test('queued render revisions do not inherit artifact URLs while names are transiently missing', () => {
  const reel = intent();
  const previousRevision: Video = {
    id: reel.videoId,
    title: reel.title,
    map: reel.map,
    score: reel.score,
    mode: reel.mode,
    status: 'review_required',
    createdAt: reel.createdAt,
    downloadUrl: '/api/demos/job/renders/viral-60-clean/videos/old.mp4',
    thumbnailUrl: '/api/demos/job/renders/viral-60-clean/covers/old.jpg',
  };

  const queuedRevision = clearVideoArtifactUrls({
    ...previousRevision,
    status: 'queued',
  });

  assert.equal(queuedRevision.downloadUrl, undefined);
  assert.equal(queuedRevision.thumbnailUrl, undefined);
  assert.equal(clearVideoArtifactUrls(queuedRevision).downloadUrl, undefined);
});
