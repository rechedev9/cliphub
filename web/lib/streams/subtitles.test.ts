import test from 'node:test';
import assert from 'node:assert/strict';
import {
  parseStreamTranscript, streamSubtitleNote, streamSubtitleRows, streamSubtitlesStatus, stretchesKey,
  type StreamTranscript, type StreamVoiceLine,
} from './subtitles.ts';

function line(id: string, startMs: number, extra: Partial<StreamVoiceLine> = {}): StreamVoiceLine {
  return { id, start_ms: startMs, end_ms: startMs + 1000, text: `frase ${id}`, corrected: false, discarded: false, ...extra };
}

function transcript(extra: Partial<StreamTranscript> = {}): StreamTranscript {
  return {
    state: 'ready', availability: 'available', lines: [], spans: [], pending: [],
    transcriber: { state: 'installed', version: '0.3.0', model: 'parakeet', download_bytes: 700 << 20 },
    subtitles: { shorts: false, summary: false, hidden: {} },
    ...extra,
  };
}

test('a video lists the frases said inside its stretches, in the order they were said', () => {
  const lines = [line('late', 125_000), line('out', 90_000), line('early', 61_000, { corrected: true, original_text: 'frase erly' })];
  const rows = streamSubtitleRows({ lines, windows: [{ start: 120, end: 130 }, { start: 60, end: 75 }], hidden: ['late'] });
  assert.deepEqual(rows.map((row) => [row.id, row.clock, row.hidden, row.original]), [
    ['early', '1:01', false, 'frase erly'],
    ['late', '2:05', true, null],
  ]);
});

test('a stream with only a mixed track says subtítulos are not available for it yet', () => {
  const status = streamSubtitlesStatus('mixed_track');
  assert.equal(status.enabled, false);
  assert.match(status.reason ?? '', /aún no están disponibles/);
  assert.equal(streamSubtitlesStatus('no_audio').enabled, false);
  assert.deepEqual(streamSubtitlesStatus('available'), { enabled: true, reason: null });
});

test('the note says what is happening with the transcripción', () => {
  assert.match(streamSubtitleNote(transcript({ transcriber: { state: 'not_installed', version: '', model: '', download_bytes: 1 } })) ?? '', /Instala la transcripción/);
  assert.match(streamSubtitleNote(transcript({ state: 'running', progress: 0.42 })) ?? '', /42 %/);
  assert.match(streamSubtitleNote(transcript({ state: 'failed' })) ?? '', /falló/);
  assert.equal(streamSubtitleNote(transcript()), null);
});

test('the same stretches in another order are the same request', () => {
  assert.equal(stretchesKey([{ start: 60, end: 75 }, { start: 10, end: 20 }]), stretchesKey([{ start: 10, end: 20 }, { start: 60, end: 75 }]));
  assert.notEqual(stretchesKey([{ start: 10, end: 20 }]), stretchesKey([{ start: 10, end: 21 }]));
});

test('an answer this build does not understand is refused', () => {
  const good = transcript({ lines: [line('a', 1000)], subtitles: { shorts: true, summary: false, hidden: { 'clip-1': ['a'] } } });
  assert.deepEqual(parseStreamTranscript(JSON.parse(JSON.stringify({ ...good, job_id: 'x', updated_at: 'now' }))), good);
  assert.equal(parseStreamTranscript({ ...good, lines: [{ id: 'a' }] }), null);
  assert.equal(parseStreamTranscript({ ...good, availability: 7 }), null);
});
