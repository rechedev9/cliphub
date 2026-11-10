'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { Check, ExternalLink, History, MonitorPlay, Plus, TriangleAlert } from 'lucide-react';
import { proMapDemos, proMatchTitle, proSeriesVeto, type ProMatch, type ProMatchMap, type ProMatchTeam } from '@/lib/api/pro-matches';
import { prettyMapName } from '@/lib/format';
import { MapThumb } from '@/components/analysis/map-thumb';
import { Badge } from '@/components/ui/badge';
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

/** One HLTV match: its teams with Seguir, and each analysed map with its way into the Replay. */
export function ProMatchCard({ match, importing, following, onOpen, onToggleFollow }: ProMatchCardProps): ReactNode {
  // A note every map shares shows once for the match, so it does not repeat under each map.
  const allOld = match.maps.length > 0 && match.maps.every((m) => m.old_analysis);
  const allRadarOff = match.maps.length > 0 && match.maps.every((m) => m.radar_may_be_off);
  const veto = proSeriesVeto(match);
  return (
    <article className="studio-panel overflow-hidden">
      <header className="flex flex-col gap-2 border-b border-border-subtle bg-surface-2 px-3 py-2">
      <div className="flex flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
        <div className="flex flex-wrap items-center gap-2">
          {veto.format ? <Badge variant="secondary" shape="square">{veto.format}</Badge> : null}
          <h3 className="text-body font-semibold text-fg-1">{proMatchTitle(match)}</h3>
        </div>
        <p className="text-meta text-fg-3">{match.event}</p>
      </div>
      {veto.bans.length > 0 ? (
        <p className="text-meta text-fg-3">
          <span className="font-medium tracking-wide">BANS</span>{' '}
          {veto.bans.map((ban) => (ban.team === '' ? prettyMapName(ban.map) : `${prettyMapName(ban.map)} (${ban.team})`)).join(', ')}
        </p>
      ) : null}
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
            size="sm"
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
      </header>
      <ul>
        {match.maps.map((map) => (
          <ProMapRow key={map.entry} map={map} teams={match.teams} mark={veto.marks.get(map.map)} notes={{ old: !allOld && map.old_analysis, radar: !allRadarOff && map.radar_may_be_off }}
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
  map: ProMatchMap;
  teams: readonly ProMatchTeam[];
  mark?: string;
  notes: { old: boolean; radar: boolean };
  busy: boolean;
  disabled: boolean;
  onOpen: () => void;
};

function ProMapRow({ map, teams, mark, notes, busy, disabled, onOpen }: ProMapRowProps): ReactNode {
  const demos = proMapDemos(map);
  const name = prettyMapName(map.map);
  const label = `Abrir el replay 2D de ${name}`;
  return (
    <li className="flex flex-col gap-1.5 px-3 py-2.5 transition-colors hover:bg-surface-2 [&:not(:first-child)]:border-t [&:not(:first-child)]:border-border-subtle">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <MapThumb map={map.map} className="h-10 w-16" crop={1.5} />
        <div className="flex w-44 shrink-0 flex-col">
          <span className="font-semibold">{name}</span>
          {mark ? <span className="text-meta font-medium tracking-wide text-fg-3">{mark}</span> : null}
        </div>
        <div className="min-w-56 flex-1 text-body-sm text-fg-2">
          {teams.map((team) => team.name).join(' vs ')}
          {demos ? <span className="text-fg-3"> · {demos}</span> : null}
          <span className="text-fg-3 tabular-nums"> · {map.rounds} rondas</span>
        </div>
        {map.library_match_id ? (
          <Button asChild variant="outline" size="xs">
            <Link href={`/partidas/${encodeURIComponent(map.library_match_id)}?tab=replay`} aria-label={label}>
              <MonitorPlay aria-hidden />
              2D
            </Link>
          </Button>
        ) : (
          <Button type="button" variant="outline" size="xs" aria-label={label} disabled={disabled} onClick={onOpen}>
            <MonitorPlay aria-hidden />
            {busy ? 'Importando…' : '2D'}
          </Button>
        )}
      </div>
      {notes.old ? <OldAnalysisNote /> : null}
      {notes.radar ? <RadarOffNote /> : null}
    </li>
  );
}
