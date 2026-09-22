import assert from 'node:assert/strict';
import test from 'node:test';
import { streamDocumentTitle, streamTitle } from './title.ts';

test('stream titles retain Unicode and do not invent characters lost in older imports', () => {
  assert.equal(streamTitle({ title: ' dale niño ! 🎯 ' }), 'dale niño ! 🎯');
  assert.equal(streamTitle({ title: 'dale ni\uFFFDo !' }), 'Clip de stream');
});

test('the editor tab is named after the project', () => {
  assert.equal(streamDocumentTitle('Zacketizor level 2'), 'Zacketizor level 2 \u00B7 Clips de stream \u00B7 ClipHub');
});
