import assert from 'node:assert/strict';
import test from 'node:test';
import {
  canForgeReel,
  canRerenderWithMusic,
  constrainEditConfig,
  isLandscapeRecap,
  musicBriefValue,
  reelCreativeBrief,
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

test('library music rerender stays blocked until the mix changes and the brief is approved', () => {
  const ready = { briefApproved: true, busy: false, musicChanged: true };
  assert.equal(canRerenderWithMusic(ready), true);
  assert.equal(canRerenderWithMusic({ ...ready, briefApproved: false }), false);
  assert.equal(canRerenderWithMusic({ ...ready, busy: true }), false);
  assert.equal(canRerenderWithMusic({ ...ready, musicChanged: false }), false);
});

test('forging stays blocked until the exact brief is approved', () => {
  const ready = { briefApproved: true, creating: false, hasPreset: true, selectionCount: 1, musicDecided: true };
  assert.equal(canForgeReel(ready), true);
  assert.equal(canForgeReel({ ...ready, briefApproved: false }), false);
  assert.equal(canForgeReel({ ...ready, creating: true }), false);
  assert.equal(canForgeReel({ ...ready, hasPreset: false }), false);
  assert.equal(canForgeReel({ ...ready, selectionCount: 0 }), false);
  assert.equal(canForgeReel({ ...ready, musicDecided: false }), false);
});

test('music brief distinguishes pending from an explicit no-music choice', () => {
  assert.equal(musicBriefValue({ status: 'pending' }), 'Pendiente de decisión');
  assert.equal(musicBriefValue({ status: 'none' }), 'Sin música');
  assert.equal(
    musicBriefValue({ status: 'track', title: 'Tema CC0', volumePercent: 35, gameVolumePercent: 20 }),
    'Tema CC0 · música 35% · juego 20%',
  );
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
    { label: 'Efecto de kill', value: 'Impacto / punch-in' },
    { label: 'Transición', value: 'Destello' },
    { label: 'Título / contador', value: 'Título automático · Contador activado' },
    { label: 'Intro', value: 'Sí · “Entrada”' },
    { label: 'Outro', value: 'No' },
    { label: 'KeyDrop', value: 'No' },
    { label: 'Música', value: 'Tema CC0 · música 35% · juego 20%' },
    { label: 'Portada', value: 'Generar candidatos de gameplay para revisión' },
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
  assert.equal(brief['HUD / killfeed'], 'Pendiente de preset');
  assert.equal(brief['Título / contador'], 'Sin título automático · Sin contador');
  assert.equal(brief['Intro'], 'No');
  assert.equal(brief['Outro'], 'Sí · firma ClipHub');
  assert.equal(brief['KeyDrop'], 'No');
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
  assert.equal(brief['Comms'], 'Mezclar comms del equipo · 85%');
  assert.equal(brief['HUD / killfeed'], NATIVE_HUD_LABEL);
  assert.equal(isLandscapeRecap(edit), true);
  assert.equal(constrainEditConfig(edit), edit);
});

test('full-demo locked edit names native CS2 HUD, not Shorts full-hud copy', () => {
  const brief = Object.fromEntries(
    reelCreativeBrief(FULL_DEMO_EDIT, FULL_DEMO_PRESET, { status: 'none' }).map((item) => [item.label, item.value]),
  );
  assert.equal(brief['HUD / killfeed'], NATIVE_HUD_LABEL);
  assert.notEqual(brief['HUD / killfeed'], 'HUD completo con killfeed');
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

test('creative brief names Tigerr and Jcorko KeyDrop styles', () => {
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
  assert.equal(tigerr.KeyDrop, 'Tigerr · TIGER · 0.0s–4.0s');
  assert.equal(jcorko.KeyDrop, 'Jcorko · JCORKO · 0.0s–4.0s');
});
