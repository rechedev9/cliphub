import test from 'node:test';
import assert from 'node:assert/strict';
import {
  DEFAULT_PRODUCE_FORMAT,
  isJobIdParam,
  NEW_DEMO_HREF,
  newDemoHref,
  PRODUCE_FORMAT,
  produceFormatParam,
  produceHref,
} from './routes.ts';

const JOB_ID = '9a1c6e2f-4b3d-4a10-8f2e-1d6c7b9a0e55';
const SERIES_ID = 'series-1';

test('newDemoHref builds the fresh-upload and resume hrefs', () => {
  const cases = [
    { name: 'short intent survives upload', opts: { format: PRODUCE_FORMAT.short }, want: `${NEW_DEMO_HREF}?formato=short` },
    { name: 'long video intent survives a resumed roster', opts: { job: JOB_ID, format: PRODUCE_FORMAT.full }, want: `${NEW_DEMO_HREF}?job=${JOB_ID}&formato=full` },
    { name: 'no job: the plain upload page', opts: {}, want: NEW_DEMO_HREF },
    { name: 'a job id resumes that scan', opts: { job: JOB_ID }, want: `${NEW_DEMO_HREF}?job=${JOB_ID}` },
  ];
  for (const { name, opts, want } of cases) {
    assert.equal(newDemoHref(opts), want, name);
  }
});

test('the long video is the default produce format', () => {
  assert.equal(DEFAULT_PRODUCE_FORMAT, PRODUCE_FORMAT.full);
});

test('produceHref omits the query only for the default format', () => {
  const base = `/clips/${JOB_ID}/nuevo`;
  const cases = [
    { name: 'implicit format opens the long video', href: produceHref(JOB_ID), want: base },
    { name: 'explicit long video needs no query', href: produceHref(JOB_ID, PRODUCE_FORMAT.full), want: base },
    { name: 'explicit Short carries the format', href: produceHref(JOB_ID, PRODUCE_FORMAT.short), want: `${base}?formato=short` },
    { name: 'series Short keeps both params', href: produceHref(JOB_ID, PRODUCE_FORMAT.short, SERIES_ID), want: `${base}?formato=short&series=${SERIES_ID}` },
    { name: 'empty series id is dropped', href: produceHref(JOB_ID, PRODUCE_FORMAT.full, ''), want: base },
  ];
  for (const { name, href, want } of cases) {
    assert.equal(href, want, name);
  }
});

test('produceFormatParam falls back to the long video', () => {
  const cases: Array<{ name: string; value: string | string[] | null | undefined; want: string }> = [
    { name: 'missing param', value: undefined, want: PRODUCE_FORMAT.full },
    { name: 'null from URLSearchParams', value: null, want: PRODUCE_FORMAT.full },
    { name: 'unknown value', value: 'vertical', want: PRODUCE_FORMAT.full },
    { name: 'repeated param', value: [PRODUCE_FORMAT.short, PRODUCE_FORMAT.short], want: PRODUCE_FORMAT.full },
    { name: 'explicit short', value: PRODUCE_FORMAT.short, want: PRODUCE_FORMAT.short },
    { name: 'explicit full', value: PRODUCE_FORMAT.full, want: PRODUCE_FORMAT.full },
  ];
  for (const { name, value, want } of cases) {
    assert.equal(produceFormatParam(value), want, name);
  }
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
