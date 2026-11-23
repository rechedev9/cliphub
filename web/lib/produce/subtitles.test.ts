import assert from 'node:assert/strict';
import test from 'node:test';
import { parseJobSubtitles, playScope, subtitleAvailability, subtitleNote, subtitleRows, toggleHiddenPhrase, type DemoVoiceLine, type JobSubtitles } from './subtitles.ts';

const line = (id: string, patch: Partial<DemoVoiceLine>): DemoVoiceLine => ({
  id, match_id: 'm', steamid64: '7656', name: 'ropz', team_key: 'ct-start', start_ms: 0, end_ms: 1500, tick: 0, round: 1, text: 'uno en B', corrected: false, discarded: false, ...patch,
});
const installed = { state: 'installed', version: '0.3.0', model: 'parakeet', download_bytes: 700 << 20 } as const;
const subs = (patch: Partial<JobSubtitles>): JobSubtitles => ({
  match_id: 'm', state: 'ready', availability: 'available', lines: [], transcriber: installed, ...patch,
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
    line('a', { start_ms: 12_500, round: 2, text: 'uno en B' }),
    line('c', { start_ms: 90_000, round: 5, text: 'eco' }),
  ];
  const plays = { kind: 'seconds', windows: [{ start: 10, end: 20 }, { start: 35, end: 45 }] } as const;
  const rows = subtitleRows(lines, plays, { enabled: true, hidden: ['b'] });
  assert.deepEqual(rows.map((row) => [row.id, row.text, row.hidden]), [['a', 'uno en B', false], ['b', 'rotad', true]]);
  assert.deepEqual(subtitleRows(lines, { kind: 'rounds', rounds: [5] }, { enabled: true, hidden: [] }).map((row) => row.id), ['c']);
  assert.equal(subtitleRows(lines, { kind: 'rounds', rounds: null }, { enabled: true, hidden: [] }).length, 3);
});

test('a row carries what the transcriber heard beside the correction, and whether the phrase is discarded', () => {
  const lines = [
    line('a', { start_ms: 1000, text: 'rotamos a B', corrected: true, original_text: 'rotamos a ve' }),
    line('b', { start_ms: 2000, text: 'gracias por ver', discarded: true }),
  ];
  const rows = subtitleRows(lines, { kind: 'rounds', rounds: null }, { enabled: true, hidden: [] });
  assert.deepEqual(rows.map((row) => [row.text, row.original, row.discarded]), [['rotamos a B', 'rotamos a ve', false], ['gracias por ver', null, true]]);
  const wire = subs({ lines });
  assert.deepEqual(parseJobSubtitles(JSON.parse(JSON.stringify(wire))), wire);
  assert.equal(parseJobSubtitles({ ...wire, lines: [{ ...lines[0], discarded: undefined }] }), null);
});

test('the chosen plays scope the panel by their demo seconds, or by round when an old analysis lacks them', () => {
  assert.deepEqual(playScope([{ round: 2, startSeconds: 10, endSeconds: 20 }]), { kind: 'seconds', windows: [{ start: 10, end: 20 }] });
  assert.deepEqual(playScope([{ round: 2, startSeconds: 10, endSeconds: 20 }, { round: 4 }, { round: 4 }]), { kind: 'rounds', rounds: [2, 4] });
});

test('hiding a phrase is a choice of this video and can be undone', () => {
  const hidden = toggleHiddenPhrase({ enabled: true, hidden: ['z'] }, 'a');
  assert.deepEqual(hidden.hidden, ['a', 'z']);
  assert.deepEqual(toggleHiddenPhrase(hidden, 'a').hidden, ['z']);
});

test('the note says what the render will do with the transcript', () => {
  const blocked = [
    subtitleNote(subs({ state: 'none', transcriber: { ...installed, state: 'not_installed' } })),
    subtitleNote(subs({ state: 'none' })),
    subtitleNote(subs({ state: 'failed', error: 'boom' })),
  ];
  assert.ok(blocked.every((note) => typeof note === 'string'), 'a missing install or transcript is said');
  assert.equal(new Set(blocked).size, blocked.length, 'each blocker has its own note');
  assert.ok(blocked.every((note) => !/Instala/.test(note ?? '')), 'Studio installs the transcription on its own, it never asks for it');
  const downloading = subtitleNote(subs({ state: 'none', transcriber: { ...installed, state: 'installing' } })) ?? '';
  assert.match(downloading, /el montaje espera/, 'a transcription still downloading holds the render');
  assert.doesNotMatch(downloading, /sin subtítulos/);
  const running = subtitleNote(subs({ state: 'running', progress: 0.42 })) ?? '';
  assert.match(running, /42 %/);
  assert.match(running, /el montaje espera/, 'a running transcription holds the render, it never drops the subtitles');
  assert.doesNotMatch(running, /sin subtítulos/);
  assert.equal(subtitleNote(subs({})), null);
  assert.equal(subtitleNote(subs({ transcriber: { ...installed, state: 'failed' } })), null, 'a done transcript needs no transcriber to render');
});

test('only a well-formed answer of the server is read', () => {
  const wire = subs({ lines: [line('a', {})] });
  assert.deepEqual(parseJobSubtitles(JSON.parse(JSON.stringify(wire))), wire);
  assert.equal(parseJobSubtitles({ ...wire, lines: [{ id: 1 }] }), null);
  assert.equal(parseJobSubtitles(null), null);
});

test('a phrase Studio changed marks the changed words with what was heard', () => {
  const changed = line('a', {
    text: 'molly under palace', original_text: 'moly under Palas', vocab_text: 'molly under palace',
    vocab_changes: [{ heard: 'moly', written: 'molly' }, { heard: 'Palas', written: 'palace' }],
  });
  const [row] = subtitleRows([changed], { kind: 'rounds', rounds: null }, { enabled: true, hidden: [] });
  assert.equal(row?.original, null, 'a change by Studio is not a correction of the user');
  assert.deepEqual(row?.vocab, {
    heard: 'moly under Palas',
    parts: [{ text: 'molly', heard: 'moly' }, { text: ' under ', heard: null }, { text: 'palace', heard: 'Palas' }],
  });
  const wire = subs({ lines: [changed] });
  assert.deepEqual(parseJobSubtitles(JSON.parse(JSON.stringify(wire))), wire);
  assert.equal(parseJobSubtitles({ ...wire, lines: [{ ...changed, vocab_changes: [{ heard: 'moly' }] }] }), null);
});

test('several heard words written as one are marked as that one word', () => {
  const changed = line('a', {
    text: 'two in stairs now', original_text: 'two in wers sirs now', vocab_text: 'two in stairs now',
    vocab_changes: [{ heard: 'wers sirs', written: 'stairs' }],
  });
  const [row] = subtitleRows([changed], { kind: 'rounds', rounds: null }, { enabled: true, hidden: [] });
  assert.deepEqual(row?.vocab?.parts, [{ text: 'two in ', heard: null }, { text: 'stairs', heard: 'wers sirs' }, { text: ' now', heard: null }]);
});

test('a changed word next to punctuation is marked without the punctuation', () => {
  const changed = line('a', {
    text: '¡Palace, molly!', original_text: '¡Palas, moly!', vocab_text: '¡Palace, molly!',
    vocab_changes: [{ heard: 'Palas', written: 'Palace' }, { heard: 'moly', written: 'molly' }],
  });
  const [row] = subtitleRows([changed], { kind: 'rounds', rounds: null }, { enabled: true, hidden: [] });
  assert.deepEqual(row?.vocab?.parts, [
    { text: '¡', heard: null }, { text: 'Palace', heard: 'Palas' }, { text: ', ', heard: null }, { text: 'molly', heard: 'moly' }, { text: '!', heard: null },
  ]);
});

test('after the undo or a correction by hand the phrase shows no change by Studio', () => {
  const vocab = { vocab_text: 'molly under palace', vocab_changes: [{ heard: 'moly', written: 'molly' }] };
  const undone = line('a', { text: 'moly under Palas', corrected: true, ...vocab });
  const edited = line('b', { start_ms: 1, text: 'molly under palacio', corrected: true, original_text: 'moly under Palas', ...vocab });
  const rows = subtitleRows([undone, edited], { kind: 'rounds', rounds: null }, { enabled: true, hidden: [] });
  assert.deepEqual(rows.map((row) => [row.text, row.original, row.vocab]), [
    ['moly under Palas', null, null],
    ['molly under palacio', 'moly under Palas', null],
  ]);
});
