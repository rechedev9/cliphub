'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { MonitorUp, Plus, Table2 } from 'lucide-react';
import type { LibraryPlay } from '@/lib/api/stratbook';
import type { TacticalSide } from '@/lib/api/tactical';
import { playHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { STATUS_LABEL, STATUS_TONE, type StatusTone } from '@/lib/libreta/status';
import { matchModeOpener } from '@/lib/match-mode';
import { PLAN_ALL_TEAMS, PLAN_NO_TEAM, busiestMap, planColumns } from '@/lib/libreta/plan-table';
import { playKindLabel } from '@/lib/stratbook/plays';
import { siteLabel } from '@/lib/tactical-labels';
import { cn } from '@/lib/utils';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { NewPlayDialog, type NewPlayPreset } from './new-play-dialog';
import { mapChoices } from './play-fields';

const SIDES = [
  { value: 'T', label: 'T' },
  { value: 'CT', label: 'CT' },
] as const satisfies readonly { value: TacticalSide; label: string }[];

const TONE_CLASS: Record<StatusTone, string> = {
  success: 'border-success/40 bg-success/10',
  warning: 'border-warning/40 bg-warning/10',
  secondary: 'border-border bg-surface-2',
  stream: 'border-stream/40 bg-stream/10',
  outline: 'border-border bg-surface-1',
};

const DOT_CLASS: Record<StatusTone, string> = {
  success: 'bg-success',
  warning: 'bg-warning',
  secondary: 'bg-fg-4',
  stream: 'bg-stream',
  outline: 'bg-border-strong',
};

/**
 * The Tabla: one map and side of the plan at a glance, a column per round
 * type, so the team sees what it runs and where the gaps are.
 */
export function PlanTable({
  plays,
  onReload,
  compact = false,
  initialMap,
  initialSide = 'T',
  initialTeam = PLAN_ALL_TEAMS,
}: {
  plays: LibraryPlay[] | null;
  onReload: () => void;
  /** The Modo partida window: no team filter and no create buttons. */
  compact?: boolean;
  initialMap?: string;
  initialSide?: TacticalSide;
  initialTeam?: string;
}): ReactNode {
  const [team, setTeam] = useState(initialTeam);
  const [map, setMap] = useState<string | null>(initialMap ?? null);
  const [side, setSide] = useState<TacticalSide>(initialSide);
  // The desktop bridge exists only in Studio's window, so it is read after hydration.
  const [matchMode, setMatchMode] = useState(false);
  useEffect(() => setMatchMode(!compact && matchModeOpener() !== null), [compact]);
  const [tint, setTint] = useState(true);
  const [creating, setCreating] = useState<NewPlayPreset | null>(null);

  if (plays === null) return <Skeleton className="h-64 w-full" />;
  const shownMap = map ?? busiestMap(plays, 'de_mirage');
  const maps = mapChoices(plays.map((play) => play.map));
  const teams = [...new Map(plays.filter((play) => play.team_id !== null).map((play) => [play.team_id ?? '', play.team_name ?? ''])).entries()];
  const columns = planColumns(plays, { team, map: shownMap, side });
  const teamId = team === PLAN_ALL_TEAMS || team === PLAN_NO_TEAM ? undefined : team;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Select value={shownMap} onValueChange={setMap}>
          <SelectTrigger aria-label="Mapa" className="h-9 w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {maps.map((value) => (
              <SelectItem key={value} value={value}>
                {prettyMapName(value)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Segmented label="Lado" value={side} options={SIDES} onChange={setSide} />
        {compact ? null : (
          <Select value={team} onValueChange={setTeam}>
            <SelectTrigger aria-label="Equipo" className="h-9 w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={PLAN_ALL_TEAMS}>Todos los equipos</SelectItem>
              <SelectItem value={PLAN_NO_TEAM}>Sin equipo</SelectItem>
              {teams.map(([id, name]) => (
                <SelectItem key={id} value={id}>
                  {name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        <label className="ml-auto flex items-center gap-2 text-body-sm text-fg-2">
          <input type="checkbox" checked={tint} onChange={(event) => setTint(event.target.checked)} className="size-4 accent-primary" />
          Colorear por estado
        </label>
        {matchMode ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            title="Abre la Tabla y los lineups de este mapa en una ventana pequeña siempre encima, para un segundo monitor"
            onClick={() => void matchModeOpener()?.({ map: shownMap, side, team: team === PLAN_ALL_TEAMS ? '' : team })}
          >
            <MonitorUp aria-hidden />
            Modo partida
          </Button>
        ) : null}
      </div>
      {columns.every((column) => column.plays.length === 0) && !compact ? (
        <EmptyState
          icon={Table2}
          title={`Sin jugadas ${side} en ${prettyMapName(shownMap)}`}
          description="Cada columna es un tipo de ronda. Añade jugadas con el botón + de cada columna o guarda rondas desde la repetición 2D."
        />
      ) : null}
      <div className="grid auto-cols-[minmax(11rem,1fr)] grid-flow-col gap-2 overflow-x-auto pb-1">
        {columns.map((column) => (
          <section key={column.kind} aria-label={playKindLabel(column.kind)} className="studio-panel flex min-h-40 flex-col gap-2 p-2">
            <header className="flex items-center justify-between gap-2">
              <h3 className="text-body-sm font-semibold text-fg-1">
                {playKindLabel(column.kind)} <span className="font-normal text-fg-3 tabular-nums">{column.plays.length}</span>
              </h3>
              {compact ? null : (
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  aria-label={`Nueva jugada de tipo ${playKindLabel(column.kind)}`}
                  onClick={() => setCreating({ map: shownMap, side, kind: column.kind, teamId })}
                >
                  <Plus aria-hidden />
                </Button>
              )}
            </header>
            <ul className="flex flex-col gap-1.5">
              {column.plays.map((play) => {
                const tone = STATUS_TONE[play.status];
                return (
                  <li key={play.id}>
                    <Link
                      href={playHref(play.id)}
                      title={STATUS_LABEL[play.status]}
                      className={cn(
                        'flex items-center gap-2 rounded-md border px-2 py-1.5 text-body-sm text-fg-1 hover:border-primary',
                        tint ? TONE_CLASS[tone] : 'border-border bg-surface-1',
                      )}
                    >
                      <span aria-hidden className={cn('size-2 shrink-0 rounded-full', DOT_CLASS[tone])} />
                      <span className="min-w-0 flex-1 truncate">{play.title}</span>
                      {play.site === 'none' ? null : <span className="text-meta text-fg-3">{siteLabel(play.site)}</span>}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </section>
        ))}
      </div>
      {creating ? (
        <NewPlayDialog preset={creating} knownMaps={maps} onClose={() => setCreating(null)} onCreated={onReload} />
      ) : null}
    </div>
  );
}
