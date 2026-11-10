'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { MonitorPlay, TriangleAlert, Video } from 'lucide-react';
import type { LibrarySource } from '@/lib/api/library';
import type { LibrarySeries } from '@/lib/api/veto';
import { matchHref, matchReplayHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { SERIES_FORMAT_LABEL } from '@/lib/library/veto';
import { hasPov, ownOutcome, seriesScore, seriesVeto, type DayGroup, type PartidaGroup, type VetoMark } from '@/lib/library/partida-groups';
import type { VoiceByMatch } from '@/lib/library/voice-summary';
import { OUTCOME_LABEL, matchTitle, type Outcome, type PartidaRow } from '@/lib/library/partidas';
import { cn } from '@/lib/utils';
import { MapThumb } from '@/components/analysis/map-thumb';
import { DemoStatus, RowActions, Scoreline, type RowHandlers } from '@/components/matches/partida-cells';
import { PartidaSource } from '@/components/matches/partida-source';
import { PartidaTags } from '@/components/matches/partida-tags';
import { PartidaVoice } from '@/components/matches/partida-voice';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';

const OUTCOME_EDGE: Record<Outcome, string> = {
  win: 'border-l-success',
  loss: 'border-l-destructive',
  draw: 'border-l-transparent',
};

const DAY_FORMAT = new Intl.DateTimeFormat('es-ES', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });

/** "lunes, 5 de octubre de 2026" for a YYYY-MM-DD day; the raw day when it does not parse. */
function dayHeading(day: string): string {
  const at = new Date(`${day}T12:00:00`);
  return Number.isNaN(at.getTime()) ? day : DAY_FORMAT.format(at);
}

/** Groups drawn before "Mostrar más", so a long library does not paint every row at once. */
const GROUPS_PER_PAGE = 40;

export type GroupListContext = {
  /** Every library row, unfiltered, so a series score counts maps a filter hides. */
  libraryRows: readonly PartidaRow[];
  ownTeam: string | null;
  /** The name of the own team, for the label of the row edge. */
  ownName: string;
  povJobs: ReadonlySet<string>;
  voices: VoiceByMatch;
  onVoiceResynced: () => void;
  allTags: readonly string[];
  handlers: RowHandlers;
  selected: ReadonlySet<string>;
  onToggle: (demoId: string) => void;
  onTagsChanged: (demoId: string, tags: string[]) => void;
  onSourceChange: (demoId: string, source: LibrarySource) => void;
};

function markLabel(mark: VetoMark): string {
  if (mark.action === 'decider') return 'DECIDER';
  return mark.team === '' ? 'PICK' : `PICK de ${mark.team}`;
}

/** One map of the library: result edge, name with its veto mark, score, POV, tags and the 2D button. */
function MapRow({ row, mark, context }: { row: PartidaRow; mark?: VetoMark; context: GroupListContext }): ReactNode {
  const { demo, match } = row;
  const outcome = ownOutcome(row, context.ownTeam);
  const pov = hasPov(row, context.povJobs);
  const voice = match ? context.voices.get(match.id) : undefined;
  const edgeLabel = outcome === null ? undefined : `Resultado de ${context.ownName}: ${OUTCOME_LABEL[outcome]}`;
  const name = match ? prettyMapName(match.map) : demo.file_name;
  const failed = demo.state === 'failed' ? <TriangleAlert aria-hidden className="size-4 shrink-0 text-destructive" /> : null;
  return (
    <li
      title={edgeLabel}
      className={cn(
        'relative flex flex-wrap items-center gap-x-4 gap-y-2 border-l-2 px-3 py-2.5 transition-colors hover:bg-surface-2',
        OUTCOME_EDGE[outcome ?? 'draw'],
      )}
    >
      <input
        type="checkbox"
        aria-label={`Elegir ${name}`}
        checked={context.selected.has(demo.id)}
        onChange={() => context.onToggle(demo.id)}
        className="relative z-10 size-4 accent-primary"
      />
      {edgeLabel ? <span className="sr-only">{edgeLabel}</span> : null}
      {match ? <MapThumb map={match.map} className="h-10 w-16" crop={1.5} /> : null}
      <div className="flex w-44 shrink-0 flex-col">
        <span className="flex items-center gap-2 font-semibold">
          {failed}
          {match ? (
            <Link href={matchHref(match.id)} title={matchTitle(match)} className="rounded-sm outline-none after:absolute after:inset-0 focus-visible:after:ring-2 focus-visible:after:ring-ring">
              {name}
            </Link>
          ) : (
            <span className="truncate text-fg-2" title={demo.file_name}>{name}</span>
          )}
        </span>
        {mark ? <span className="text-meta font-medium tracking-wide text-fg-3">{markLabel(mark)}</span> : null}
      </div>
      <div className="min-w-56 flex-1">
        {match && demo.state !== 'failed' ? <Scoreline match={match} /> : <DemoStatus row={row} />}
      </div>
      {pov ? (
        <Badge variant="accent" shape="square">
          <Video aria-hidden />
          POV
        </Badge>
      ) : null}
      {voice ? (
        <div className="relative z-10">
          <PartidaVoice summary={voice} onResynced={context.onVoiceResynced} />
        </div>
      ) : null}
      <div className="relative z-10">
        <PartidaSource demo={demo} onChange={context.onSourceChange} />
      </div>
      <div className="relative z-10">
        <PartidaTags demo={demo} allTags={context.allTags} onChanged={context.onTagsChanged} />
      </div>
      <div className="relative z-10 flex items-center gap-1">
        {match && demo.state === 'ready' ? (
          <Button asChild variant="outline" size="xs">
            <Link href={matchReplayHref(match.id)} aria-label={`Abrir el replay 2D de ${name}`}>
              <MonitorPlay aria-hidden />
              2D
            </Link>
          </Button>
        ) : null}
        <RowActions row={row} handlers={context.handlers} />
      </div>
    </li>
  );
}

function seriesTitle(series: LibrarySeries, rows: readonly PartidaRow[]): string {
  const [a, b] = series.teams;
  if (a && b) return `${a.name} vs ${b.name}`;
  const first = rows.find((row) => row.match !== null)?.match;
  return first ? matchTitle(first) : series.event || `Serie ${SERIES_FORMAT_LABEL[series.format]}`;
}

/** A series: its title, format and event, the bans, and its maps with how each was picked. */
function SeriesCard({ series, rows, context }: { series: LibrarySeries; rows: readonly PartidaRow[]; context: GroupListContext }): ReactNode {
  const veto = seriesVeto(series);
  const score = seriesScore(series, context.libraryRows.filter((row) => row.match?.series_id === series.id));
  return (
    <section aria-label={seriesTitle(series, rows)} className="studio-panel overflow-hidden">
      <header className="flex flex-col gap-1 border-b border-border-subtle bg-surface-2 px-3 py-2">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="secondary" shape="square">{SERIES_FORMAT_LABEL[series.format]}</Badge>
          <h3 className="text-body font-semibold">{seriesTitle(series, rows)}</h3>
          {score ? (
            <span className="text-body font-semibold tabular-nums">
              {score[0]}
              <span aria-hidden className="px-1.5 text-fg-3">:</span>
              <span className="sr-only"> a </span>
              {score[1]}
            </span>
          ) : null}
          {series.event ? <span className="truncate text-body-sm text-fg-2">{series.event}</span> : null}
        </div>
        {veto.bans.length > 0 ? (
          <p className="text-meta text-fg-3">
            <span className="font-medium tracking-wide">BANS</span>{' '}
            {veto.bans.map((ban) => (ban.team === '' ? prettyMapName(ban.map) : `${prettyMapName(ban.map)} (${ban.team})`)).join(', ')}
          </p>
        ) : null}
      </header>
      <ul>
        {rows.map((row) => (
          <MapRow key={row.demo.id} row={row} mark={row.match ? veto.marks.get(row.match.map) : undefined} context={context} />
        ))}
      </ul>
    </section>
  );
}

function SingleCard({ row, context }: { row: PartidaRow; context: GroupListContext }): ReactNode {
  return (
    <div className="studio-panel overflow-hidden">
      <ul>
        <MapRow row={row} context={context} />
      </ul>
    </div>
  );
}

function GroupCard({ group, context }: { group: PartidaGroup; context: GroupListContext }): ReactNode {
  if (group.kind === 'single') return <SingleCard row={group.row} context={context} />;
  return <SeriesCard series={group.series} rows={group.rows} context={context} />;
}

/** Partidas grouped by day, each series as one card with its bans and the pick of every map. */
export function PartidaGroupsList({ days, context }: { days: readonly DayGroup[]; context: GroupListContext }): ReactNode {
  const [limit, setLimit] = useState(GROUPS_PER_PAGE);
  if (days.length === 0) return <p className="text-body-sm text-fg-2">Ninguna partida cumple estos filtros.</p>;
  const total = days.reduce((sum, day) => sum + day.groups.length, 0);
  let left = limit;
  const visible = days.flatMap((day) => {
    const groups = day.groups.slice(0, Math.max(left, 0));
    left -= groups.length;
    return groups.length === 0 ? [] : [{ day: day.day, groups }];
  });
  return (
    <div className="flex flex-col gap-5">
      {visible.map((day) => (
        <section key={day.day} aria-label={dayHeading(day.day)} className="flex flex-col gap-2">
          <h2 className="text-body-sm font-medium text-fg-2 first-letter:uppercase">{dayHeading(day.day)}</h2>
          {day.groups.map((group) => (
            <GroupCard key={group.kind === 'series' ? group.series.id : group.row.demo.id} group={group} context={context} />
          ))}
        </section>
      ))}
      {total > limit ? (
        <Button type="button" variant="outline" size="sm" className="self-center" onClick={() => setLimit(limit + GROUPS_PER_PAGE)}>
          Mostrar más ({total - limit} sin ver)
        </Button>
      ) : null}
    </div>
  );
}
