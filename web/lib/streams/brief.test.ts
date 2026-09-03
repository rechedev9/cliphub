import test from 'node:test';
import assert from 'node:assert/strict';
import { canCreateStreamShorts, streamCreativeBrief, streamCreativeBriefLine } from './brief.ts';
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
  assert.equal(byLabel.Layout, 'Facecam 40');
  assert.equal(byLabel.Facecam, 'Recorte confirmado');
  assert.match(byLabel.Clips, /1 clip/);
  assert.equal(byLabel.Banner, 'pro_player · Twitch · slide');
  assert.equal(byLabel.Afiliado, 'No');
  assert.equal(byLabel.Música, 'phonk-01 · 25%');
  assert.equal(byLabel.Grade, 'Sí');
});

test('stream creative brief names Kick when the banner platform is kick', () => {
  const items = streamCreativeBrief(plan({ streamer_banner: { nick: 'aimagia', platform: 'kick' } }));
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel.Banner, 'aimagia · Kick');
});

test('stream creative brief lists KeyDrop when enabled', () => {
  const items = streamCreativeBrief(
    plan({
      keydrop_banner: { style: 'classic', code: 'zackcsgo', slide_enabled: true },
    }),
  );
  const byLabel = Object.fromEntries(items.map((item) => [item.label, item.value]));
  assert.equal(byLabel.Afiliado, 'KeyDrop · Classic · ZACKCSGO · slide');
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
  assert.equal(byLabel.Facecam, 'Sin facecam');
  assert.equal(byLabel.Música, 'Sin música');
  assert.equal(byLabel.Grade, 'No');
  assert.equal(byLabel.Banner, 'Sin nick');
});

test('stream shorts require brief approval', () => {
  assert.equal(canCreateStreamShorts({ briefApproved: true, busy: false }), true);
  assert.equal(canCreateStreamShorts({ briefApproved: false, busy: false }), false);
  assert.equal(canCreateStreamShorts({ briefApproved: true, busy: true }), false);
});

test('the brief line joins every decision with a middle dot', () => {
  assert.equal(
    streamCreativeBriefLine(plan()),
    'Facecam 40 · Recorte confirmado · 1 clip · ~10.0s salida · pro_player · Twitch · slide · No · phonk-01 · 25% · Sí',
  );
});
