/**
 * The pure half of a record list: sorting, column visibility, filtering and
 * the saved views a person keeps for a table ("Pistols en Mirage"). The React
 * side lives in `components/records/`; this module never touches storage.
 */

export type SortDirection = 'asc' | 'desc';
export type TableSort = { readonly column: string; readonly direction: SortDirection };
/** A sortable cell value. `null` always sorts last, whatever the direction. */
export type SortValue = string | number | null;

/** Everything a saved view restores: order, hidden columns and filter values. */
export type TableViewState = {
  readonly sort: TableSort | null;
  readonly hidden: readonly string[];
  readonly filters: Readonly<Record<string, string>>;
};

export type SavedTableView = { readonly id: string; readonly name: string; readonly state: TableViewState };

/** The value a filter holds when it is not narrowing anything. */
export const FILTER_ANY = '';
/** The free-text search shares the filter map under this key. */
export const SEARCH_FILTER = 'q';
const MAX_VIEW_NAME = 60;
const MAX_SAVED_VIEWS = 30;

/** Header click cycle: ascending, descending, then back to the default order. */
export function nextSort(current: TableSort | null, column: string): TableSort | null {
  if (current?.column !== column) return { column, direction: 'asc' };
  if (current.direction === 'asc') return { column, direction: 'desc' };
  return null;
}

const COLLATOR = new Intl.Collator('es', { numeric: true, sensitivity: 'base' });

function compareValues(a: SortValue, b: SortValue): number {
  if (typeof a === 'number' && typeof b === 'number') return a - b;
  return COLLATOR.compare(String(a), String(b));
}

/** Stable sort; rows without a value stay at the bottom in both directions. */
export function sortRows<Row>(
  rows: readonly Row[],
  sort: TableSort | null,
  valueOf: (row: Row, column: string) => SortValue,
): Row[] {
  if (sort === null) return [...rows];
  const sign = sort.direction === 'asc' ? 1 : -1;
  return rows
    .map((row, index) => ({ row, index, value: valueOf(row, sort.column) }))
    .sort((left, right) => {
      if (left.value === null || right.value === null) {
        if (left.value === right.value) return left.index - right.index;
        return left.value === null ? 1 : -1;
      }
      return sign * compareValues(left.value, right.value) || left.index - right.index;
    })
    .map((entry) => entry.row);
}

export function toggleHidden(hidden: readonly string[], column: string): string[] {
  return hidden.includes(column) ? hidden.filter((entry) => entry !== column) : [...hidden, column];
}

/** Lowercase without accents, so "mirage" finds "Mirage" and "jugador" finds "Jugadór". */
export function normalizeSearch(text: string): string {
  return text.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase().trim();
}

/**
 * Rows that pass every active filter. `fields` reads the value a filter id
 * compares against, or several values when a row has more than one (the two
 * teams of a match); the search filter matches any word of `searchText`.
 */
export function filterRows<Row>(
  rows: readonly Row[],
  filters: Readonly<Record<string, string>>,
  read: { fields: Readonly<Record<string, (row: Row) => string | readonly string[]>>; searchText: (row: Row) => string },
): Row[] {
  const active = Object.entries(filters).filter(([, value]) => value !== FILTER_ANY);
  if (active.length === 0) return [...rows];
  return rows.filter((row) =>
    active.every(([id, value]) => {
      if (id === SEARCH_FILTER) {
        const haystack = normalizeSearch(read.searchText(row));
        return normalizeSearch(value).split(/\s+/).every((word) => haystack.includes(word));
      }
      const field = read.fields[id];
      if (field === undefined) return true;
      const got = field(row);
      return typeof got === 'string' ? got === value : got.includes(value);
    }),
  );
}

export function activeFilterCount(filters: Readonly<Record<string, string>>): number {
  return Object.values(filters).filter((value) => value !== FILTER_ANY).length;
}

/** The id a view saved under `name` gets; two names that differ only in case or accents share it. */
export function savedViewId(name: string): string {
  return normalizeSearch(name.trim().slice(0, MAX_VIEW_NAME)).replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'vista';
}

/** Saving under an existing name (ignoring case and accents) replaces that view. */
export function upsertView(views: readonly SavedTableView[], name: string, state: TableViewState): SavedTableView[] {
  const trimmed = name.trim().slice(0, MAX_VIEW_NAME);
  if (trimmed === '') return [...views];
  const id = savedViewId(trimmed);
  const next = { id, name: trimmed, state };
  const index = views.findIndex((view) => view.id === id);
  if (index === -1) return [...views, next].slice(-MAX_SAVED_VIEWS);
  return views.map((view, at) => (at === index ? next : view));
}

export function removeView(views: readonly SavedTableView[], id: string): SavedTableView[] {
  return views.filter((view) => view.id !== id);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseSort(value: unknown): TableSort | null {
  if (!isRecord(value)) return null;
  const { column, direction } = value;
  if (typeof column !== 'string' || (direction !== 'asc' && direction !== 'desc')) return null;
  return { column, direction };
}

function parseState(value: unknown): TableViewState | null {
  if (!isRecord(value)) return null;
  const hidden = Array.isArray(value.hidden) ? value.hidden.filter((entry) => typeof entry === 'string') : [];
  const filters: Record<string, string> = {};
  if (isRecord(value.filters)) {
    for (const [key, entry] of Object.entries(value.filters)) if (typeof entry === 'string') filters[key] = entry;
  }
  return { sort: parseSort(value.sort), hidden, filters };
}

/** Reads what a previous session stored; anything malformed is dropped, never thrown. */
export function parseSavedViews(raw: string | null): SavedTableView[] {
  if (raw === null) return [];
  let decoded: unknown;
  try {
    decoded = JSON.parse(raw);
  } catch {
    return [];
  }
  if (!Array.isArray(decoded)) return [];
  const views: SavedTableView[] = [];
  for (const entry of decoded) {
    if (!isRecord(entry) || typeof entry.name !== 'string') continue;
    const state = parseState(entry.state);
    if (state === null) continue;
    const merged = upsertView(views, entry.name, state);
    views.splice(0, views.length, ...merged);
  }
  return views;
}

/** The localStorage key of one table's saved views. */
export function savedViewsKey(table: string): string {
  return `cliphub.records.${table}.views`;
}
