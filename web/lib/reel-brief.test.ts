import assert from 'node:assert/strict';
import test from 'node:test';
import {
  canForgeReel,
  canRerenderWithMusic,
  constrainEditConfig,
  isLandscapeRecap,
  missingAffiliateCode,
  reelCreativeBrief,
  verticalHudAllowed,
  withPresetLimits,
} from './reel-brief.ts';
import type { EditConfig, Preset } from './api/types.ts';
import { FULL_DEMO_EDIT, FULL_DEMO_PRESET } from './full-demo.ts';
import { NATIVE_HUD_LABEL } from './preset-copy.ts';

const PRESET: Preset = {
  name: 'viral-60-clean',
  label: 'Kill Feed',
  description: 'test',
  hudMode: 'deathnotices',
};

test('library music rerender needs a changed mix and no active operation', () => {
  const ready = { busy: false, musicChanged: true };
  assert.equal(canRerenderWithMusic(ready), true);
  assert.equal(canRerenderWithMusic({ ...ready, busy: true }), false);
  assert.equal(canRerenderWithMusic({ ...ready, musicChanged: false }), false);
});

test('forging starts with configured options and no separate approval', () => {
  const ready = { creating: false, hasPreset: true, selectionCount: 1 };
  assert.equal(canForgeReel(ready), true);
  assert.equal(canForgeReel({ ...ready, creating: true }), false);
  assert.equal(canForgeReel({ ...ready, hasPreset: false }), false);
  assert.equal(canForgeReel({ ...ready, selectionCount: 0 }), false);
});

test('creative brief resolves every required production choice', () => {
  const edit: EditConfig = {
    format: 'short-9x16',
    killEffect: 'punch-in',
    transition: 'flash',
    hookText: true,
    killCounter: true,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'generated-gameplay',
    intro: true,
    introText: 'Entrada',
    outro: false,
    outroText: '',
  };

  assert.deepEqual(reelCreativeBrief(edit, PRESET, { status: 'track', title: 'Tema CC0', volumePercent: 35, gameVolumePercent: 20 }), [
    { label: 'Formato', value: 'Vertical 9:16 · 1080×1920' },
    { label: 'Entrega', value: 'Compilado de jugadas' },
    { label: 'Comms', value: 'Sin comms' },
    { label: 'HUD / killfeed', value: 'Sin HUD, conserva killfeed' },
    { label: 'Efecto de kill', value: 'Impacto (zoom rápido en cada baja)' },
    { label: 'Transiciones', value: 'Destello' },
    { label: 'Título / contador', value: 'Título automático · Contador de kills' },
    { label: 'Insignias / tarjeta', value: 'Sin insignias · Sin tarjeta' },
    { label: 'Probabilidad de victoria', value: 'No' },
    { label: 'Idioma de los textos', value: 'Español' },
    { label: 'Repetición', value: 'No' },
    { label: 'Sobre el juego', value: 'No' },
    { label: 'Radar', value: 'No' },
    { label: 'Apertura', value: 'Sí · «Entrada»' },
    { label: 'Cierre', value: 'No' },
    { label: 'Banner afiliado', value: 'No' },
    { label: 'Música', value: 'Tema CC0 · música 35 % · juego 20 %' },
    { label: 'Portada', value: 'Automática (fotograma del vídeo)' },
  ]);
});

test('creative brief makes disabled options and missing preset explicit', () => {
  const edit: EditConfig = {
    format: 'landscape-16x9',
    killEffect: 'clean',
    transition: 'cut',
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'no-cover',
    intro: false,
    outro: true,
    introText: '',
    outroText: '',
  };
  const brief = Object.fromEntries(reelCreativeBrief(edit, null, { status: 'none' }).map((item) => [item.label, item.value]));
  assert.equal(brief['Formato'], 'Horizontal 16:9 · 1920×1080');
  assert.equal(brief['Entrega'], 'Compilado de jugadas');
  assert.equal(brief['Comms'], 'Sin comms');
  assert.equal(brief['HUD / killfeed'], 'Pendiente de estilo');
  assert.equal(brief['Título / contador'], 'Sin título automático · Sin contador');
  assert.equal(brief['Apertura'], 'No');
  assert.equal(brief['Cierre'], 'Sí · firma ClipHub');
  assert.equal(brief['Banner afiliado'], 'No');
  assert.equal(brief['Música'], 'Sin música');
  assert.equal(brief['Portada'], 'No generar portada');
});

test('9:16 shorts brief never claims a landscape recap even if those flags leak in', () => {
  const edit: EditConfig = {
    format: 'short-9x16',
    killEffect: 'clean',
    transition: 'cut',
    hookText: false,
    killCounter: false,
    matchRecap: true,
    voiceComms: true,
    nativeHud: true,
    coverStrategy: 'no-cover',
    intro: false,
    outro: false,
  };
  const brief = Object.fromEntries(reelCreativeBrief(edit, PRESET, { status: 'none' }).map((item) => [item.label, item.value]));
  assert.equal(brief['Formato'], 'Vertical 9:16 · 1080×1920');
  assert.equal(brief['Entrega'], 'Compilado de jugadas');
  assert.equal(brief['Comms'], 'Sin comms');
  assert.equal(brief['HUD / killfeed'], 'Sin HUD, conserva killfeed');
  assert.equal(isLandscapeRecap(edit), false);
  assert.deepEqual(constrainEditConfig(edit), { ...edit, matchRecap: false, voiceComms: false, nativeHud: false });
});

test('creative brief names the optional recap extras when they are on', () => {
  const edit: EditConfig = {
    format: 'landscape-16x9',
    killEffect: 'clean',
    transition: 'cut',
    hookText: false,
    killCounter: false,
    matchRecap: true,
    voiceComms: true,
    nativeHud: true,
    coverStrategy: 'no-cover',
    intro: false,
    outro: false,
  };
  const brief = Object.fromEntries(reelCreativeBrief(edit, PRESET, { status: 'none' }).map((item) => [item.label, item.value]));
  assert.equal(brief['Entrega'], 'POV landscape · rondas en vivo (sin freeze)');
  assert.equal(brief['Comms'], 'Mezclar comms del equipo · 85 %');
  assert.equal(brief['HUD / killfeed'], NATIVE_HUD_LABEL);
  assert.equal(isLandscapeRecap(edit), true);
  assert.equal(constrainEditConfig(edit), edit);
});

test('full-demo locked edit names native CS2 HUD, not Shorts full-hud copy', () => {
  const brief = Object.fromEntries(
    reelCreativeBrief(FULL_DEMO_EDIT, FULL_DEMO_PRESET, { status: 'none' }).map((item) => [item.label, item.value]),
  );
  assert.equal(brief['Formato'], 'Horizontal 16:9 · 1920×1080');
  assert.equal(brief['HUD / killfeed'], NATIVE_HUD_LABEL);
  assert.notEqual(brief['HUD / killfeed'], 'HUD completo con killfeed');
});

test('shorts 9:16 brief names landscape geometry when POV nativo is selected', () => {
  const edit: EditConfig = {
    format: 'short-9x16',
    killEffect: 'clean',
    transition: 'cut',
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'no-cover',
    intro: false,
    outro: false,
  };
  const preset: Preset = {
    name: 'gameplay-pov-60',
    label: 'POV nativo',
    description: '1920×1080 at 60fps',
    hudMode: 'gameplay',
    width: 1920,
    height: 1080,
  };
  const brief = Object.fromEntries(reelCreativeBrief(edit, preset, { status: 'none' }).map((item) => [item.label, item.value]));
  assert.equal(brief['Formato'], 'Horizontal 16:9 · 1920×1080');
});

test('shorts full-hud-60 brief keeps HUD completo when nativeHud is off', () => {
  const edit: EditConfig = {
    format: 'short-9x16',
    killEffect: 'punch-in',
    transition: 'flash',
    hookText: true,
    killCounter: true,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'generated-gameplay',
    intro: false,
    outro: false,
  };
  const preset: Preset = {
    name: 'full-hud-60',
    label: 'HUD completo',
    description: 'test',
    hudMode: 'gameplay',
  };
  const brief = Object.fromEntries(reelCreativeBrief(edit, preset, { status: 'none' }).map((item) => [item.label, item.value]));
  assert.equal(brief['Formato'], 'Vertical 9:16 · 1080×1920');
  assert.equal(brief['HUD / killfeed'], 'HUD completo con killfeed');
});

test('creative brief names Tigerr and Jcorko affiliate styles', () => {
  const base: EditConfig = {
    format: 'short-9x16',
    killEffect: 'clean',
    transition: 'cut',
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'no-cover',
    intro: false,
    outro: false,
    keyDropStartSeconds: 0,
    keyDropEndSeconds: 4,
  };
  const tigerr = Object.fromEntries(
    reelCreativeBrief({ ...base, keyDropStyle: 'tigerr', keyDropCode: 'tiger' }, PRESET, { status: 'none' }).map(
      (item) => [item.label, item.value],
    ),
  );
  const jcorko = Object.fromEntries(
    reelCreativeBrief({ ...base, keyDropStyle: 'jcorko', keyDropCode: 'jcorko' }, PRESET, { status: 'none' }).map(
      (item) => [item.label, item.value],
    ),
  );
  assert.equal(tigerr['Banner afiliado'], 'KeyDrop · Tigerr · TIGER · de 0,0 s a 4,0 s');
  assert.equal(jcorko['Banner afiliado'], 'KeyDrop · Jcorko · JCORKO · de 0,0 s a 4,0 s');
});

test('an affiliate banner without the user code is not ready and never shows a sample code', () => {
  const edit: EditConfig = {
    format: 'short-9x16', killEffect: 'clean', transition: 'cut', hookText: false, killCounter: false,
    matchRecap: false, voiceComms: false, nativeHud: false, coverStrategy: 'no-cover', intro: false, outro: false,
    keyDropFamily: 'KEYDROP', keyDropStyle: 'operator', keyDropStartSeconds: 0, keyDropEndSeconds: 4,
  };
  const brief = Object.fromEntries(reelCreativeBrief(edit, PRESET, { status: 'none' }).map((item) => [item.label, item.value]));
  assert.equal(brief['Banner afiliado'], 'KeyDrop · Operator · falta tu código · de 0,0 s a 4,0 s');
  assert.equal(missingAffiliateCode(edit), true);
  assert.equal(missingAffiliateCode({ ...edit, keyDropCode: '  ' }), true);
  assert.equal(missingAffiliateCode({ ...edit, keyDropCode: 'mal código' }), true);
  assert.equal(missingAffiliateCode({ ...edit, keyDropCode: 'MICODIGO' }), false);
  assert.equal(missingAffiliateCode({ ...edit, keyDropStyle: '' }), false);
});

test('the brief names the victim replay and its speed', () => {
  const edit: EditConfig = {
    format: 'short-9x16', killEffect: 'clean', transition: 'cut', hookText: false, killCounter: false,
    matchRecap: false, voiceComms: false, nativeHud: false, coverStrategy: 'no-cover', intro: false, outro: false,
    victimReplay: true,
  };
  const replay = (config: EditConfig) =>
    reelCreativeBrief(config, PRESET, { status: 'none' }).find((item) => item.label === 'Repetición')?.value;
  assert.equal(replay(edit), 'Desde la víctima · 0,5x');
  assert.equal(replay({ ...edit, victimReplaySpeed: 0.25 }), 'Desde la víctima · 0,25x');
  assert.equal(replay({ ...edit, victimReplay: false }), 'No');
});

test('creative brief names CSGOSkins when that family is selected', () => {
  const skins = Object.fromEntries(
    reelCreativeBrief(
      {
        format: 'short-9x16',
        killEffect: 'clean',
        transition: 'cut',
        hookText: false,
        killCounter: false,
        matchRecap: false,
        voiceComms: false,
        nativeHud: false,
        coverStrategy: 'no-cover',
        intro: false,
        outro: false,
        keyDropFamily: 'CSGOSKINS',
        keyDropStyle: 'classic',
        keyDropCode: 'skins99',
        keyDropStartSeconds: 0,
        keyDropEndSeconds: 4,
      },
      PRESET,
      { status: 'none' },
    ).map((item) => [item.label, item.value]),
  );
  assert.equal(skins['Banner afiliado'], 'CSGOSkins · Classic · SKINS99 · de 0,0 s a 4,0 s');
  assert.equal(skins.KeyDrop, undefined);
});

test('a death reel drops the centered kill feed from the brief, since its clips have no kills', () => {
  const edit: EditConfig = {
    format: 'short-9x16',
    killEffect: 'clean',
    transition: 'cut',
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'no-cover',
    intro: false,
    outro: false,
    centeredKillfeed: true,
  };
  const hud = (config: EditConfig) =>
    reelCreativeBrief(config, PRESET, { status: 'none' }).find((item) => item.label === 'HUD / killfeed')?.value ?? '';
  assert.match(hud(edit), /kill feed centrada arriba/);
  assert.doesNotMatch(hud({ ...edit, deathReel: true }), /kill feed centrada/);
  assert.equal('centeredKillfeed' in constrainEditConfig({ ...edit, deathReel: true }), false);
});

test('the vertical HUD is sent only over a style without the CS2 HUD and only on a kill Short', () => {
  const edit: EditConfig = {
    format: 'short-9x16',
    killEffect: 'clean',
    transition: 'cut',
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'no-cover',
    intro: false,
    outro: false,
    verticalHud: true,
    verticalHudTheme: 'prism',
  };
  const fullHud: Preset = { ...PRESET, name: 'full-hud-60', label: 'Full HUD', hudMode: 'gameplay' };
  const hud = (config: EditConfig, preset: Preset) =>
    reelCreativeBrief(config, preset, { status: 'none' }).find((item) => item.label === 'HUD / killfeed')?.value ?? '';
  assert.equal(verticalHudAllowed(PRESET), true);
  assert.equal(verticalHudAllowed(null), true);
  assert.equal(verticalHudAllowed(fullHud), false);
  assert.equal(withPresetLimits(edit, PRESET), edit);
  const limited = withPresetLimits(edit, fullHud);
  assert.equal('verticalHud' in limited || 'verticalHudTheme' in limited, false);
  assert.match(hud(edit, PRESET), /HUD vertical Prism con kill feed centrada/);
  assert.doesNotMatch(hud(edit, fullHud), /HUD vertical/);
  for (const config of [constrainEditConfig({ ...edit, deathReel: true }), constrainEditConfig({ ...edit, format: 'landscape-16x9' })]) {
    assert.equal('verticalHud' in config || 'verticalHudTheme' in config, false);
  }
});

test('a style that records CS2 HUD drops the radar from the sent edit and the brief', () => {
  const edit: EditConfig = {
    format: 'short-9x16',
    killEffect: 'clean',
    transition: 'cut',
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'no-cover',
    intro: false,
    outro: false,
    radar: true,
    radarPosition: 'top-right',
    radarSize: 'large',
    radarOpacity: 0.6,
  };
  const fullHud: Preset = { ...PRESET, name: 'full-hud-60', label: 'Full HUD', hudMode: 'gameplay' };
  const radar = (config: EditConfig) =>
    reelCreativeBrief(config, PRESET, { status: 'none' }).find((item) => item.label === 'Radar')?.value;
  assert.equal(withPresetLimits(edit, PRESET), edit);
  const limited = withPresetLimits(edit, fullHud);
  for (const key of ['radar', 'radarPosition', 'radarSize', 'radarOpacity'] as const) assert.equal(key in limited, false);
  assert.equal(radar(edit), 'Arriba a la derecha · Grande · 60 %');
  assert.equal(radar(limited), 'No');
});
