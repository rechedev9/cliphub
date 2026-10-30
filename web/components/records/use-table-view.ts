'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  FILTER_ANY,
  parseSavedViews,
  removeView,
  savedViewId,
  savedViewsKey,
  toggleHidden,
  upsertView,
  type SavedTableView,
  type TableSort,
  type TableViewState,
} from '@/lib/records/table-view';

/** What a list page hands to its FilterBar and DataTable: one state, one set of saved views. */
export type TableViewControl = {
  readonly state: TableViewState;
  readonly views: readonly SavedTableView[];
  /** The saved view on screen, until the person changes anything. */
  readonly activeViewId: string | null;
  setSort: (sort: TableSort | null) => void;
  toggleColumn: (column: string) => void;
  setFilter: (id: string, value: string) => void;
  resetFilters: () => void;
  saveView: (name: string) => void;
  applyView: (id: string) => void;
  deleteView: (id: string) => void;
};

// Saved views are a per-PC convenience: storage can be missing or full, and the table must work without it.
function readViews(table: string): SavedTableView[] {
  try {
    return parseSavedViews(window.localStorage.getItem(savedViewsKey(table)));
  } catch {
    return [];
  }
}

function writeViews(table: string, views: readonly SavedTableView[]): void {
  try {
    window.localStorage.setItem(savedViewsKey(table), JSON.stringify(views));
  } catch {
    // The views stay in memory for this session.
  }
}

export function useTableView(table: string, initial: TableViewState): TableViewControl {
  const [state, setState] = useState<TableViewState>(initial);
  const [views, setViews] = useState<SavedTableView[]>([]);
  const [activeViewId, setActiveViewId] = useState<string | null>(null);

  // Read after mount: the server render has no storage, so the first paint never depends on it.
  useEffect(() => {
    setViews(readViews(table));
  }, [table]);

  const change = useCallback((update: (current: TableViewState) => TableViewState) => {
    setState(update);
    setActiveViewId(null);
  }, []);

  const persist = useCallback(
    (next: SavedTableView[]) => {
      setViews(next);
      writeViews(table, next);
    },
    [table],
  );

  return {
    state,
    views,
    activeViewId,
    setSort: (sort) => change((current) => ({ ...current, sort })),
    toggleColumn: (column) => change((current) => ({ ...current, hidden: toggleHidden(current.hidden, column) })),
    setFilter: (id, value) => change((current) => ({ ...current, filters: { ...current.filters, [id]: value } })),
    resetFilters: () =>
      change((current) => ({
        ...current,
        filters: Object.fromEntries(Object.keys(current.filters).map((key) => [key, FILTER_ANY])),
      })),
    saveView: (name) => {
      if (name.trim() === '') return;
      persist(upsertView(views, name, state));
      setActiveViewId(savedViewId(name));
    },
    applyView: (id) => {
      const view = views.find((entry) => entry.id === id);
      if (view === undefined) return;
      setState({ ...initial, ...view.state, filters: { ...initial.filters, ...view.state.filters } });
      setActiveViewId(id);
    },
    deleteView: (id) => {
      persist(removeView(views, id));
      if (activeViewId === id) setActiveViewId(null);
    },
  };
}
