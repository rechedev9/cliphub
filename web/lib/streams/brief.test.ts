import test from 'node:test';
import assert from 'node:assert/strict';
import { streamCreativeBrief } from './brief.ts';
import { EDIT_PLAN_SCHEMA_VERSION } from './plan.ts';
import type { StreamEditPlan } from '../api/streams.ts';

function plan(overrides: Partial<StreamEditPlan> = {}): StreamEditPlan {
  return {
    schema_version: EDIT_PLAN_SCHEMA_VERSION,
    variant: 'streamer-vertical-stack-40-60',
    face_crop_reviewed: true,
    clips: [{ id: 'c1', start_seconds: 0, end_seconds: 10, title: 'Ace' }],
    music: { key: 'phonk-01', volume: 0.25 },
    effects: { grade: true },
    streamer_banner: { nick: 'pro_player', slide_enabled: true },
    ...overrides,
  };
}

test('stream creative brief lists every production decision', () => {
  const items = streamCreativeBrief(plan());
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel.Formato, 'Cámara grande');
  assert.equal(byLabel['Cámara'], 'Recorte confirmado');
  assert.match(byLabel.Shorts, /1 Short/);
  assert.equal(byLabel['Banner del streamer'], 'pro_player · Twitch · deslizante');
  assert.equal(byLabel.Afiliado, 'No');
  assert.equal(byLabel.Música, 'phonk-01 · 25%');
  assert.equal(byLabel['Realce de color'], 'Sí');
});

test('the brief names the catalog track and reads the affiliate window without dashes', () => {
  const items = streamCreativeBrief(
    plan({ keydrop_banner: { style: 'classic', code: 'zackcsgo', start_seconds: 0, end_seconds: 4 } }),
    'Night Drive',
  );
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel['Música'], 'Night Drive · 25%');
  assert.equal(byLabel.Afiliado, 'KeyDrop · Classic · ZACKCSGO · de 0.0 s a 4.0 s');
});

test('stream creative brief names Kick when the banner platform is kick', () => {
  const items = streamCreativeBrief(plan({ streamer_banner: { nick: 'aimagia', platform: 'kick' } }));
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel['Banner del streamer'], 'aimagia · Kick');
});

test('stream creative brief lists KeyDrop when enabled', () => {
  const items = streamCreativeBrief(
    plan({
      keydrop_banner: { style: 'classic', code: 'zackcsgo', slide_enabled: true },
    }),
  );
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel.Afiliado, 'KeyDrop · Classic · ZACKCSGO · deslizante');
});

test('stream creative brief names Tigerr and Jcorko KeyDrop styles', () => {
  const tigerr = Object.fromEntries(
    streamCreativeBrief(plan({ keydrop_banner: { style: 'tigerr', code: 'tiger' } })).map((item) => [
      item.label,
      item.value,
    ]),
  );
  const jcorko = Object.fromEntries(
    streamCreativeBrief(plan({ keydrop_banner: { style: 'jcorko', code: 'jcorko' } })).map((item) => [
      item.label,
      item.value,
    ]),
  );
  assert.equal(tigerr.Afiliado, 'KeyDrop · Tigerr · TIGER');
  assert.equal(jcorko.Afiliado, 'KeyDrop · Jcorko · JCORKO');
});

test('stream creative brief names CSGOSkins when that family is selected', () => {
  const items = streamCreativeBrief(
    plan({
      keydrop_banner: { family: 'CSGOSKINS', style: 'classic', code: 'skins99' },
    }),
  );
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel.Afiliado, 'CSGOSkins · Classic · SKINS99');
  assert.equal(byLabel.KeyDrop, undefined);
});

test('stream creative brief marks unreviewed facecam and empty music', () => {
  const items = streamCreativeBrief(
    plan({
      face_crop_reviewed: false,
      music: { key: '', volume: 0 },
      effects: { grade: false },
      streamer_banner: { nick: '' },
      variant: 'streamer-fullframe-nocam',
    }),
  );
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel['Cámara'], 'Sin cámara');
  assert.equal(byLabel.Música, 'Sin música');
  assert.equal(byLabel['Realce de color'], 'No');
  assert.equal(byLabel['Banner del streamer'], 'Sin banner');
});


test('the clip summary reads as a clock, never a unit glued to digits', () => {
  const items = streamCreativeBrief(plan());
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel.Shorts, '1 Short · 0:10 en total');
  assert.doesNotMatch(byLabel.Shorts, /\ds en total/);
});
