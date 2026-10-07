import test from 'node:test';
import assert from 'node:assert/strict';
import { FULL_DEMO_NO_VOICE_PACKETS, fullDemoBlockerText } from './full-demo-blocker-copy.ts';

test('a no_packets voice blocker tells the user to turn team voices off', () => {
  assert.equal(
    fullDemoBlockerText({ code: 'voice_unavailable', message: 'Team voice is unavailable: no_packets' }),
    FULL_DEMO_NO_VOICE_PACKETS,
  );
  assert.match(FULL_DEMO_NO_VOICE_PACKETS, /Desactiva «Incluir voces del equipo»/);
  assert.doesNotMatch(FULL_DEMO_NO_VOICE_PACKETS, /no_packets|Team voice/i);
});

test('other blockers stay as the planner wrote them', () => {
  const cases = [
    { code: 'voice_unavailable', message: 'Team voice is unavailable: silent' },
    { code: 'voice_unavailable', message: 'Team voice is unavailable: no_team_packets' },
    { code: 'full_demo_asset_missing', message: 'Team voice is unavailable: no_packets' },
    { code: 'voice_decode', message: 'Team voice extraction is incompatible or failed: no_packets' },
  ];
  for (const notice of cases) {
    assert.equal(fullDemoBlockerText(notice), notice.message);
  }
});
