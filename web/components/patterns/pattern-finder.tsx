'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { Map as MapIcon, PlugZap } from 'lucide-react';
import { getMapCallouts, type MapCallout } from '@/lib/api/callouts';
import { isLibraryOffline, type LibraryMatch, type LibraryRosterEntry, type TeamCriterion } from '@/lib/api/library';
import {
  runTeamPattern,
  type PatternArea,
  type PatternBlock,
  type PatternDefinition,
  type PatternRunResponse,
  type PatternScope,
  type SavedPattern,
  type TeamPatternScope,
} from '@/lib/api/patterns';
import { RADAR_LEVELS, type RadarLevel, type TacticalSide } from '@/lib/api/tactical';
import {
  INVERT_LABEL,
  PATTERN_QUERY,
  areaShape,
  blockTramo,
  defaultPattern,
  patternFromQuery,
  timedKind,
  withTramo,
  writePatternQuery,
  type AreaShape,
} from '@/lib/analysis/pattern-finder';
import { busiestMap, inScopePeriod, scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { prettyMapName } from '@/lib/format';
import { rivalOf } from '@/lib/library/partidas';
import { ROUND_CLOCK_SECONDS, WHOLE_ROUND, isWholeRound, tramoLabel, type Tramo } from '@/lib/replay/tramo';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap } from '@/lib/tactical-transform';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { EmptyState } from '@/components/records/empty-state';
import { QuickTramos, TramoBracket } from '@/components/replay/tramo-bar';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { PatternBlocks } from './pattern-blocks';
import { PatternRadar, type PatternPicture } from './pattern-radar';
import { PatternResults } from './pattern-results';
import { SavePatternButton, SavedPatterns } from './pattern-saved';

type Load = { kind: 'loading' } | { kind: 'ready'; data: PatternRunResponse } | { kind: 'offline' } | { kind: 'failed' };

/** Waits this long after the last change before asking again, so a drag of the tramo asks once. */
const RUN_DELAY_MS = 250;

const PICTURES: readonly { value: PatternPicture; label: string }[] = [
  { value: 'recorridos', label: 'Recorridos' },
  { value: 'calor', label: 'Mapa de calor' },
];

type Props = {
  teamId: string;
  scope: AnalysisScope;
  lineup: readonly string[];
  /** The bar's other filters as team scope keys (last N matches, buys, AWP, advantage, range, rival level, whose). */
  criteria: readonly TeamCriterion[];
  matches: readonly LibraryMatch[];
  roster: readonly LibraryRosterEntry[];
  onMapChange: (map: string) => void;
  /** Opening a saved pattern sets the bar to its own scope. */
  onScopeChange: (patch: Partial<AnalysisScope>) => void;
  onCounts: (counts: ScopeCounts | null) => void;
};

/** A team's pattern finder on one map: blocks that add up, the rounds meeting them all, and their radar. */
export function PatternFinder({ teamId, scope, lineup, criteria, matches, roster, onMapChange, onScopeChange, onCounts }: Props): ReactNode {
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const [now] = useState(() => Date.now());
  const { from, to } = scopeDateRange(scope.period, now);
  const [active, setActive] = useState(0);
  const [picture, setPicture] = useState<PatternPicture>('recorridos');
  const [floor, setFloor] = useState<RadarLevel | 'both'>('both');
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const [saved, setSaved] = useState(0);
  const [shape, setShape] = useState<AreaShape>('box');
  const [opened, setOpened] = useState<SavedPattern | null>(null);
  const [callouts, setCallouts] = useState<MapCallout[]>([]);
  const { map, opponent, buy, opponentBuy, limit, period } = scope;
  // With no map in the scope the finder opens on the team's busiest one, as Granadas does.
  const defaultMap = useMemo(
    () =>
      busiestMap(
        matches
          .filter((match) => inScopePeriod(match.played_at, period, now) && (opponent === '' || rivalOf(match, teamId)?.team_id === opponent))
          .map((match) => match.map),
      ),
    [matches, period, now, opponent, teamId],
  );
  useEffect(() => {
    if (map === '' && defaultMap !== '') onMapChange(defaultMap);
  }, [map, defaultMap, onMapChange]);
  useEffect(() => {
    if (map === '') return;
    let live = true;
    getMapCallouts(map)
      .then((read) => {
        if (live) setCallouts(read.callouts);
      })
      .catch(() => {
        if (live) setCallouts([]);
      });
    return () => {
      live = false;
    };
  }, [map]);

  // Keyed by the raw text, so the pattern only changes when its JSON does.
  const raw = search.get(PATTERN_QUERY);
  const pattern: PatternDefinition = useMemo(() => {
    const base = patternFromQuery(new URLSearchParams(raw === null ? [] : [[PATTERN_QUERY, raw]])) ?? defaultPattern();
    return base.side === undefined ? { ...base, side: 'T' } : base;
  }, [raw]);
  const side: TacticalSide = pattern.side ?? 'T';
  const block: PatternBlock | undefined = pattern.blocks[Math.min(active, pattern.blocks.length - 1)];
  const setPattern = useCallback(
    (next: PatternDefinition) => {
      const query = writePatternQuery(search.toString(), next);
      router.replace(query === '' ? pathname : `${pathname}?${query}`, { scroll: false });
    },
    [pathname, router, search],
  );

  const lineupKey = lineup.join(',');
  const runScope = useMemo<TeamPatternScope>(
    () => ({
      map: map === '' ? undefined : map,
      from,
      to,
      opponent_ids: opponent === '' ? [] : [opponent],
      lineup: lineupKey === '' ? [] : lineupKey.split(','),
      criteria,
    }),
    [map, from, to, opponent, lineupKey, criteria],
  );
  const saveScope = useMemo<PatternScope>(() => {
    const out: PatternScope = {};
    if (limit !== null) out.matches = limit;
    if (buy !== '') out.buy = buy;
    if (opponentBuy !== '') out.opponent_buy = opponentBuy;
    return out;
  }, [limit, buy, opponentBuy]);
  useEffect(() => {
    if (map === '') {
      onCounts({ maps: 0, rounds: 0 });
      return;
    }
    let live = true;
    const timer = window.setTimeout(() => {
      runTeamPattern({ teamId, scope: runScope, pattern })
        .then((data) => {
          if (!live) return;
          setLoad({ kind: 'ready', data });
          onCounts({ maps: data.result.matches.length, rounds: data.result.rounds });
        })
        .catch((error: unknown) => {
          if (live) setLoad({ kind: isLibraryOffline(error) ? 'offline' : 'failed' });
        });
    }, RUN_DELAY_MS);
    return () => {
      live = false;
      window.clearTimeout(timer);
    };
  }, [teamId, map, runScope, pattern, attempt, onCounts]);

  if (map === '' && defaultMap !== '') return <FinderSkeleton />;
  if (map === '') {
    const maps = [...new Set(matches.map((match) => match.map))].sort();
    return (
      <EmptyState
        icon={MapIcon}
        title="Elige un mapa"
        description="Los patrones se buscan sobre el radar de un mapa."
        actions={
          maps.length === 0 ? undefined : (
            <div className="flex flex-wrap gap-2">
              {maps.map((entry) => (
                <Button key={entry} type="button" variant="outline" size="sm" onClick={() => onMapChange(entry)}>
                  {prettyMapName(entry)}
                </Button>
              ))}
            </div>
          )
        }
      />
    );
  }
  if (load.kind === 'offline' || load.kind === 'failed') {
    return (
      <EmptyState
        icon={PlugZap}
        title={load.kind === 'offline' ? 'Servicio local sin conexión' : 'No se pudo buscar el patrón'}
        description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }
  if (load.kind === 'loading') return <FinderSkeleton />;
  const { data } = load;
  const geometry = data.geometry;
  if (geometry === null) {
    return <EmptyState icon={MapIcon} title="Sin radar para este mapa" description={`Ninguna partida de ${prettyMapName(map)} tiene un análisis con el que dibujar el radar.`} />;
  }

  const multiLevel = isMultiLevelMap(geometry.calibration);
  const levels: RadarLevel[] = multiLevel && floor === 'both' ? [RADAR_LEVELS.default, RADAR_LEVELS.lower] : [floor === 'both' ? RADAR_LEVELS.default : floor];
  const focusBlock = pattern.blocks.find((entry) => entry.kind === 'player');
  const players = [...roster].sort((a, b) => b.last_played_at.localeCompare(a.last_played_at) || b.matches - a.matches);
  const tramo = block === undefined ? WHOLE_ROUND : blockTramo(block);
  const setBlock = (next: PatternBlock): void => setPattern({ ...pattern, blocks: pattern.blocks.map((entry) => (entry === block ? next : entry)) });
  const onArea = (area: PatternArea | null): void => {
    if (block === undefined) return;
    const next: PatternBlock = { ...block };
    if (area === null) delete next.area;
    else next.area = area;
    setBlock(next);
  };
  const blockShape = block?.area === undefined ? shape : areaShape(block.area);
  const teamName = data.team_name ?? '';
  const shownTramo: Tramo = isWholeRound(tramo) ? { from: 0, to: ROUND_CLOCK_SECONDS } : tramo;

  return (
    <div className="grid items-start gap-4 xl:grid-cols-[17rem_minmax(0,1fr)_20rem]">
      <aside className="flex flex-col gap-3">
        <Segmented<TacticalSide>
          label="Lado"
          value={side}
          options={[
            { value: 'T', label: 'Ataque (T)' },
            { value: 'CT', label: 'Defensa (CT)' },
          ]}
          onChange={(next) => setPattern({ ...pattern, side: next })}
        />
        <PatternBlocks
          pattern={pattern}
          active={active}
          onActive={setActive}
          onChange={setPattern}
          onReset={() => {
            setActive(0);
            setPattern(defaultPattern());
          }}
          result={data.result}
          players={players}
          shape={shape}
          onShape={setShape}
          callouts={callouts}
        />
        <SavedPatterns
          map={map}
          teamId={teamId}
          teamName={data.team_name ?? ''}
          scope={runScope}
          version={saved}
          onOpen={(entry) => {
            setActive(0);
            setOpened(entry);
            setPattern(entry.pattern);
            onScopeChange({ limit: entry.scope.matches ?? null, buy: entry.scope.buy ?? '', opponentBuy: entry.scope.opponent_buy ?? '' });
          }}
        />
      </aside>

      <section aria-label="Radar" className="flex min-w-0 flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          {data.maps.length > 1 ? (
            <Segmented<string> label="Mapa" value={data.map} options={data.maps.map((entry) => ({ value: entry.map, label: prettyMapName(entry.map) }))} onChange={onMapChange} />
          ) : null}
          <Segmented<PatternPicture> label="Qué se dibuja" value={picture} options={PICTURES} onChange={setPicture} />
          {multiLevel ? (
            <Segmented<RadarLevel | 'both'>
              label="Piso"
              value={floor}
              options={[{ value: 'both', label: 'Los dos' }, ...[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({ value, label: LEVEL_LABEL[value] }))]}
              onChange={setFloor}
            />
          ) : null}
        </div>
        <PatternRadar
          geometry={geometry}
          levels={levels}
          multiLevel={multiLevel}
          picture={picture}
          overlay={data.overlay ?? null}
          heatmap={data.heatmap ?? null}
          side={side}
          focus={focusBlock?.rival === true ? [] : (focusBlock?.players ?? [])}
          tramo={focusBlock === undefined ? WHOLE_ROUND : blockTramo(focusBlock)}
          area={block?.area ?? null}
          others={pattern.blocks.flatMap((entry) => (entry === block || entry.area === undefined ? [] : [entry.area]))}
          tool={blockShape === 'callouts' ? 'none' : blockShape}
          onArea={onArea}
        />
        {block === undefined ? null : (
          <div role="group" aria-label="Tramo del bloque" className="flex flex-col gap-2 rounded-md border border-border bg-surface-2 px-3 py-2.5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="text-body-sm font-medium text-fg-1">{tramoLabel(tramo)}</span>
              <QuickTramos tramo={tramo} plant={false} onChange={(next) => setBlock(withTramo(block, next))} />
            </div>
            <div className="relative h-7 overflow-hidden rounded-sm bg-surface-0 shadow-[inset_0_0_0_1px_var(--border)]">
              <TramoBracket tramo={shownTramo} duration={ROUND_CLOCK_SECONDS} onChange={(next) => setBlock(withTramo(block, next))} />
            </div>
          </div>
        )}
      </section>

      <aside className="flex flex-col gap-3 xl:sticky xl:top-[calc(var(--shell-strip-height)+3.5rem)] xl:max-h-[calc(100dvh-var(--shell-strip-height)-4.5rem)] xl:overflow-y-auto">
        <PatternResults
          result={data.result}
          timed={timedKind(pattern)}
          title={`${teamName === '' ? 'Patrón' : teamName} en ${prettyMapName(data.map)}`}
          widen={
            block === undefined ? undefined : (
              <>
                {block.invert === true ? null : (
                  <Button type="button" variant="outline" size="sm" onClick={() => setBlock({ ...block, invert: true })}>
                    {INVERT_LABEL[block.kind]}
                  </Button>
                )}
                {pattern.blocks.some((entry) => entry.area !== undefined) ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      setPattern({
                        ...pattern,
                        blocks: pattern.blocks.map((entry) => {
                          const next = { ...entry };
                          delete next.area;
                          return next;
                        }),
                      })
                    }
                  >
                    Quitar zona
                  </Button>
                ) : null}
              </>
            )
          }
          actions={<SavePatternButton map={data.map} teamId={teamId} teamName={teamName} pattern={pattern} scope={saveScope} opened={opened} onSaved={() => setSaved((n) => n + 1)} />}
        />
      </aside>
    </div>
  );
}

function FinderSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Buscando el patrón" className="grid gap-4 xl:grid-cols-[17rem_minmax(0,1fr)_20rem]">
      <Skeleton className="h-80 w-full" />
      <Skeleton className="aspect-square w-full" />
      <Skeleton className="h-80 w-full" />
    </div>
  );
}
