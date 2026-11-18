import test from 'node:test';
import assert from 'node:assert/strict';
import { clipDemoTitle, demoHeading } from './demo-title.ts';

test('a titled demo leads with its title and moves the teams to the second line', () => {
  assert.deepEqual(demoHeading({ title: 'Final ESL', teams: 'Vitality vs MOUZ', fallback: 'Mirage' }), { heading: 'Final ESL', secondary: 'Vitality vs MOUZ' });
});

test('without a title the teams lead, and without teams the card keeps what it showed', () => {
  assert.deepEqual(demoHeading({ teams: 'Vitality vs MOUZ', fallback: 'Mirage' }), { heading: 'Vitality vs MOUZ', secondary: '' });
  assert.deepEqual(demoHeading({ title: '', teams: '', fallback: 'final.dem' }), { heading: 'final.dem', secondary: '' });
});

test('a title draft stops at 80 characters, an emoji counting as one', () => {
  assert.equal(clipDemoTitle('é'.repeat(90)), 'é'.repeat(80));
  assert.equal([...clipDemoTitle('🔥'.repeat(90))].length, 80);
  assert.equal(clipDemoTitle('Final ESL'), 'Final ESL');
});
