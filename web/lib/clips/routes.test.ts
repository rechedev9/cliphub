import test from 'node:test';
import assert from 'node:assert/strict';
import { isJobIdParam, NEW_DEMO_HREF, newDemoHref, PRODUCE_FORMAT } from './routes.ts';

const JOB_ID = '9a1c6e2f-4b3d-4a10-8f2e-1d6c7b9a0e55';

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
