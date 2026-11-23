import type { FaceitRanking, FaceitRankedPlayer, FaceitRankingKind } from './api/faceit.ts';
import { offlineMessage } from './load-error.ts';

export type RankingChoice = { kind: FaceitRankingKind; code: string };

/** One entry of the ranking selector: label is what the row shows, hint the line beside it. */
export type RankingOption = RankingChoice & { label: string; hint: string };

/** A ranking holds the top 100 of its entry (faceit.RankingSize); the page shows it 20 at a time. */
export const RANKING_SIZE = 100;
export const RANKING_PAGE = 20;

/** localStorage key of the last choice; Studio's storage lives in its profile. */
export const RANKING_CHOICE_KEY = 'cliphub.faceit-ranking-choice.v1';

const REGIONS = [
  { code: 'EU', label: 'Europa' },
  { code: 'NA', label: 'Norteamérica' },
  { code: 'SA', label: 'Sudamérica' },
  { code: 'OCE', label: 'Oceanía' },
  { code: 'SEA', label: 'Sudeste Asiático' },
] as const;

const ZONES = [
  { code: 'cis', label: 'CIS' },
  { code: 'latam', label: 'LATAM' },
] as const;

const FALLBACK_CHOICE: RankingChoice = { kind: 'region', code: 'EU' };

/** Regions, then zones, then every country by ISO code with its name as the hint. */
export function rankingOptions(countries: readonly string[], countryName: (code: string) => string | undefined): RankingOption[] {
  return [
    ...REGIONS.map((region): RankingOption => ({ kind: 'region', code: region.code, label: region.label, hint: 'Región FACEIT' })),
    ...ZONES.map((zone): RankingOption => ({ kind: 'zone', code: zone.code, label: zone.label, hint: 'Zona' })),
    ...countries.map((code): RankingOption => ({ kind: 'country', code, label: code.toUpperCase(), hint: countryName(code) ?? '' })),
  ];
}

const REGION_NAMES = new Intl.DisplayNames(['es'], { type: 'region', fallback: 'none' });

/** The Spanish name of a country code, or undefined when ICU knows none. */
export function spanishCountryName(code: string): string | undefined {
  try {
    return REGION_NAMES.of(code.toUpperCase());
  } catch {
    return undefined;
  }
}

function fold(value: string): string {
  return value.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().trim();
}

/** Options whose code, label or hint holds the query, the one whose code is the query first. */
export function filterRankingOptions(options: readonly RankingOption[], query: string): RankingOption[] {
  const needle = fold(query);
  if (needle === '') return [...options];
  const matches = options.filter((option) => [option.code, option.label, option.hint].some((text) => fold(text).includes(needle)));
  const exact = matches.filter((option) => fold(option.code) === needle);
  return [...exact, ...matches.filter((option) => !exact.includes(option))];
}

export function sameRankingChoice(a: RankingChoice | null, b: RankingChoice | null): boolean {
  return a !== null && b !== null && a.kind === b.kind && a.code === b.code;
}

function knownChoice(choice: RankingChoice, countries: readonly string[] | null): boolean {
  if (choice.kind === 'region') return REGIONS.some((region) => region.code === choice.code);
  if (choice.kind === 'zone') return ZONES.some((zone) => zone.code === choice.code);
  return countries === null ? /^[a-z]{2}$/.test(choice.code) : countries.includes(choice.code);
}

/** The saved choice when it still exists, else the system's country, else Europe. */
export function initialRankingChoice({ saved, systemCountry, countries }: {
  saved: RankingChoice | null;
  systemCountry: string | undefined;
  countries: readonly string[];
}): RankingChoice {
  if (saved !== null && knownChoice(saved, countries)) return saved;
  const code = systemCountry?.trim().toLowerCase() ?? '';
  return countries.includes(code) ? { kind: 'country', code } : FALLBACK_CHOICE;
}

/** Reads the stored last choice; anything but a known kind and code is no choice. */
export function parseRankingChoice(raw: string | null): RankingChoice | null {
  if (raw === null || raw === '') return null;
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof value !== 'object' || value === null || !('kind' in value) || !('code' in value)) return null;
  const { kind, code } = value;
  if ((kind !== 'country' && kind !== 'region' && kind !== 'zone') || typeof code !== 'string') return null;
  const choice: RankingChoice = { kind, code };
  return knownChoice(choice, null) ? choice : null;
}

/** How a choice reads in a sentence: a country's name (or its code), a region or a zone. */
export function rankingChoiceLabel(options: readonly RankingOption[], choice: RankingChoice): string {
  const option = options.find((entry) => sameRankingChoice(entry, choice));
  if (option === undefined) return choice.code.toUpperCase();
  return option.kind === 'country' && option.hint !== '' ? option.hint : option.label;
}

/** The line under a loaded list; when is the date and time the service read it. */
export function rankingNote(list: { stale: boolean; staleCode?: string }, when: string): string {
  if (!list.stale) return `Leído de FACEIT el ${when}.`;
  const reason = list.staleCode === 'faceit_rate_limited' ? 'FACEIT está limitando las peticiones.' : 'FACEIT no responde.';
  return `${reason} Mostramos la copia guardada el ${when}.`;
}

/** Why a ranking with no stored copy could not load. */
export function rankingErrorMessage(code: string | undefined): string {
  if (code === 'faceit_rate_limited') {
    return 'FACEIT está limitando las peticiones. No es un fallo de Studio: vuelve a intentarlo en unos minutos.';
  }
  if (code === 'service_unavailable') return offlineMessage();
  return 'No se pudo leer el ranking de FACEIT. Comprueba la conexión y vuelve a intentarlo.';
}

export type RankingListState = {
  choice: RankingChoice | null;
  status: 'idle' | 'loading' | 'ready' | 'error';
  players: FaceitRankedPlayer[];
  total: number;
  /** When the service read the ranking from FACEIT, epoch ms. */
  fetchedAt: number;
  stale: boolean;
  staleCode?: string;
  errorCode?: string;
  loadingMore: boolean;
  moreErrorCode?: string;
};

type RankingAction =
  | { type: 'choose'; choice: RankingChoice }
  | { type: 'more' }
  | { type: 'page'; choice: RankingChoice; offset: number; page: FaceitRanking }
  | { type: 'failed'; choice: RankingChoice; offset: number; code: string };

export const EMPTY_RANKING_LIST: RankingListState = { choice: null, status: 'idle', players: [], total: 0, fetchedAt: 0, stale: false, loadingMore: false };

export function rankingListReducer(state: RankingListState = EMPTY_RANKING_LIST, action: RankingAction): RankingListState {
  switch (action.type) {
    case 'choose':
      return { ...EMPTY_RANKING_LIST, choice: action.choice, status: 'loading' };
    case 'more':
      return state.status === 'ready' ? { ...state, loadingMore: true, moreErrorCode: undefined } : state;
    case 'page': {
      // A reply for a choice the user already left must not paint over the new one.
      if (!sameRankingChoice(state.choice, action.choice)) return state;
      const kept = action.offset === 0 ? [] : state.players;
      const fresh = action.page.players.filter((player) => !kept.some((row) => row.id === player.id));
      return {
        ...state, status: 'ready', players: [...kept, ...fresh], total: action.page.total, fetchedAt: action.page.fetchedAt,
        stale: action.page.stale, staleCode: action.page.staleCode, errorCode: undefined, loadingMore: false,
      };
    }
    case 'failed':
      if (!sameRankingChoice(state.choice, action.choice)) return state;
      if (action.offset === 0) return { ...state, status: 'error', errorCode: action.code, loadingMore: false };
      return { ...state, loadingMore: false, moreErrorCode: action.code };
  }
}

export function rankingHasMore(state: RankingListState): boolean {
  return state.status === 'ready' && state.players.length < Math.min(state.total, RANKING_SIZE);
}
