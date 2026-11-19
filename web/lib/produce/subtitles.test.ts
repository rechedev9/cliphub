import assert from 'node:assert/strict';
import test from 'node:test';
import { parseJobSubtitles, playScope, subtitleAvailability, subtitleNote, subtitleRows, toggleHiddenPhrase, type DemoVoiceLine, type JobSubtitles } from './subtitles.ts';

const line = (id: string, patch: Partial<DemoVoiceLine>): DemoVoiceLine => ({
  id, match_id: 'm', steamid64: '7656', name: 'ropz', team_key: 'ct-start', start_ms: 0, end_ms: 1500, tick: 0, round: 1, text: 'uno en B', corrected: false, ...patch,
});
const installed = { state: 'installed', version: '0.3.0', model: 'parakeet', download_bytes: 700 << 20 } as const;
const subs = (patch: Partial<JobSubtitles>): JobSubtitles => ({
  match_id: 'm', state: 'ready', english: 'none', availability: 'available', lines: [], transcriber: installed, translator: { ...installed, state: 'not_installed' }, ...patch,
});

test('the switch stays off with its reason until the video carries voices the demo has', () => {
  const off = subtitleAvailability({ voices: false, voicesLabel: 'Comms del equipo', availability: 'available' });
  assert.equal(off.enabled, false);
  assert.match(off.reason ?? '', /«Comms del equipo»/, 'the reason names the switch that adds the voices');
  const silent = subtitleAvailability({ voices: true, voicesLabel: 'Comms del equipo', availability: 'no_team_voice' });
  assert.equal(silent.enabled, false);
  assert.ok(silent.reason);
  assert.deepEqual(subtitleAvailability({ voices: true, voicesLabel: 'Comms del equipo', availability: 'available' }), { enabled: true, reason: null });
  // Not read yet: the voices are checked when the video is prepared.
  assert.equal(subtitleAvailability({ voices: true, voicesLabel: 'Comms del equipo', availability: null }).enabled, true);
});

test('the panel lists the phrases inside the chosen plays or rounds, in order, with the hidden ones marked', () => {
  const lines = [
    line('b', { start_ms: 40_000, round: 3, text: 'rotad' }),
    line('a', { start_ms: 12_500, round: 2, text: 'uno en B', english: 'one on B' }),
    line('c', { start_ms: 90_000, round: 5, text: 'eco' }),
  ];
  const plays = { kind: 'seconds', windows: [{ start: 10, end: 20 }, { start: 35, end: 45 }] } as const;
  const rows = subtitleRows(lines, plays, { enabled: true, english: false, hidden: ['b'] });
  assert.deepEqual(rows.map((row) => [row.id, row.text, row.hidden]), [['a', 'uno en B', false], ['b', 'rotad', true]]);
  assert.deepEqual(subtitleRows(lines, plays, { enabled: true, english: true, hidden: [] }).map((row) => row.text), ['one on B', '']);
  assert.deepEqual(subtitleRows(lines, { kind: 'rounds', rounds: [5] }, { enabled: true, english: false, hidden: [] }).map((row) => row.id), ['c']);
  assert.equal(subtitleRows(lines, { kind: 'rounds', rounds: null }, { enabled: true, english: false, hidden: [] }).length, 3);
});

test('the chosen plays scope the panel by their demo seconds, or by round when an old analysis lacks them', () => {
  assert.deepEqual(playScope([{ round: 2, startSeconds: 10, endSeconds: 20 }]), { kind: 'seconds', windows: [{ start: 10, end: 20 }] });
  assert.deepEqual(playScope([{ round: 2, startSeconds: 10, endSeconds: 20 }, { round: 4 }, { round: 4 }]), { kind: 'rounds', rounds: [2, 4] });
});

test('hiding a phrase is a choice of this video and can be undone', () => {
  const hidden = toggleHiddenPhrase({ enabled: true, english: false, hidden: ['z'] }, 'a');
  assert.deepEqual(hidden.hidden, ['a', 'z']);
  assert.deepEqual(toggleHiddenPhrase(hidden, 'a').hidden, ['z']);
});

test('the note says what the render will do with the transcript', () => {
  const blocked = [
    subtitleNote(subs({ transcriber: { ...installed, state: 'not_installed' } }), false),
    subtitleNote(subs({ state: 'none' }), false),
    subtitleNote(subs({ state: 'failed', error: 'boom' }), false),
    subtitleNote(subs({}), true),
  ];
  assert.ok(blocked.every((note) => typeof note === 'string'), 'a missing install, transcript or translation is said');
  assert.equal(new Set(blocked).size, blocked.length, 'each blocker has its own note');
  assert.match(subtitleNote(subs({ state: 'running', progress: 0.42 }), false) ?? '', /42 %/);
  assert.equal(subtitleNote(subs({}), false), null);
});

test('only a well-formed answer of the server is read', () => {
  const wire = subs({ lines: [line('a', {})] });
  assert.deepEqual(parseJobSubtitles(JSON.parse(JSON.stringify(wire))), wire);
  assert.equal(parseJobSubtitles({ ...wire, lines: [{ id: 1 }] }), null);
  assert.equal(parseJobSubtitles(null), null);
});
