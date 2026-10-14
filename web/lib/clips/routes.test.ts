import test from 'node:test';
import assert from 'node:assert/strict';
import {
  flowHref,
  isJobIdParam,
  LONG_VIDEOS_HREF,
  PRODUCE_FORMAT,
  produceHref,
  publishHref,
  RESULTS_ORIGIN,
  resultsHref,
  resultsOriginParam,
  SHORTS_HREF,
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

test('resultsHref filters by where the video was recorded', () => {
  assert.equal(resultsHref(), '/resultados');
  assert.equal(resultsHref({ origin: RESULTS_ORIGIN.cloud }), '/resultados?origen=nube');
  assert.equal(resultsHref({ origin: RESULTS_ORIGIN.local }), '/resultados?origen=pc');
});

test('resultsOriginParam shows everything unless the origin is known', () => {
  const cases: Array<{ name: string; value: string | string[] | null | undefined; want: string | null }> = [
    { name: 'missing param', value: undefined, want: null },
    { name: 'null from URLSearchParams', value: null, want: null },
    { name: 'unknown value', value: 'servidor', want: null },
    { name: 'repeated param', value: [RESULTS_ORIGIN.cloud, RESULTS_ORIGIN.cloud], want: null },
    { name: 'this PC', value: RESULTS_ORIGIN.local, want: RESULTS_ORIGIN.local },
    { name: 'cloud', value: RESULTS_ORIGIN.cloud, want: RESULTS_ORIGIN.cloud },
  ];
  for (const { name, value, want } of cases) {
    assert.equal(resultsOriginParam(value), want, name);
  }
});

test('publishHref lives under the results area', () => {
  assert.equal(publishHref(JOB_ID, 'clip 1'), `/resultados/${JOB_ID}/publicar/clip%201`);
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
