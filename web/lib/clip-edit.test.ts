import test from 'node:test';
import assert from 'node:assert/strict';
import {
  clipEditIssue,
  streamRangeIssue,
  streamRangeOverlapIssue,
  streamRangesIssue,
  withoutEmptyTextOverlays,
} from './clip-edit.ts';
import type { StreamClipRange } from './api/streams.ts';

function clip(edit: StreamClipRange['edit']): StreamClipRange[] {
  return [{ id: 'clip-1', start_seconds: 0, end_seconds: 10, edit }];
}

test('clips without edits or with valid edits pass', () => {
  assert.equal(clipEditIssue(clip(undefined)), null);
  assert.equal(
    clipEditIssue(
      clip({
        speed: 2,
        fade_in_seconds: 0.5,
        fade_out_seconds: 1,
        text_overlays: [{ text: 'GG', position_y: 0.5, start_seconds: 1, end_seconds: 3, font_size: 72 }],
      }),
    ),
    null,
  );
});

test('empty overlay text is reported with the clip label', () => {
  const issue = clipEditIssue(clip({ text_overlays: [{ text: '   ', position_y: 0.5 }] }));
  assert.match(issue ?? '', /Clip 1: hay un texto en pantalla vacío/);
});

test('fractional or out-of-range font size is rejected before the PUT', () => {
  assert.match(clipEditIssue(clip({ text_overlays: [{ text: 'GG', position_y: 0.5, font_size: 64.5 }] })) ?? '', /entero entre 24 y 120/);
  assert.match(clipEditIssue(clip({ text_overlays: [{ text: 'GG', position_y: 0.5, font_size: 12 }] })) ?? '', /entero entre 24 y 120/);
});

test('fades must fit the sped-up output duration', () => {
  // 10s source at 2.5x plays back in 4s; 4.5s of fades cannot fit.
  const issue = clipEditIssue(clip({ speed: 2.5, fade_in_seconds: 2.5, fade_out_seconds: 2 }));
  assert.match(issue ?? '', /los fundidos no caben/);
});

test('overlay windows must stay inside the clip and be ordered', () => {
  assert.match(clipEditIssue(clip({ text_overlays: [{ text: 'GG', position_y: 0.5, start_seconds: 12 }] })) ?? '', /inicio de un texto/);
  assert.match(clipEditIssue(clip({ text_overlays: [{ text: 'GG', position_y: 0.5, end_seconds: 11 }] })) ?? '', /fin de un texto/);
  assert.match(
    clipEditIssue(clip({ text_overlays: [{ text: 'GG', position_y: 0.5, start_seconds: 3, end_seconds: 1 }] })) ?? '',
    /termina antes de empezar/,
  );
});

test('range validation reports source bounds immediately in Spanish', () => {
  assert.match(streamRangeIssue({ id: 'clip-1', start_seconds: 2, end_seconds: 1 }, 15.112, 0) ?? '', /fin debe ser posterior/);
  assert.match(streamRangeIssue({ id: 'clip-1', start_seconds: 0, end_seconds: 20 }, 15.112, 0) ?? '', /15\.11 s/);
  assert.equal(streamRangeIssue({ id: 'clip-1', start_seconds: 0, end_seconds: 15.112 }, 15.112, 0), null);
});

const overlapCases: [string, StreamClipRange[], boolean][] = [
  ['disjoint cuts do not overlap', [
    { id: 'c1', start_seconds: 0, end_seconds: 5 },
    { id: 'c2', start_seconds: 5, end_seconds: 10 },
  ], false],
  ['adjacent-but-touching cuts do not overlap', [
    { id: 'c1', start_seconds: 0, end_seconds: 5 },
    { id: 'c2', start_seconds: 5, end_seconds: 10 },
  ], false],
  ['overlapping cuts are reported', [
    { id: 'c1', start_seconds: 0, end_seconds: 6 },
    { id: 'c2', start_seconds: 5, end_seconds: 10 },
  ], true],
  ['one cut fully inside another is reported', [
    { id: 'c1', start_seconds: 0, end_seconds: 10 },
    { id: 'c2', start_seconds: 2, end_seconds: 4 },
  ], true],
  ['out-of-order but non-overlapping cuts pass', [
    { id: 'c1', start_seconds: 5, end_seconds: 10 },
    { id: 'c2', start_seconds: 0, end_seconds: 5 },
  ], false],
];

test('streamRangesIssue reports overlapping cuts', () => {
  for (const [name, clips, wantsIssue] of overlapCases) {
    const issue = streamRangesIssue(clips, 0);
    if (wantsIssue) {
      assert.match(issue ?? '', /se solapan en la fuente/, name);
    } else {
      assert.equal(issue, null, name);
    }
  }
});

test('streamRangeOverlapIssue flags the card that overlaps another cut', () => {
  const clips: StreamClipRange[] = [
    { id: 'c1', start_seconds: 0, end_seconds: 6, title: 'Ace' },
    { id: 'c2', start_seconds: 5, end_seconds: 10 },
  ];
  assert.match(streamRangeOverlapIssue(clips, 0) ?? '', /Se solapa con Clip 2/);
  assert.match(streamRangeOverlapIssue(clips, 1) ?? '', /Se solapa con Ace/);
  assert.equal(
    streamRangeOverlapIssue([{ id: 'c1', start_seconds: 0, end_seconds: 5 }, { id: 'c2', start_seconds: 5, end_seconds: 10 }], 0),
    null,
  );
});

test('withoutEmptyTextOverlays drops blank-text overlays but keeps the rest', () => {
  assert.deepEqual(
    withoutEmptyTextOverlays([
      { text: 'GG', position_y: 0.5 },
      { text: '   ', position_y: 0.5 },
      { text: '', position_y: 0.5 },
    ]),
    [{ text: 'GG', position_y: 0.5 }],
  );
  assert.deepEqual(withoutEmptyTextOverlays([]), []);
});
