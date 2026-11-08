import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalSide } from '../api/tactical.ts';
import { matchStory, roundBlocks, sideRecord, type StoryRound, type TeamNames } from './match-story.ts';

const NAMES: TeamNames = { 'ct-start': 'CatClan', 't-start': 'team_miwel' };

const END: Record<string, string> = { def: 'bomb_defused', exp: 'bomb_exploded', elim: 't_eliminated_ct', time: 'time_expired' };

/** A won round: `a` is the team that started CT, `b` the one that started T. */
type Spec = { team: 'a' | 'b'; end?: keyof typeof END; site?: 'A' | 'B'; loserEco?: boolean };

/** MR12 with MR3 overtimes: the half and overtime the scan gives round `index`, and the side each team plays.
 * Overtime halves count on from 3 and each overtime starts on the sides the half before it ended on. */
function build(specs: readonly Spec[]): StoryRound[] {
  return specs.map((spec, index) => {
    const overtime = index < 24 ? 0 : Math.floor((index - 24) / 6) + 1;
    const half = index < 24 ? Math.floor(index / 12) + 1 : 3 + Math.floor((index - 24) / 3);
    const aSide: TacticalSide = Math.floor(half / 2) % 2 === 0 ? 'CT' : 'T';
    const bSide: TacticalSide = aSide === 'CT' ? 'T' : 'CT';
    const winnerSide = spec.team === 'a' ? aSide : bSide;
    const loserBuy = spec.loserEco ? 'eco' : 'full';
    return {
      number: index + 1,
      winnerSide,
      winnerTeamKey: spec.team === 'a' ? 'ct-start' : 't-start',
      round: {
        half,
        overtime,
        end_reason: END[spec.end ?? 'elim'] ?? '',
        economy: winnerSide === 'CT' ? { ct_buy: 'full', t_buy: loserBuy } : { ct_buy: loserBuy, t_buy: 'full' },
        bomb: spec.site ? { site: spec.site } : undefined,
        class: { tags: index === 0 || index === 12 ? ['pistol'] : [] },
      },
    };
  });
}

function wins(pattern: string): Spec[] {
  return [...pattern].map((team) => ({ team: team === 'a' ? 'a' : 'b' }));
}

// Inferno 10-13 from the design: CatClan takes R12 to R16, team_miwel the last seven as CT.
const INFERNO = build([
  ...wins('aabbabaabbba'),
  ...wins('aaaabb'),
  { team: 'b', loserEco: true },
  ...wins('bbb'),
  { team: 'b', end: 'def', site: 'B' },
]);

test('the story of a comeback names the pistols, both runs and the close', () => {
  assert.deepEqual(matchStory(INFERNO, NAMES), [
    { rounds: 'R 1 y 13', text: 'CatClan gana las dos pistolas.' },
    { rounds: 'R 12 a 16', text: 'CatClan encadena cinco rondas y se pone 10-6 tras el descanso.' },
    { rounds: 'R 17 a 23', text: 'team_miwel gana las siete últimas rondas como CT, una de ellas con CatClan en eco.' },
    { rounds: 'R 23', text: 'team_miwel gana la última ronda desactivando la bomba en B y cierra 13-10.' },
  ]);
});

test('split pistols name both winners', () => {
  const rounds = build([...wins('abababababab'), ...wins('bababababab')]);
  const [pistols] = matchStory(rounds, NAMES);
  assert.deepEqual(pistols, { rounds: 'R 1 y 13', text: 'CatClan gana la pistola de la ronda 1 y team_miwel la de la ronda 13.' });
});

test('an overtime is told before the run that decides it', () => {
  const rounds = build([...wins('ab'.repeat(6)), ...wins('abababababba'), ...wins('bbbb')]);
  assert.deepEqual(matchStory(rounds, NAMES), [
    { rounds: 'R 1 y 13', text: 'CatClan gana las dos pistolas.' },
    { rounds: 'R 25', text: 'Empate a 12 al final del tiempo reglamentario: la partida se va a la prórroga.' },
    { rounds: 'R 25 a 28', text: 'team_miwel gana las cuatro últimas rondas.' },
    { rounds: 'R 28', text: 'team_miwel gana la última ronda eliminando al rival y cierra 16-12.' },
  ]);
});

test('a run tells the score it leaves: ahead, level or still behind', () => {
  assert.deepEqual(matchStory(build(wins('bbbaaab')), NAMES).slice(1), [
    { rounds: 'R 1 a 3', text: 'team_miwel encadena tres rondas como T y se pone 3-0.' },
    { rounds: 'R 4 a 6', text: 'CatClan encadena tres rondas como CT y empata a 3.' },
    { rounds: 'R 7', text: 'team_miwel gana la última ronda eliminando al rival y cierra 4-3.' },
  ]);
  assert.equal(matchStory(build(wins('aaaabbba')), NAMES)[2]?.text, 'team_miwel encadena tres rondas como T y recorta hasta el 3-4.');
});

test('a last round won by the side that loses the match', () => {
  assert.equal(matchStory(build(wins('bbbaa')), NAMES).at(-1)?.text, 'CatClan gana la última ronda eliminando al rival, pero team_miwel se lleva la partida 3-2.');
});

test('a last round with no winner tells no closing line', () => {
  const rounds = build(wins('aab'));
  const last = rounds.at(-1);
  assert.ok(last);
  rounds[rounds.length - 1] = { ...last, winnerSide: '', winnerTeamKey: '' };
  assert.equal(matchStory(rounds, NAMES).some((fact) => fact.text.includes('última ronda')), false);
});

test('no rounds tell no story', () => {
  assert.deepEqual(matchStory([], NAMES), []);
});

test('the history breaks at the half and before the overtime, with the score of each block', () => {
  const regulation = roundBlocks(INFERNO).map(({ key, label, wins: score, rounds }) => ({ key, label, score, count: rounds.length }));
  assert.deepEqual(regulation, [
    { key: 'half-1', label: 'Primera mitad', score: [6, 6], count: 12 },
    { key: 'half-2', label: 'Segunda mitad', score: [4, 7], count: 11 },
  ]);
  const overtime = build([...wins('ab'.repeat(12)), ...wins('ababab'), ...wins('bbbb')]);
  assert.deepEqual(roundBlocks(overtime).map((block) => [block.label, block.wins]), [
    ['Primera mitad', [6, 6]],
    ['Segunda mitad', [6, 6]],
    ['Prórroga 1', [3, 3]],
    ['Prórroga 2', [0, 4]],
  ]);
});

test('each team wins and plays its rounds on the side it held', () => {
  assert.deepEqual(sideRecord(INFERNO, 'ct-start'), { CT: { won: 6, played: 12 }, T: { won: 4, played: 11 } });
  assert.deepEqual(sideRecord(INFERNO, 't-start'), { T: { won: 6, played: 12 }, CT: { won: 7, played: 11 } });
});
