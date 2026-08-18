// Format-only checks for CS2 share codes. Run: node --test "lib/**/*.test.ts"
import test from 'node:test';
import assert from 'node:assert/strict';
import { checkShareCode, normalizeShareCode } from './sharecode.ts';

const GOLDEN = 'GADqfjjyJ8cSP2rsmZRoTO2xK';

test('normalizeShareCode strips prefix, dashes and whitespace', () => {
  const cases: ReadonlyArray<{ name: string; raw: string; want: string }> = [
    { name: 'with CSGO- prefix', raw: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK', want: GOLDEN },
    { name: 'without prefix', raw: 'GADqf-jjyJ8-cSP2r-smZRo-TO2xK', want: GOLDEN },
    { name: 'lowercase csgo- prefix', raw: 'csgo-GADqf-jjyJ8-cSP2r-smZRo-TO2xK', want: GOLDEN },
    { name: 'surrounding spaces', raw: '  CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK  ', want: GOLDEN },
    { name: 'internal spaces', raw: 'CSGO-GADqf jjyJ8 cSP2r smZRo TO2xK', want: GOLDEN },
  ];
  for (const c of cases) {
    assert.equal(normalizeShareCode(c.raw), c.want, c.name);
  }
});

test('checkShareCode accepts a well-formed code', () => {
  const cases: ReadonlyArray<{ name: string; raw: string }> = [
    { name: 'with prefix', raw: 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK' },
    { name: 'without prefix', raw: 'GADqf-jjyJ8-cSP2r-smZRo-TO2xK' },
  ];
  for (const c of cases) {
    const got = checkShareCode(c.raw);
    assert.deepEqual(got, { ok: true, normalized: GOLDEN }, c.name);
  }
});

test('checkShareCode rejects malformed input with the right reason', () => {
  const cases: ReadonlyArray<{ name: string; raw: string; reason: 'empty' | 'length' | 'character' }> = [
    { name: 'empty', raw: '', reason: 'empty' },
    { name: 'whitespace-only', raw: '   ', reason: 'empty' },
    { name: '24 chars', raw: GOLDEN.slice(0, 24), reason: 'length' },
    { name: '26 chars', raw: `${GOLDEN}A`, reason: 'length' },
    { name: 'excluded character I', raw: `I${GOLDEN.slice(1)}`, reason: 'character' },
    { name: 'excluded character g', raw: `g${GOLDEN.slice(1)}`, reason: 'character' },
    { name: 'excluded character l', raw: `l${GOLDEN.slice(1)}`, reason: 'character' },
    { name: 'excluded character 0', raw: `0${GOLDEN.slice(1)}`, reason: 'character' },
    { name: 'excluded character 1', raw: `1${GOLDEN.slice(1)}`, reason: 'character' },
  ];
  for (const c of cases) {
    const got = checkShareCode(c.raw);
    assert.equal(got.ok, false, c.name);
    if (got.ok) continue;
    assert.equal(got.reason, c.reason, c.name);
    assert.notEqual(got.message, '', c.name);
  }
});
