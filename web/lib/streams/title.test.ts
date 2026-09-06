import assert from 'node:assert/strict';
import test from 'node:test';
import { streamTitle } from './title.ts';

test('stream titles retain Unicode and do not invent characters lost in older imports', () => {
  assert.equal(streamTitle({ title: ' dale niño ! 🎯 ' }), 'dale niño ! 🎯');
  assert.equal(streamTitle({ title: 'dale ni\uFFFDo !' }), 'Clip de stream');
});
