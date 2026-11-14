'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { EllipsisVertical, FileText, TriangleAlert, Video } from 'lucide-react';
import type { LibrarySource } from '@/lib/api/library';
import type { LibrarySeries } from '@/lib/api/veto';
import { matchHref, matchReplayHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { dayMonthLabel } from '@/lib/library/day-label';
import { SERIES_FORMAT_LABEL } from '@/lib/library/veto';
import { hasPov, ownOutcome, seriesScore, seriesVeto, type DayGroup, type PartidaGroup, type VetoMark } from '@/lib/library/partida-groups';
import type { VoiceByMatch } from '@/lib/library/voice-summary';
import { OUTCOME_LABEL, matchTitle, partidaDate, type Outcome, type PartidaRow } from '@/lib/library/partidas';
import { DayHeading, MapCell, SeriesStrip, TwoDCell, TwoDLink, mapTag, type MapEdge, type MapTag } from '@/components/matches/series-block';
import { DemoStatus, RowActions, ScoreStack, type RowHandlers } from '@/components/matches/partida-cells';
import { PartidaSource } from '@/components/matches/partida-source';
import { PartidaTags } from '@/components/matches/partida-tags';
import { PartidaVoice } from '@/components/matches/partida-voice';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

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

function markTagOf(mark: VetoMark): MapTag {
  if (mark.action === 'decider') return mapTag('DECIDER');
  return mapTag(mark.team === '' ? 'PICK' : `PICK de ${mark.team}`);
}

const EDGE_OF_OUTCOME: Record<Outcome, MapEdge> = { win: 'win', loss: 'loss', draw: 'none' };

/** The row kebab: the way into the record of the partida, which the 2D button skips. */
function RowMenu({ matchId, name }: { matchId: string; name: string }): ReactNode {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="ghost" size="icon-xs" aria-label={`Más acciones de ${name}`}>
          <EllipsisVertical aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem asChild>
          <Link href={matchHref(matchId)}>
            <FileText aria-hidden />
            Detalles de la demo
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** One map of the library: result edge, name with its veto tag, scores, POV, voice, source, tags and the 2D cell. */
function MapRow({ row, mark, context }: { row: PartidaRow; mark?: VetoMark; context: GroupListContext }): ReactNode {
  const { demo, match } = row;
  const outcome = ownOutcome(row, context.ownTeam);
  const pov = hasPov(row, context.povJobs);
  const voice = match ? context.voices.get(match.id) : undefined;
  const edgeLabel = outcome === null ? undefined : `Resultado de ${context.ownName}: ${OUTCOME_LABEL[outcome]}`;
  const name = match ? prettyMapName(match.map) : demo.file_name;
  const failed = demo.state === 'failed' ? <TriangleAlert aria-hidden className="size-4 shrink-0 text-destructive" /> : null;
  const label = (
    <span className="flex items-center gap-1.5">
      {failed}
      <span className="truncate" title={match ? matchTitle(match) : demo.file_name}>
        {name}
      </span>
    </span>
  );
  // Under 40rem of content the row stacks: the map fills the first line, the details take the next and the buttons sit at the right.
  return (
    <li
      title={edgeLabel}
      className="relative flex items-stretch border-t border-border-subtle transition-colors first:border-t-0 hover:bg-surface-3 @max-[40rem]/content:flex-wrap @max-[40rem]/content:justify-end"
    >
      <div className="flex w-8 shrink-0 items-center justify-center">
        <input
          type="checkbox"
          aria-label={`Elegir ${name}`}
          checked={context.selected.has(demo.id)}
          onChange={() => context.onToggle(demo.id)}
          className="size-4 accent-primary"
        />
      </div>
      {edgeLabel ? <span className="sr-only">{edgeLabel}</span> : null}
      <MapCell
        map={match ? match.map : null}
        day={dayMonthLabel(partidaDate(row))}
        name={
          match ? (
            <Link href={matchHref(match.id)} className="block rounded-sm outline-none after:absolute after:inset-0 focus-visible:after:ring-2 focus-visible:after:ring-ring">
              {label}
            </Link>
          ) : (
            label
          )
        }
        tag={mark ? markTagOf(mark) : undefined}
        edge={outcome === null ? 'none' : EDGE_OF_OUTCOME[outcome]}
        className="@max-[40rem]/content:grow"
      />
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-4 gap-y-1.5 px-3 py-2 @max-[40rem]/content:basis-full">
        {match && demo.state !== 'failed' ? (
          <ScoreStack match={match} />
        ) : (
          <div className="min-w-40">
            {/* A phone has no hover for the tooltip, so the reason wraps there. */}
            <DemoStatus row={row} className="@max-[40rem]/content:whitespace-normal" />
          </div>
        )}
        <span aria-hidden className="hidden h-8 w-px bg-border @min-[44rem]/content:block" />
        <div className="flex flex-wrap items-center gap-2">
          {pov ? (
            <Badge variant="secondary" shape="square">
              <Video aria-hidden />
              POV
            </Badge>
          ) : null}
          {voice ? <PartidaVoice summary={voice} onResynced={context.onVoiceResynced} /> : null}
          <PartidaSource demo={demo} onChange={context.onSourceChange} />
          <PartidaTags demo={demo} allTags={context.allTags} onChanged={context.onTagsChanged} />
        </div>
      </div>
      {match && demo.state === 'ready' ? (
        <TwoDCell className="@max-[40rem]/content:border-l-0 @max-[40rem]/content:pb-2">
          <TwoDLink href={matchReplayHref(match.id)} label={`Abrir el replay 2D de ${name}`} />
        </TwoDCell>
      ) : null}
      <div className="flex shrink-0 items-center gap-1 px-2 @max-[40rem]/content:pb-2 @max-[40rem]/content:empty:hidden">
        <RowActions row={row} handlers={context.handlers} />
        {match ? <RowMenu matchId={match.id} name={name} /> : null}
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

/** A series: the strip with format, teams, score and bans, then its maps with how each was picked. */
function SeriesBlock({ series, rows, context }: { series: LibrarySeries; rows: readonly PartidaRow[]; context: GroupListContext }): ReactNode {
  const veto = seriesVeto(series);
  const score = seriesScore(series, context.libraryRows.filter((row) => row.match?.series_id === series.id));
  const a = series.teams.find((team) => team.slot === 'a');
  const b = series.teams.find((team) => team.slot === 'b');
  return (
    <section aria-label={seriesTitle(series, rows)} className="overflow-hidden border border-border bg-surface-2">
      <SeriesStrip
        format={SERIES_FORMAT_LABEL[series.format]}
        title={seriesTitle(series, rows)}
        teams={a && b ? [a.name, b.name] : null}
        score={score}
        event={series.event}
        bans={veto.bans}
      />
      <ul>
        {rows.map((row) => (
          <MapRow key={row.demo.id} row={row} mark={row.match ? veto.marks.get(row.match.map) : undefined} context={context} />
        ))}
      </ul>
    </section>
  );
}

function SingleBlock({ row, context }: { row: PartidaRow; context: GroupListContext }): ReactNode {
  return (
    <div className="overflow-hidden border border-border bg-surface-2">
      <ul>
        <MapRow row={row} context={context} />
      </ul>
    </div>
  );
}

function GroupBlock({ group, context }: { group: PartidaGroup; context: GroupListContext }): ReactNode {
  if (group.kind === 'single') return <SingleBlock row={group.row} context={context} />;
  return <SeriesBlock series={group.series} rows={group.rows} context={context} />;
}

/** Partidas grouped by day under slash headings, each series as one block with its bans and the pick of every map. */
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
        <section key={day.day} className="flex flex-col gap-2">
          <DayHeading day={day.day} />
          {day.groups.map((group) => (
            <GroupBlock key={group.kind === 'series' ? group.series.id : group.row.demo.id} group={group} context={context} />
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
