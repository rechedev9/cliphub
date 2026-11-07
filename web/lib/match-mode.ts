import type { TacticalSide } from './api/tactical.ts';

/** What the Modo partida window opens on; the desktop parses it again (desktop/src/match-mode.ts). */
export type MatchModeRequest = { map: string; side: TacticalSide; team: string };

type MatchModeOpen = (request: MatchModeRequest) => Promise<boolean>;

/** The desktop's Modo partida bridge, or null in a browser. */
export function matchModeOpener(scope: object = globalThis): MatchModeOpen | null {
  const bridge: unknown = Reflect.get(scope, 'cliphubMatchMode');
  if (typeof bridge !== 'object' || bridge === null || !('open' in bridge)) return null;
  const open: unknown = bridge.open;
  if (typeof open !== 'function') return null;
  return (request) => Promise.resolve(Reflect.apply(open, bridge, [request])).then((ok: unknown) => ok === true);
}
