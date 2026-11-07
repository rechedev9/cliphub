import { getMapCallouts, type MapCallout } from '../api/callouts.ts';
import { placeLabel } from '../tactical-labels.ts';

/** A CS2 place of a map as the team says it: its own name when it has one, else CS2's; '' for no place. */
export type CalloutLabel = (map: string, token: string) => string;

export type CalloutNameLoader = (map: string) => Promise<readonly Pick<MapCallout, 'token' | 'name'>[]>;

export type CalloutRename = { map: string; token: string; name: string | null };

/** The team's callout names, read once per map; `label` changes identity whenever a name does. */
export type CalloutNameStore = {
  label: () => CalloutLabel;
  subscribe: (listener: () => void) => () => void;
  load: (map: string) => Promise<void>;
  /** Puts what a fresh read of the map returned. */
  replace: (map: string, callouts: readonly Pick<MapCallout, 'token' | 'name'>[]) => void;
  /** A name set or, with null, cleared; a map not read yet is left for its first read. */
  rename: (change: CalloutRename) => void;
  /** Forgets every map, so the next use reads them again, as after an import. */
  reset: () => void;
};

type Names = ReadonlyMap<string, ReadonlyMap<string, string>>;

function labelOf(names: Names): CalloutLabel {
  return (map, token) => {
    const place = token.trim();
    if (place === '') return '';
    return names.get(map)?.get(place) ?? placeLabel(place);
  };
}

export function createCalloutNameStore(loader: CalloutNameLoader): CalloutNameStore {
  let names: Names = new Map();
  let label = labelOf(names);
  const pending = new Map<string, Promise<void>>();
  const listeners = new Set<() => void>();

  function publish(next: Names): void {
    names = next;
    label = labelOf(next);
    for (const listener of listeners) listener();
  }

  function replace(map: string, callouts: readonly Pick<MapCallout, 'token' | 'name'>[]): void {
    // A read still on its way is older than this one, so its answer is dropped.
    pending.delete(map);
    const named = new Map<string, string>();
    for (const callout of callouts) if (callout.name) named.set(callout.token, callout.name);
    publish(new Map(names).set(map, named));
  }

  return {
    label: () => label,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    load: (map) => {
      if (map === '' || names.has(map)) return Promise.resolve();
      const known = pending.get(map);
      if (known !== undefined) return known;
      // A failed read stores nothing, so labels stay CS2's and the next use tries again.
      const read: Promise<void> = loader(map)
        .then((callouts) => {
          if (pending.get(map) === read) replace(map, callouts);
        })
        .catch(() => undefined)
        .finally(() => {
          if (pending.get(map) === read) pending.delete(map);
        });
      pending.set(map, read);
      return read;
    },
    replace,
    rename: ({ map, token, name }) => {
      const current = names.get(map);
      if (current === undefined) return;
      const next = new Map(current);
      if (name === null) next.delete(token);
      else next.set(token, name);
      publish(new Map(names).set(map, next));
    },
    reset: () => {
      pending.clear();
      publish(new Map());
    },
  };
}

/** The session's one cache of callout names, shared by every view that shows a place. */
export const CALLOUT_NAMES = createCalloutNameStore(async (map) => (await getMapCallouts(map)).callouts);
