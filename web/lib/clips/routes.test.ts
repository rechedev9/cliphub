import test from 'node:test';
import assert from 'node:assert/strict';
import {
  flowHref,
  isJobIdParam,
  LONG_VIDEOS_HREF,
  matchHref,
  matchesHref,
  parseRoundsParam,
  playerHref,
  PRODUCE_FORMAT,
  produceHref,
  publishHref,
  roundsProduceHref,
  SHORTS_HREF,
  teamHref,
} from './routes.ts';

const JOB_ID = '9a1c6e2f-4b3d-4a10-8f2e-1d6c7b9a0e55';
const SERIES_ID = 'series-1';

test('flowHref keeps each format in its own flow', () => {
  const cases = [
    { name: 'Short home', href: flowHref(PRODUCE_FORMAT.short), want: SHORTS_HREF },
    { name: 'long video home', href: flowHref(PRODUCE_FORMAT.full), want: LONG_VIDEOS_HREF },
    { name: 'a job id resumes that scan in the Short flow', href: flowHref(PRODUCE_FORMAT.short, { job: JOB_ID }), want: `${SHORTS_HREF}?job=${JOB_ID}` },
    { name: 'a job id resumes that scan in the long flow', href: flowHref(PRODUCE_FORMAT.full, { job: JOB_ID }), want: `${LONG_VIDEOS_HREF}?job=${JOB_ID}` },
  ];
  for (const { name, href, want } of cases) {
    assert.equal(href, want, name);
  }
});

test('produceHref opens the producer of the given flow', () => {
  const cases = [
    { name: 'Short producer', href: produceHref(JOB_ID, PRODUCE_FORMAT.short), want: `${SHORTS_HREF}/${JOB_ID}` },
    { name: 'long video producer', href: produceHref(JOB_ID, PRODUCE_FORMAT.full), want: `${LONG_VIDEOS_HREF}/${JOB_ID}` },
    { name: 'series Short keeps the series', href: produceHref(JOB_ID, PRODUCE_FORMAT.short, SERIES_ID), want: `${SHORTS_HREF}/${JOB_ID}?series=${SERIES_ID}` },
    { name: 'empty series id is dropped', href: produceHref(JOB_ID, PRODUCE_FORMAT.full, ''), want: `${LONG_VIDEOS_HREF}/${JOB_ID}` },
  ];
  for (const { name, href, want } of cases) {
    assert.equal(href, want, name);
  }
});

test('publishHref lives under the results area', () => {
  assert.equal(publishHref(JOB_ID, 'clip 1'), `/mis-videos/${JOB_ID}/publicar/clip%201`);
});

test('matchHref opens the partida record under Partidas', () => {
  assert.equal(matchHref(JOB_ID), `/partidas/${JOB_ID}`);
  assert.equal(matchHref('a/b'), '/partidas/a%2Fb');
  assert.equal(teamHref(JOB_ID), `/equipos/${JOB_ID}`);
  assert.equal(matchesHref(), '/partidas');
  assert.equal(matchesHref({ team: JOB_ID, map: 'de_mirage' }), `/partidas?team=${JOB_ID}&map=de_mirage`);
  assert.equal(playerHref('76561198000000001'), '/jugadores/76561198000000001');
});

test('isJobIdParam accepts only a well-formed job id', () => {
  const cases: Array<{ name: string; value: string | string[] | null | undefined; want: boolean }> = [
    { name: 'canonical lowercase uuid', value: JOB_ID, want: true },
    { name: 'uppercase uuid', value: JOB_ID.toUpperCase(), want: true },
    { name: 'missing param', value: null, want: false },
    { name: 'undefined param', value: undefined, want: false },
    { name: 'repeated param', value: [JOB_ID, JOB_ID], want: false },
    { name: 'empty string', value: '', want: false },
    { name: 'mock id', value: 'm-upload-1', want: false },
    { name: 'path traversal', value: '../jobs', want: false },
    { name: 'uuid with a trailing segment', value: `${JOB_ID}/roster`, want: false },
  ];
  for (const { name, value, want } of cases) {
    assert.equal(isJobIdParam(value), want, name);
  }
});

test('a round selection travels as ?rondas= and only clean round lists are read back', () => {
  assert.equal(roundsProduceHref(JOB_ID, PRODUCE_FORMAT.full, [12, 3]), `${LONG_VIDEOS_HREF}/${JOB_ID}?rondas=3%2C12`);
  assert.equal(roundsProduceHref(JOB_ID, PRODUCE_FORMAT.short, []), `${SHORTS_HREF}/${JOB_ID}`);
  assert.deepEqual(parseRoundsParam('12,3,3'), [3, 12]);
  for (const bad of ['', '0', '3,x', '1000', '-1', undefined]) assert.equal(parseRoundsParam(bad), null);
  assert.equal(parseRoundsParam(['3']), null);
  assert.equal(parseRoundsParam(Array.from({ length: 61 }, (_, i) => i + 1).join(',')), null);
});
