'use client';

import type { ReactNode } from 'react';
import { CalendarDays, Check, EllipsisVertical, ExternalLink, History, Play, Plus, TriangleAlert } from 'lucide-react';
import { proMapDemos, proMatchTitle, proSeriesVeto, type ProMatch, type ProMatchMap, type ProMatchTeam } from '@/lib/api/pro-matches';
import { matchReplayHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { dayMonthLabel } from '@/lib/library/day-label';
import { MapCell, SeriesStrip, TwoDCell, TwoDLink, mapTag } from '@/components/matches/series-block';
import { TeamLogo } from '@/components/matches/team-logo';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

export const RADAR_OFF_NOTE = 'El radar de este mapa cambió después de analizar la partida: las posiciones pueden salir algo desplazadas.';
export const OLD_ANALYSIS_NOTE = 'Análisis antiguo: se hizo con una versión anterior de ClipHub y puede faltarle algún dato que hoy se lee de la demo.';
export const HLTV_DEMO_NOTE = 'Para grabar clips necesitas la .dem, que puedes bajar desde la página de la partida en HLTV.';

const FOLLOW_HINT = 'Importa sus cinco últimos mapas, los de las dos últimas semanas y los nuevos';

export type ProMatchCardProps = {
  match: ProMatch;
  /** The map being imported, as `match/entry`, or null. */
  importing: string | null;
  /** The team whose follow is changing, or null. */
  following: number | null;
  onOpen: (match: ProMatch, map: ProMatchMap) => void;
  onToggleFollow: (team: ProMatchTeam) => void;
};

/** One HLTV series as an EDGE block: the strip with format, teams, bans and the series menu, then each analysed map with its 2D button. */
export function ProMatchCard({ match, importing, following, onOpen, onToggleFollow }: ProMatchCardProps): ReactNode {
  const veto = proSeriesVeto(match);
  const [first, second] = match.teams;
  const followed = match.teams.filter((team) => team.followed);
  return (
    <article className="overflow-hidden border border-border bg-surface-2">
      <SeriesStrip
        format={veto.format}
        title={proMatchTitle(match)}
        teams={first && second && match.teams.length === 2 ? [first.name, second.name] : null}
        score={null}
        bans={veto.bans}
        actions={
          <>
            {followed.length > 0 ? (
              <Badge variant="secondary" shape="square" title={`Studio importa las partidas nuevas de ${followed.map((team) => team.name).join(' y ')}`}>
                <Check aria-hidden />
                {followed.length === 1 ? `Sigues a ${followed[0]?.name ?? ''}` : 'Sigues a los dos'}
              </Badge>
            ) : null}
            <SeriesMenu match={match} following={following} onToggleFollow={onToggleFollow} />
          </>
        }
      />
      <ul>
        {match.maps.map((map) => (
          <ProMapRow key={map.entry} match={match} map={map} mark={veto.marks.get(map.map)} busy={importing === `${match.id}/${map.entry}`} disabled={importing !== null} onOpen={() => onOpen(match, map)} />
        ))}
      </ul>
    </article>
  );
}

/** The series kebab: HLTV, following each team and where the .dem comes from, out of the rows' way. */
function SeriesMenu({ match, following, onToggleFollow }: Pick<ProMatchCardProps, 'match' | 'following' | 'onToggleFollow'>): ReactNode {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="ghost" size="icon-xs" aria-label={`Más acciones de ${proMatchTitle(match)}`}>
          <EllipsisVertical aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-72">
        <DropdownMenuItem asChild>
          <a href={match.hltv_url} target="_blank" rel="noreferrer">
            <ExternalLink aria-hidden />
            Ver en HLTV
          </a>
        </DropdownMenuItem>
        {match.teams.length > 0 ? <DropdownMenuSeparator /> : null}
        {match.teams.map((team) => (
          <DropdownMenuItem key={team.id} disabled={following === team.id} title={team.followed ? 'Studio importa sus partidas nuevas' : FOLLOW_HINT} onSelect={() => onToggleFollow(team)}>
            {team.followed ? <Check aria-hidden /> : <Plus aria-hidden />}
            <span className="min-w-0 flex-1 truncate">{team.followed ? `Dejar de seguir a ${team.name}` : `Seguir a ${team.name}`}</span>
            {team.rank ? <span className="text-meta text-fg-3 tabular-nums">#{team.rank}</span> : null}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuLabel className="text-meta font-normal">{HLTV_DEMO_NOTE}</DropdownMenuLabel>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** A note on one map as an icon whose tooltip holds the text; the text also stays for screen readers. */
function MapNote({ icon, text, warn = false }: { icon: ReactNode; text: string; warn?: boolean }): ReactNode {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span tabIndex={0} className={warn ? 'text-warning' : 'text-fg-3'}>
          {icon}
          <span className="sr-only">{text}</span>
        </span>
      </TooltipTrigger>
      <TooltipContent className="max-w-72">{text}</TooltipContent>
    </Tooltip>
  );
}

type ProMapRowProps = {
  match: ProMatch;
  map: ProMatchMap;
  mark?: string;
  busy: boolean;
  disabled: boolean;
  onOpen: () => void;
};

function ProMapRow({ match, map, mark, busy, disabled, onOpen }: ProMapRowProps): ReactNode {
  const demos = proMapDemos(map);
  const name = prettyMapName(map.map);
  const label = `Abrir el replay 2D de ${name}`;
  return (
    <li className="flex items-stretch transition-colors hover:bg-surface-3 [&:not(:first-child)]:border-t [&:not(:first-child)]:border-border-subtle">
      <MapCell map={map.map} day={dayMonthLabel(match.played_at)} name={name} tag={mark ? mapTag(mark) : undefined} />
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-6 gap-y-1 px-3 py-2">
        {match.teams.length > 0 ? (
          <span className="flex w-44 min-w-0 flex-col gap-0.5 text-body-sm font-semibold text-fg-1">
            {match.teams.map((team) => (
              <span key={team.id} className="flex min-w-0 items-center gap-2">
                <TeamLogo name={team.name} className="size-4" />
                <span className="truncate">{team.name}</span>
              </span>
            ))}
          </span>
        ) : null}
        <span className="flex w-36 items-center gap-2 text-meta text-fg-3 tabular-nums">
          {map.rounds} rondas
          {demos ? <span>· {demos}</span> : null}
          {map.old_analysis ? <MapNote icon={<History aria-hidden className="size-3.5" />} text={OLD_ANALYSIS_NOTE} /> : null}
          {map.radar_may_be_off ? <MapNote icon={<TriangleAlert aria-hidden className="size-3.5" />} text={RADAR_OFF_NOTE} warn /> : null}
        </span>
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
    </li>
  );
}
