import test from 'node:test';
import assert from 'node:assert/strict';
import { buildEditRequest } from './edit-request.ts';
import { constrainEditConfig } from '../reel-brief.ts';
import type { EditConfig } from './types.ts';
import { FULL_DEMO_EDIT } from '../full-demo.ts';
import { parseEffectiveEditConfig } from './render-hydration.ts';
import { coerceEditConfig } from './reel-store.ts';

function edit(overrides: Partial<EditConfig> = {}): EditConfig {
  return {
    format: 'short-9x16',
    killEffect: 'punch-in',
    transition: 'flash',
    intro: false,
    outro: false,
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'generated-gameplay',
    ...overrides,
  };
}

test('buildEditRequest persists family with the style so a later render cannot forget it', () => {
  const keyDrop = buildEditRequest(edit({ keyDropStyle: 'tigerr', keyDropCode: 'zackcsgo' }));
  assert.equal(keyDrop.keydrop_family, 'KEYDROP');
  assert.equal(keyDrop.keydrop_style, 'tigerr');
  assert.equal(keyDrop.keydrop_code, 'ZACKCSGO');

  const skins = buildEditRequest(
    edit({ keyDropFamily: 'CSGOSKINS', keyDropStyle: 'classic', keyDropCode: 'skins99' }),
  );
  assert.equal(skins.keydrop_family, 'CSGOSKINS');
  assert.equal(skins.keydrop_style, 'classic');
  assert.equal(skins.keydrop_code, 'SKINS99');

  const off = buildEditRequest(edit({ keyDropFamily: 'KEYDROP', keyDropStyle: '' }));
  assert.equal(off.keydrop_family, undefined);
  assert.equal(off.keydrop_style, undefined);
});

test('buildEditRequest sends the locked Full Demo treatment on the wire', () => {
  assert.deepEqual(buildEditRequest(FULL_DEMO_EDIT), {
    format: 'landscape-16x9',
    killEffect: 'clean',
    transition: 'cut',
    intro: false,
    outro: false,
    hook_text: false,
    kill_counter: false,
    match_recap: true,
    voice_comms: true,
    voice_volume: 0.85,
    native_hud: true,
    cover_strategy: 'generated-gameplay',
    demo_source: 'faceit',
    overlay_theme: 'faceit-orange',
  });
});

test('buildEditRequest serializes overlay_theme when set', () => {
  const body = buildEditRequest(
    edit({ format: 'landscape-16x9', matchRecap: true, demoSource: 'faceit', overlayTheme: 'neon-violet' }),
  );
  assert.equal(body.overlay_theme, 'neon-violet');
  assert.equal(body.demo_source, 'faceit');
  assert.equal(buildEditRequest(edit()).overlay_theme, undefined);
});

test('a death reel travels as death_reel without the kill counter or hook', () => {
  const body = buildEditRequest(constrainEditConfig(edit({ deathReel: true, killCounter: true, hookText: true })));
  assert.equal(body.death_reel, true);
  assert.equal(body.kill_counter, false);
  assert.equal(body.hook_text, false);
  assert.equal(Object.hasOwn(buildEditRequest(edit()), 'death_reel'), false);
});

test('buildEditRequest sends the victim replay only for Shorts that ask for it', () => {
  assert.equal(buildEditRequest(edit()).victim_replay, undefined);
  assert.equal(buildEditRequest(edit({ victimReplay: true })).victim_replay, true);
  assert.equal(buildEditRequest({ ...FULL_DEMO_EDIT, victimReplay: true }).victim_replay, undefined);
});

test('buildEditRequest sends centered_killfeed only for a vertical Short', () => {
  assert.equal(buildEditRequest(edit({ centeredKillfeed: true })).centered_killfeed, true);
  assert.equal('centered_killfeed' in buildEditRequest(edit()), false);
  // The server rejects the option outside 9:16, so a stale toggle must not reach it.
  assert.equal('centered_killfeed' in buildEditRequest(edit({ centeredKillfeed: true, format: 'landscape-16x9' })), false);
  // Death clips have no kills to draw.
  assert.equal('centered_killfeed' in buildEditRequest(edit({ centeredKillfeed: true, deathReel: true })), false);
});

test('buildEditRequest sends the vertical HUD with its kill feed only for a vertical kill Short', () => {
  const hud = buildEditRequest(edit({ verticalHud: true, verticalHudTheme: 'prism' }));
  assert.equal(hud.vertical_hud, true);
  assert.equal(hud.vertical_hud_theme, 'prism');
  // The HUD draws its kill feed under the scoreboard, and the server refuses it without one.
  assert.equal(hud.centered_killfeed, true);
  assert.equal('vertical_hud_theme' in buildEditRequest(edit({ verticalHud: true })), false);
  for (const off of [
    edit({ verticalHud: true, verticalHudTheme: 'prism', format: 'landscape-16x9' }),
    edit({ verticalHud: true, verticalHudTheme: 'prism', deathReel: true }),
    edit({ verticalHud: true, verticalHudTheme: 'prism', nativeHud: true }),
    edit({ verticalHud: true, verticalHudTheme: 'prism', matchRecap: true }),
    edit({ verticalHudTheme: 'prism' }),
  ]) {
    const body = buildEditRequest(off);
    assert.equal('vertical_hud' in body, false);
    assert.equal('vertical_hud_theme' in body, false);
    assert.equal('centered_killfeed' in body, false);
  }
});

test('buildEditRequest sends the radar with its chosen options only for a Short without CS2 HUD', () => {
  const body = buildEditRequest(edit({ radar: true, radarPosition: 'top-right', radarSize: 'large', radarOpacity: 0.6 }));
  assert.equal(body.radar, true);
  assert.equal(body.radar_position, 'top-right');
  assert.equal(body.radar_size, 'large');
  assert.equal(body.radar_opacity, 0.6);
  // Defaults go unset, as the server stores them, so a rendered radar reads back as the same request.
  const defaults = buildEditRequest(edit({ radar: true, radarSize: 'medium', radarOpacity: 0.9 }));
  assert.equal(defaults.radar, true);
  for (const key of ['radar_position', 'radar_size', 'radar_opacity'] as const) assert.equal(key in defaults, false);
  for (const off of [
    edit({ radar: true, nativeHud: true }),
    edit({ radar: true, matchRecap: true }),
    edit({ radarPosition: 'top-left', radarSize: 'large' }),
  ]) {
    const sent = buildEditRequest(off);
    for (const key of ['radar', 'radar_position', 'radar_size', 'radar_opacity'] as const) assert.equal(key in sent, false);
  }
});

test('buildEditRequest sends a pre-roll other than 3 s only for a video of kills', () => {
  assert.equal(buildEditRequest(edit({ preRollSeconds: 5 })).pre_roll_seconds, 5);
  assert.equal(buildEditRequest(edit({ format: 'landscape-16x9', preRollSeconds: 8 })).pre_roll_seconds, 8);
  assert.equal(buildEditRequest(edit({ preRollSeconds: 3 })).pre_roll_seconds, undefined, 'the analysis cut is the default');
  assert.equal(buildEditRequest(edit({ preRollSeconds: 5, deathReel: true })).pre_roll_seconds, undefined);
  assert.equal(buildEditRequest(edit({ preRollSeconds: 5, matchRecap: true })).pre_roll_seconds, undefined);
});

test('buildEditRequest sends the capture style only for a 16:9 video without the native HUD', () => {
  assert.equal(buildEditRequest(edit({ format: 'landscape-16x9', captureHud: 'clean' })).capture_hud, 'clean');
  assert.equal(buildEditRequest(edit({ format: 'landscape-16x9', captureHud: 'clean', nativeHud: true })).capture_hud, undefined);
  assert.equal(buildEditRequest(edit({ captureHud: 'clean' })).capture_hud, undefined, 'a 9:16 Short keeps its preset HUD');
});

test('a horizontal asks for the kill feed framing of its Short only in 16:9 with a kill feed', () => {
  const framed = (patch: Partial<EditConfig>) => buildEditRequest(edit({ portraitSafeKillfeed: true, ...patch })).portrait_safe_killfeed;
  assert.equal(framed({ format: 'landscape-16x9', captureHud: 'deathnotices' }), true);
  assert.equal(framed({ format: 'landscape-16x9', nativeHud: true }), true, 'HUD completo keeps the kill feed');
  assert.equal(framed({ format: 'landscape-16x9', captureHud: 'clean' }), undefined, 'POV limpio has no kill feed');
  assert.equal(framed({}), undefined, 'a Short frames it from its preset');
});

test('a Short keeps its own comms, sends its subtitles only with them, and reads them back for Editar', () => {
  const short = constrainEditConfig(edit({ voiceComms: true, subtitles: true, subtitlesHidden: ['b', 'a'] }));
  const body = buildEditRequest(short);
  assert.equal(body.voice_comms, true);
  assert.equal(body.subtitles, true);
  assert.deepEqual(body.subtitles_hidden, ['a', 'b']);
  assert.equal(buildEditRequest(edit({ subtitles: true })).subtitles, undefined);
  const back = parseEffectiveEditConfig(JSON.parse(JSON.stringify(body)));
  assert.equal(back?.subtitles, true);
  assert.deepEqual(back?.subtitlesHidden, ['a', 'b']);
});

test('a Short saved or rendered by 5.25.0 with the English subtitles loads with the subtitles in the spoken language', () => {
  const stored: unknown = JSON.parse('{"format":"short-9x16","voiceComms":true,"subtitles":true,"subtitlesEnglish":true,"subtitlesHidden":["a"]}');
  const saved = coerceEditConfig(stored);
  assert.equal(saved.subtitles, true);
  assert.deepEqual(saved.subtitlesHidden, ['a']);
  assert.equal('subtitlesEnglish' in saved, false);
  const rendered = parseEffectiveEditConfig({ ...buildEditRequest(saved), subtitles_english: true });
  assert.equal(rendered?.subtitles, true);
  assert.equal(rendered !== undefined && 'subtitlesEnglish' in rendered, false);
  assert.equal('subtitles_english' in buildEditRequest(saved), false);
});
