import { request } from './library.ts';

/** Browser-side contract of `GET /api/library/search` (`searchResult` in Go). */
export const SEARCH_KINDS = ['play', 'lineup', 'call', 'team', 'player', 'match'] as const;
export type SearchKind = (typeof SEARCH_KINDS)[number];

export type SearchResult = {
  kind: SearchKind;
  id: string;
  title: string;
  map?: string;
  side?: string;
  status?: string;
  detail?: string;
  team_name?: string;
};

export type SearchResponse = { query: string; map?: string; grenade?: string; results: SearchResult[] };

export const SEARCH_ROUTE = '/search';
export const SEARCH_QUERY_KEYS = ['q', 'limit'];
export const SEARCH_RESPONSE_KEYS = ['query', 'map', 'grenade', 'results'];

function isSearchKind(value: unknown): value is SearchKind {
  return SEARCH_KINDS.some((kind) => kind === value);
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

/** Keeps the rows the UI can show; a kind a newer Studio adds is dropped. */
export function searchResultsOf(value: unknown): SearchResult[] {
  if (typeof value !== 'object' || value === null || !('results' in value) || !Array.isArray(value.results)) return [];
  const out: SearchResult[] = [];
  for (const row of value.results) {
    if (typeof row !== 'object' || row === null) continue;
    const { kind, id, title } = { kind: Reflect.get(row, 'kind'), id: Reflect.get(row, 'id'), title: Reflect.get(row, 'title') };
    if (!isSearchKind(kind) || typeof id !== 'string' || typeof title !== 'string') continue;
    out.push({
      kind, id, title,
      map: optionalString(Reflect.get(row, 'map')),
      side: optionalString(Reflect.get(row, 'side')),
      status: optionalString(Reflect.get(row, 'status')),
      detail: optionalString(Reflect.get(row, 'detail')),
      team_name: optionalString(Reflect.get(row, 'team_name')),
    });
  }
  return out;
}

export async function searchLibrary(query: string, signal?: AbortSignal): Promise<SearchResult[]> {
  const params = new URLSearchParams({ q: query });
  const body = await request<unknown>(`${SEARCH_ROUTE}?${params.toString()}`, { signal });
  return searchResultsOf(body);
}
