import test from 'node:test';
import assert from 'node:assert/strict';
import { withPreRoll } from './pre-roll.ts';
import { withSummaryStyle } from './summary-style.ts';
import {
  horizontalCtaLabel,
  horizontalNeedsCapture,
  horizontalNotice,
  projectHorizontalDraft,
  withShortFraming,
  type HorizontalDraft,
} from './horizontal.ts';

const BODY = {
  revision: '7c0c1d4e-8f7a-4f64-9a51-0d2f5a3c9b11',
  created_at: '2026-11-18T14:05:00Z',
  preset: 'gameplay-pov-60',
  segment_ids: ['seg-002', 'seg-001', 'seg-004'],
  edit: {
    format: 'landscape-16x9', killEffect: 'glitch', transition: 'whip', intro: true, intro_text: 'ACE en Mirage', outro: false, hook_text: false,
    kill_counter: false, match_recap: false, voice_comms: true, native_hud: false, cover_strategy: 'generated-gameplay', play_badges: true,
    pre_roll_seconds: 5, capture_hud: 'deathnotices', portrait_safe_killfeed: true,
  },
  music: { key: 'song-1', volume: 0.6, game_volume: 0.4 },
  dropped: ['vertical_hud', 'stats_card', 'radar', 'something-new'],
  capture: { needed: false, missing_segment_ids: [] },
};

function draftOf(patch: Record<string, unknown> = {}): HorizontalDraft {
  const draft = projectHorizontalDraft({ ...BODY, ...patch });
  assert.ok(draft, 'the server draft parses');
  return draft;
}

test('Crear horizontal opens the summary with the Short plays in their order, its style, pre-roll and music', () => {
  const { settings, dropped } = draftOf();
  assert.deepEqual(settings.selectedIds, ['seg-002', 'seg-001', 'seg-004']);
  assert.equal(settings.variant, 'gameplay-pov-60');
  assert.deepEqual([settings.songId, settings.musicVolume, settings.gameVolume, settings.musicDecided], ['song-1', 60, 40, true]);
  const edit = settings.editConfig;
  assert.deepEqual([edit.format, edit.captureHud, edit.portraitSafeKillfeed, edit.preRollSeconds], ['landscape-16x9', 'deathnotices', true, 5]);
  assert.deepEqual([edit.killEffect, edit.transition, edit.intro, edit.introText, edit.playBadges], ['glitch', 'whip', true, 'ACE en Mirage', true]);
  assert.deepEqual(dropped, ['HUD vertical', 'tarjeta de estadísticas', 'radar de ClipHub']);
});

test('the horizontal names the player cards the Short had', () => {
  assert.deepEqual(draftOf({ dropped: ['player_cards'] }).dropped, ['cartas de jugador']);
});

test('a draft without plays or a 16:9 edit is refused', () => {
  assert.equal(projectHorizontalDraft({ ...BODY, segment_ids: [] }), null);
  assert.equal(projectHorizontalDraft({ ...BODY, edit: { ...BODY.edit, format: 'short-9x16' } }), null);
  assert.equal(projectHorizontalDraft({ ...BODY, preset: '../x' }), null);
});

test('the horizontal records only when the Short clips cannot be reused', () => {
  const draft = draftOf({ capture: { needed: true, missing_segment_ids: ['seg-004'] } });
  const edit = draft.settings.editConfig;
  const choice = (playIds: string[], next = edit) => ({ playIds, hudMode: 'gameplay', edit: withShortFraming(next, draft) });
  assert.equal(horizontalNeedsCapture(draft, choice(['seg-002', 'seg-001'])), false, 'the recorded plays');
  assert.equal(horizontalNeedsCapture(draft, choice(['seg-002', 'seg-001', 'seg-004'])), true, 'a play whose clip is gone');
  assert.equal(horizontalNeedsCapture(draft, choice(['seg-001', 'seg-003'])), true, 'a new play');
  assert.equal(horizontalNeedsCapture(draft, choice(['seg-001'], withSummaryStyle(edit, 'gameplay'))), true, 'HUD completo');
  assert.equal(horizontalNeedsCapture(draft, choice(['seg-001'], withPreRoll(edit, 8))), true, 'another pre-roll');
  const back = withSummaryStyle(withSummaryStyle(edit, 'gameplay'), 'deathnotices');
  assert.equal(horizontalNeedsCapture(draft, choice(['seg-001'], back)), false, 'back to the Short style');
});

test('the producer says whether the horizontal is only rendered or recorded too', () => {
  assert.equal(horizontalCtaLabel(false), 'Montar el horizontal');
  assert.equal(horizontalCtaLabel(true), 'Grabar y montar el horizontal');
  assert.equal(horizontalCtaLabel(null), 'Crear resumen');
  const draft = draftOf({ capture: { needed: true, missing_segment_ids: ['seg-004'] } });
  const intro = 'Creas el horizontal del Short del ';
  assert.ok(horizontalNotice({ draft, needsCapture: false, playIds: ['seg-001'] }).startsWith(intro));
  assert.match(horizontalNotice({ draft, needsCapture: false, playIds: ['seg-001'] }), /mismo orden.*El Short no cambia\. Se monta con lo ya grabado, sin abrir CS2\.$/);
  assert.match(horizontalNotice({ draft, needsCapture: true, playIds: ['seg-001', 'seg-004'] }), /La grabación de 1 jugada del Short ya no está en este PC: se graba otra vez con CS2\.$/);
  assert.match(horizontalNotice({ draft: draftOf({ capture: { needed: true, missing_segment_ids: ['seg-001', 'seg-002', 'seg-004'] } }), needsCapture: true, playIds: ['seg-001', 'seg-002', 'seg-004'] }),
    /Las grabaciones del Short ya no están en este PC: se graban otra vez con CS2\.$/);
  assert.match(horizontalNotice({ draft, needsCapture: true, playIds: ['seg-003'] }), /Estos cambios necesitan grabar con CS2\.$/);
  assert.match(horizontalNotice({ draft, needsCapture: true, playIds: ['seg-003'] }), /El horizontal no lleva estas opciones del vertical: HUD vertical, tarjeta de estadísticas y radar de ClipHub\./);
});
