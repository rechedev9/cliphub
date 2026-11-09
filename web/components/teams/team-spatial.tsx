'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { LibraryRosterEntry, TeamCriterion } from '@/lib/api/library';
import type { SpatialResponse } from '@/lib/api/spatial';
import { unrankedCount } from '@/lib/api/team-insights';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { heatChoiceFromQuery, writeHeatChoice } from '@/lib/replay/heat-choice';
import type { HeatChoice } from '@/lib/replay/heat-choice';
import { teamFocusFromQuery, toggleTeamFocus, writeTeamFocus } from '@/lib/replay/player-focus';
import { tramoFromQuery, writeTramo } from '@/lib/replay/tramo';
import type { Tramo } from '@/lib/replay/tramo';
import { cn } from '@/lib/utils';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { SpatialWorkspace } from '@/components/tactical/spatial/spatial-workspace';

/**
 * A team's rounds on the 2D radar, one map at a time, read through the page's
 * analysis scope. What the heatmap counts, the tramo and the players in focus
 * live in the URL, the keys `team_heatmap` writes in its `studio_path`.
 */
export function TeamSpatial({
  teamId,
  scope,
  lineup,
  criteria,
  view,
  roster,
  onMapChange,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  /** The rest of the Replay's filters bar, as query entries; the buys travel here too. */
  criteria: readonly TeamCriterion[];
  /** The form the team's Replay is on: a heatmap or the superposed paths. */
  view: 'heatmap' | 'overlay';
  roster: readonly LibraryRosterEntry[];
  onMapChange: (map: string) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const [now] = useState(() => Date.now());
  const { from, to } = scopeDateRange(scope.period, now);
  const onLoaded = useCallback(
    (response: SpatialResponse) => {
      const rounds = response.heatmap?.rounds ?? response.overlay?.total ?? 0;
      onCounts({ maps: response.matches.length, rounds, unrated: unrankedCount(response.skipped) });
    },
    [onCounts],
  );
  const replace = useCallback(
    (query: string) => router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false }),
    [pathname, router],
  );
  const tramoValue = useMemo(() => tramoFromQuery(search), [search]);
  const tramo = useMemo(
    () => ({ tramo: tramoValue, onChange: (next: Tramo) => replace(writeTramo(search.toString(), next)) }),
    [replace, search, tramoValue],
  );
  const heatValue = useMemo(() => heatChoiceFromQuery(search), [search]);
  const heat = useMemo(
    () => ({ choice: heatValue, onChange: (next: HeatChoice) => replace(writeHeatChoice(search.toString(), next)) }),
    [heatValue, replace, search],
  );
  const known = useMemo(() => new Set(roster.map((player) => player.steamid64)), [roster]);
  const focus = useMemo(() => teamFocusFromQuery(search).filter((id) => known.has(id)), [known, search]);
  const players = useMemo(() => [...roster].sort((a, b) => b.last_played_at.localeCompare(a.last_played_at) || b.matches - a.matches), [roster]);

  return (
    <div className="flex flex-col gap-3">
      {players.length > 1 ? (
        <div role="group" aria-label="Jugadores que cuenta" className="flex flex-wrap items-center gap-1.5">
          <span className="text-body-sm text-fg-3">Jugadores:</span>
          {players.map((player) => (
            <button
              key={player.steamid64}
              type="button"
              aria-pressed={focus.includes(player.steamid64)}
              onClick={() => replace(writeTeamFocus(search.toString(), toggleTeamFocus(focus, player.steamid64)))}
              className={cn(
                'h-7 max-w-40 truncate rounded-sm border px-2 text-body-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
                focus.includes(player.steamid64) ? 'border-primary text-fg-1' : 'border-border text-fg-2 hover:text-fg-1',
              )}
            >
              {player.nick || player.steamid64}
            </button>
          ))}
          <span className="text-meta text-fg-3">{focus.length === 0 ? 'Sin selección cuenta todo el equipo.' : ''}</span>
        </div>
      ) : null}
      <SpatialWorkspace
        side={scope.side}
        view={view}
        hideViewSwitch
        onMapChange={onMapChange}
        onLoaded={onLoaded}
        tramo={tramo}
        heat={heat}
        scope={{
          kind: 'team',
          teamId,
          query: {
            map: scope.map === '' ? undefined : scope.map,
            side: scope.side,
            opponent_ids: scope.opponent === '' ? [] : [scope.opponent],
            from,
            to,
            lineup,
            criteria,
            players: focus,
          },
        }}
      />
    </div>
  );
}
