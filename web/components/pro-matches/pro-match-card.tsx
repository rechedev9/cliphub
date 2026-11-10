'use client';

import type { ReactNode } from 'react';
import { CalendarDays, Check, ExternalLink, History, Play, Plus, TriangleAlert } from 'lucide-react';
import { proMapDemos, proMatchTitle, proSeriesVeto, type ProMatch, type ProMatchMap, type ProMatchTeam } from '@/lib/api/pro-matches';
import { matchReplayHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { dayMonthLabel } from '@/lib/library/day-label';
import { MapCell, SeriesStrip, TwoDCell, TwoDLink, mapTag } from '@/components/matches/series-block';
import { Button } from '@/components/ui/button';

export const RADAR_OFF_NOTE = 'El radar de este mapa cambió después de analizar la partida: las posiciones pueden salir algo desplazadas.';
export const OLD_ANALYSIS_NOTE = 'Análisis antiguo: se hizo con una versión anterior de ClipHub y puede faltarle algún dato que hoy se lee de la demo.';
export const HLTV_DEMO_NOTE = 'Para grabar clips necesitas la .dem, que puedes bajar desde la página de la partida en HLTV.';

export type ProMatchCardProps = {
  match: ProMatch;
  /** The map being imported, as `match/entry`, or null. */
  importing: string | null;
  /** The team whose follow is changing, or null. */
  following: number | null;
  onOpen: (match: ProMatch, map: ProMatchMap) => void;
  onToggleFollow: (team: ProMatchTeam) => void;
};

/** One HLTV series as an EDGE block: the strip with format, teams and bans, Seguir and HLTV, then each analysed map with its 2D button. */
export function ProMatchCard({ match, importing, following, onOpen, onToggleFollow }: ProMatchCardProps): ReactNode {
  // A note every map shares shows once for the match, so it does not repeat under each map.
  const allOld = match.maps.length > 0 && match.maps.every((m) => m.old_analysis);
  const allRadarOff = match.maps.length > 0 && match.maps.every((m) => m.radar_may_be_off);
  const veto = proSeriesVeto(match);
  const [first, second] = match.teams;
  return (
    <article className="overflow-hidden border border-border bg-surface-2">
      <SeriesStrip
        format={veto.format}
        title={proMatchTitle(match)}
        teams={first && second && match.teams.length === 2 ? [first.name, second.name] : null}
        score={null}
        bans={veto.bans}
      />
      <div className="flex flex-col gap-1.5 border-b border-border-subtle px-3 py-2">
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-meta text-fg-3">
          <a href={match.hltv_url} target="_blank" rel="noreferrer"
            className="inline-flex items-center gap-1 text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1">
            Ver en HLTV <ExternalLink aria-hidden className="size-3.5" />
          </a>
          <span>{HLTV_DEMO_NOTE}</span>
        </p>
        {allOld ? <OldAnalysisNote /> : null}
        {allRadarOff ? <RadarOffNote /> : null}
        <div className="flex flex-wrap gap-2">
          {match.teams.map((team) => (
            <Button
              key={team.id}
              type="button"
              variant={team.followed ? 'secondary' : 'outline'}
              size="xs"
              aria-pressed={team.followed}
              disabled={following === team.id}
              onClick={() => onToggleFollow(team)}
              title={team.followed ? 'Studio importa sus partidas nuevas. Pulsa para dejar de seguirlo.' : 'Importa sus cinco últimos mapas, los de las dos últimas semanas y los nuevos'}
            >
              {team.followed ? <Check aria-hidden /> : <Plus aria-hidden />}
              {team.followed ? `Sigues a ${team.name}` : `Seguir a ${team.name}`}
              {team.rank ? <span className="text-fg-3 tabular-nums">#{team.rank}</span> : null}
            </Button>
          ))}
        </div>
      </div>
      <ul>
        {match.maps.map((map) => (
          <ProMapRow key={map.entry} match={match} map={map} mark={veto.marks.get(map.map)} notes={{ old: !allOld && map.old_analysis, radar: !allRadarOff && map.radar_may_be_off }}
            busy={importing === `${match.id}/${map.entry}`} disabled={importing !== null} onOpen={() => onOpen(match, map)} />
        ))}
      </ul>
    </article>
  );
}

function OldAnalysisNote(): ReactNode {
  return (
    <p className="flex items-start gap-1.5 text-meta text-fg-3">
      <History aria-hidden className="mt-0.5 size-3.5 shrink-0" />
      {OLD_ANALYSIS_NOTE}
    </p>
  );
}

function RadarOffNote(): ReactNode {
  return (
    <p className="flex items-start gap-1.5 text-meta text-warning">
      <TriangleAlert aria-hidden className="mt-0.5 size-3.5 shrink-0" />
      {RADAR_OFF_NOTE}
    </p>
  );
}

type ProMapRowProps = {
  match: ProMatch;
  map: ProMatchMap;
  mark?: string;
  notes: { old: boolean; radar: boolean };
  busy: boolean;
  disabled: boolean;
  onOpen: () => void;
};

function ProMapRow({ match, map, mark, notes, busy, disabled, onOpen }: ProMapRowProps): ReactNode {
  const demos = proMapDemos(map);
  const name = prettyMapName(map.map);
  const label = `Abrir el replay 2D de ${name}`;
  const teams = match.teams.length === 2 ? match.teams.map((team) => team.name).join(' vs ') : null;
  const details = [teams, demos, `${map.rounds} rondas`].filter((part) => part !== null);
  return (
    <li className="flex flex-col transition-colors hover:bg-surface-3 [&:not(:first-child)]:border-t [&:not(:first-child)]:border-border-subtle">
      <div className="flex items-stretch">
        <MapCell map={map.map} day={dayMonthLabel(match.played_at)} name={name} tag={mark ? mapTag(mark) : undefined} />
        <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-6 gap-y-1 px-3 py-2">
          <p className="flex min-w-40 flex-wrap gap-x-2 text-body-sm text-fg-1">
            {details.map((part, i) => (
              <span key={part} className={i === 0 && teams !== null ? 'font-semibold' : 'text-fg-3 tabular-nums'}>
                {i === 0 ? '' : '· '}
                {part}
              </span>
            ))}
          </p>
          {match.event ? (
            <p className="flex min-w-0 items-center gap-1.5 text-body-sm font-semibold text-fg-2">
              <CalendarDays aria-hidden className="size-3.5 shrink-0 text-fg-3" />
              <span className="truncate">{match.event}</span>
            </p>
          ) : null}
        </div>
        <TwoDCell>
          {map.library_match_id ? (
            <TwoDLink href={matchReplayHref(map.library_match_id)} label={label} />
          ) : (
            <Button type="button" size="xs" aria-label={busy ? `Importando ${name}…` : label} aria-busy={busy} disabled={disabled} onClick={onOpen}>
              <Play aria-hidden />
              {busy ? 'Importando…' : '2D'}
            </Button>
          )}
        </TwoDCell>
      </div>
      {notes.old || notes.radar ? (
        <div className="flex flex-col gap-1 px-3 pb-2">
          {notes.old ? <OldAnalysisNote /> : null}
          {notes.radar ? <RadarOffNote /> : null}
        </div>
      ) : null}
    </li>
  );
}
