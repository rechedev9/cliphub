import assert from 'node:assert/strict';
import test from 'node:test';
import { pinRenderRevision, renderRevisionFromPrefix, requestedRenderRevision } from './render-revision.ts';

const id = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
const revision = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
test('pins only an immutable revision from the same job and variant', () => {
  for (const [prefix, expected] of [
    [`jobs/${id}/renders/gameplay-pov-60/revisions/${revision}`, revision],
    [`jobs/${revision}/renders/gameplay-pov-60/revisions/${revision}`, undefined],
    [`jobs/${id}/renders/viral-60-clean/revisions/${revision}`, undefined],
    [`jobs/${id}/renders/gameplay-pov-60/revisions/../status.json`, undefined],
    [`jobs/${id}/renders/gameplay-pov-60`, undefined],
  ]) assert.equal(renderRevisionFromPrefix(prefix, id, 'gameplay-pov-60'), expected);
  assert.equal(pinRenderRevision('/video', revision), `/video?revision=${revision}`);
  assert.equal(pinRenderRevision('/video', undefined), '/video');
});
test('revision query is optional for legacy and rejects ambiguous or escaping values', () => {
  for (const [query, expected] of [
    ['', ''], [`?revision=${revision}`, `/revisions/${revision}`],
    ['?revision=', null], ['?revision=../status', null],
    ['?revision=00000000-0000-0000-0000-000000000000', null],
    [`?revision=${revision}&revision=${revision}`, null],
  ] as const) assert.equal(requestedRenderRevision(new Request(`http://localhost/video${query}`)), expected);
});
