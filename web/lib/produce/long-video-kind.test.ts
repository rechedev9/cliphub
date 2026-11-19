import test from 'node:test';
import assert from 'node:assert/strict';
import { LONG_VIDEOS_HREF } from '../clips/routes.ts';
import { horizontalHref, LONG_VIDEO_KIND, longVideoKind, longVideoKindHref, longVideoKinds, parseHorizontalParam } from './long-video-kind.ts';

const JOB = '0b5c3f9e-7a51-4c1e-9d0a-2f6f8e4b1c7d';

test('one player chooses the whole match or a summary; several players always make a summary', () => {
  assert.deepEqual(longVideoKinds(1), [LONG_VIDEO_KIND.match, LONG_VIDEO_KIND.summary]);
  assert.deepEqual(longVideoKinds(0), [LONG_VIDEO_KIND.match, LONG_VIDEO_KIND.summary], 'an older job without POV players is one player');
  for (const players of [2, 5]) assert.deepEqual(longVideoKinds(players), [LONG_VIDEO_KIND.summary], `${players} players`);
});

test('the URL picks the video kind only when the players allow it', () => {
  assert.equal(longVideoKind(1, undefined), LONG_VIDEO_KIND.match, 'one player opens the whole match as before');
  assert.equal(longVideoKind(1, 'resumen'), LONG_VIDEO_KIND.summary);
  assert.equal(longVideoKind(1, 'otra-cosa'), LONG_VIDEO_KIND.match);
  assert.equal(longVideoKind(1, ['resumen', 'partida']), LONG_VIDEO_KIND.match, 'a repeated key is not trusted');
  assert.equal(longVideoKind(3, 'partida'), LONG_VIDEO_KIND.summary);
});

test('switching the kind keeps the series and the rounds of the URL', () => {
  const query = { series: 's1', rondas: '3,7', video: 'resumen' };
  assert.equal(longVideoKindHref({ jobId: JOB, kind: LONG_VIDEO_KIND.match, query }), `${LONG_VIDEOS_HREF}/${JOB}?series=s1&rondas=3%2C7`);
  assert.equal(longVideoKindHref({ jobId: JOB, kind: LONG_VIDEO_KIND.summary, query: {} }), `${LONG_VIDEOS_HREF}/${JOB}?video=resumen`);
  assert.equal(longVideoKindHref({ jobId: JOB, kind: LONG_VIDEO_KIND.match, query: {} }), `${LONG_VIDEOS_HREF}/${JOB}`);
});

test('Crear horizontal opens the summary producer with the Short it comes from', () => {
  const href = horizontalHref({ jobId: JOB, variant: 'viral-60-clean' });
  assert.equal(href, `${LONG_VIDEOS_HREF}/${JOB}?video=resumen&horizontal=viral-60-clean`);
  const query = Object.fromEntries(new URL(href, 'http://studio').searchParams);
  assert.equal(longVideoKind(3, query.video), LONG_VIDEO_KIND.summary);
  assert.equal(parseHorizontalParam(query), 'viral-60-clean');
  assert.equal(parseHorizontalParam({ horizontal: '../x' }), null);
  assert.equal(parseHorizontalParam({ horizontal: ['a', 'b'] }), null);
  assert.equal(parseHorizontalParam({}), null);
});
