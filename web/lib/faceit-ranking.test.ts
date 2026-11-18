import assert from 'node:assert/strict';
import test from 'node:test';
import type { FaceitRanking, FaceitRankedPlayer } from './api/faceit.ts';
import {
  filterRankingOptions, initialRankingChoice, parseRankingChoice, rankingChoiceLabel, rankingErrorMessage, rankingHasMore,
  rankingListReducer, rankingNote, rankingOptions, spanishCountryName,
  type RankingChoice, type RankingListState,
} from './faceit-ranking.ts';

const names: Record<string, string> = { es: 'España', ee: 'Estonia', br: 'Brasil', pl: 'Polonia' };
const countryName = (code: string): string | undefined => names[code];
const options = rankingOptions(['br', 'ee', 'es', 'pl'], countryName);

test('the selector lists the five regions, then CIS and LATAM, then every country by its ISO code', () => {
  assert.deepEqual(options.map((option) => `${option.kind}:${option.label}`), [
    'region:Europa', 'region:Norteamérica', 'region:Sudamérica', 'region:Oceanía', 'region:Sudeste Asiático',
    'zone:CIS', 'zone:LATAM',
    'country:BR', 'country:EE', 'country:ES', 'country:PL',
  ]);
  assert.deepEqual(options.find((option) => option.code === 'es'), { kind: 'country', code: 'es', label: 'ES', hint: 'España' });
});

test('typing filters by code or name, without accents, with the exact code first', () => {
  const labels = (query: string): string[] => filterRankingOptions(options, query).map((option) => option.label);
  assert.deepEqual(labels('es'), ['ES', 'Sudeste Asiático', 'EE']);
  assert.deepEqual(labels('ESPANA'), ['ES']);
  assert.deepEqual(labels('latam'), ['LATAM']);
  assert.deepEqual(labels('  '), options.map((option) => option.label));
  assert.deepEqual(labels('zz'), []);
});

test('the page opens on the saved choice, else on the system country, else on Europe', () => {
  const countries = ['br', 'es'];
  const saved: RankingChoice = { kind: 'zone', code: 'latam' };
  assert.deepEqual(initialRankingChoice({ saved, systemCountry: 'ES', countries }), saved);
  assert.deepEqual(initialRankingChoice({ saved: null, systemCountry: 'ES', countries }), { kind: 'country', code: 'es' });
  assert.deepEqual(initialRankingChoice({ saved: { kind: 'country', code: 'zz' }, systemCountry: 'BR', countries }), { kind: 'country', code: 'br' });
  assert.deepEqual(initialRankingChoice({ saved: null, systemCountry: 'XK', countries }), { kind: 'region', code: 'EU' });
  assert.deepEqual(initialRankingChoice({ saved: null, systemCountry: undefined, countries }), { kind: 'region', code: 'EU' });
});

test('a saved choice reads back only when it is a known kind and code', () => {
  assert.deepEqual(parseRankingChoice(JSON.stringify({ kind: 'region', code: 'SEA' })), { kind: 'region', code: 'SEA' });
  assert.deepEqual(parseRankingChoice(JSON.stringify({ kind: 'country', code: 'es' })), { kind: 'country', code: 'es' });
  for (const raw of [null, '', '{', JSON.stringify({ kind: 'region', code: 'GLOBAL' }), JSON.stringify({ kind: 'zone', code: 'eu' }),
    JSON.stringify({ kind: 'country', code: 'ESP' }), JSON.stringify({ kind: 'team', code: 'es' })]) {
    assert.equal(parseRankingChoice(raw), null, String(raw));
  }
});

const ranked = (rank: number): FaceitRankedPlayer => ({
  rank, id: `p${rank}`, nickname: `p${rank}`, profile_url: `https://www.faceit.com/en/players/p${rank}`, elo: 4000 - rank,
});
const page = (choice: RankingChoice, ranks: number[], total = 100): FaceitRanking => ({
  ...choice, total, fetchedAt: 1000, stale: false, players: ranks.map(ranked),
});
const spain: RankingChoice = { kind: 'country', code: 'es' };
const poland: RankingChoice = { kind: 'country', code: 'pl' };
const first20 = Array.from({ length: 20 }, (_, i) => i + 1);

test('the list loads 20 at a time up to the ranking size and drops answers for an older choice', () => {
  let state: RankingListState = rankingListReducer(undefined, { type: 'choose', choice: spain });
  assert.equal(state.status, 'loading');
  state = rankingListReducer(state, { type: 'page', choice: spain, offset: 0, page: page(spain, first20) });
  assert.equal(state.status, 'ready');
  assert.equal(state.players.length, 20);
  assert.equal(rankingHasMore(state), true);

  state = rankingListReducer(state, { type: 'more' });
  assert.equal(state.loadingMore, true);
  // The next page repeats p20, as a list reread between two pages can; it takes one place.
  state = rankingListReducer(state, { type: 'page', choice: spain, offset: 20, page: page(spain, Array.from({ length: 20 }, (_, i) => i + 20)) });
  assert.equal(state.loadingMore, false);
  assert.deepEqual(state.players.map((player) => player.rank).slice(18, 22), [19, 20, 21, 22]);
  assert.equal(state.players.length, 39);

  state = rankingListReducer(state, { type: 'choose', choice: poland });
  const late = rankingListReducer(state, { type: 'page', choice: spain, offset: 0, page: page(spain, first20) });
  assert.deepEqual(late, state);
  assert.deepEqual(state.players, []);

  const short = rankingListReducer(state, { type: 'page', choice: poland, offset: 0, page: page(poland, [1, 2, 3], 3) });
  assert.equal(rankingHasMore(short), false);
});

test('a failed first page is an error to show, a failed next page keeps the rows', () => {
  const loading = rankingListReducer(undefined, { type: 'choose', choice: spain });
  const failed = rankingListReducer(loading, { type: 'failed', choice: spain, offset: 0, code: 'faceit_rate_limited' });
  assert.equal(failed.status, 'error');
  assert.equal(failed.errorCode, 'faceit_rate_limited');

  const ready = rankingListReducer(loading, { type: 'page', choice: spain, offset: 0, page: { ...page(spain, first20), stale: true, staleCode: 'faceit_unavailable' } });
  assert.equal(ready.stale, true);
  assert.equal(ready.staleCode, 'faceit_unavailable');
  const more = rankingListReducer(rankingListReducer(ready, { type: 'more' }), { type: 'failed', choice: spain, offset: 20, code: 'faceit_unavailable' });
  assert.equal(more.status, 'ready');
  assert.equal(more.players.length, 20);
  assert.equal(more.loadingMore, false);
  assert.equal(more.moreErrorCode, 'faceit_unavailable');
});

test('the note under the list says when it was read, or why the stored copy answers', () => {
  assert.equal(rankingNote({ stale: false }, '18/11/26, 12:00'), 'Leído de FACEIT el 18/11/26, 12:00.');
  assert.equal(rankingNote({ stale: true, staleCode: 'faceit_rate_limited' }, '18/11/26, 12:00'),
    'FACEIT está limitando las peticiones. Mostramos la copia guardada el 18/11/26, 12:00.');
  assert.equal(rankingNote({ stale: true, staleCode: 'faceit_unavailable' }, '18/11/26, 12:00'),
    'FACEIT no responde. Mostramos la copia guardada el 18/11/26, 12:00.');
});

test('a ranking that cannot load says whose fault it is', () => {
  assert.match(rankingErrorMessage('faceit_rate_limited'), /^FACEIT está limitando las peticiones\. No es un fallo de Studio/);
  assert.match(rankingErrorMessage('service_unavailable'), /^Servicio local sin conexión/);
  assert.match(rankingErrorMessage('faceit_unavailable'), /^No se pudo leer el ranking de FACEIT/);
});

test('a choice reads as its country name, region or zone', () => {
  assert.equal(rankingChoiceLabel(options, { kind: 'country', code: 'es' }), 'España');
  assert.equal(rankingChoiceLabel(options, { kind: 'region', code: 'SEA' }), 'Sudeste Asiático');
  assert.equal(rankingChoiceLabel(options, { kind: 'zone', code: 'cis' }), 'CIS');
  assert.equal(rankingChoiceLabel(rankingOptions(['xk'], () => undefined), { kind: 'country', code: 'xk' }), 'XK');
});

test('a country reads by its Spanish name, and a code with no name has none', () => {
  assert.equal(spanishCountryName('es'), 'España');
  assert.equal(spanishCountryName('br'), 'Brasil');
  assert.equal(spanishCountryName('xx'), undefined);
});
