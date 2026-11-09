'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { Check, MonitorPlay, Plus, TriangleAlert } from 'lucide-react';
import { proMapPart, proMatchTitle, type ProMatch, type ProMatchMap, type ProMatchTeam } from '@/lib/api/pro-matches';
import { prettyMapName } from '@/lib/format';
import { Button } from '@/components/ui/button';

export const RADAR_OFF_NOTE = 'El radar de este mapa cambió después de analizar la partida: las posiciones pueden salir algo desplazadas.';

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
  return (
    <article className="studio-panel flex flex-col gap-3 p-4">
      <div className="flex flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
        <h3 className="text-body font-semibold text-fg-1">{proMatchTitle(match)}</h3>
        <p className="text-meta text-fg-3">{match.event}</p>
      </div>
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
            title={team.followed ? 'Studio importa sus partidas nuevas. Pulsa para dejar de seguirlo.' : 'Importa sus partidas de las dos últimas semanas y las nuevas'}
          >
            {team.followed ? <Check aria-hidden /> : <Plus aria-hidden />}
            {team.followed ? `Sigues a ${team.name}` : `Seguir a ${team.name}`}
            {team.rank ? <span className="text-fg-3 tabular-nums">#{team.rank}</span> : null}
          </Button>
        ))}
      </div>
      <ul className="flex flex-col divide-y divide-border">
        {match.maps.map((map) => (
          <ProMapRow key={map.entry} map={map} busy={importing === `${match.id}/${map.entry}`} disabled={importing !== null} onOpen={() => onOpen(match, map)} />
        ))}
      </ul>
    </article>
  );
}

function ProMapRow({ map, busy, disabled, onOpen }: { map: ProMatchMap; busy: boolean; disabled: boolean; onOpen: () => void }): ReactNode {
  const part = proMapPart(map.entry);
  return (
    <li className="flex flex-col gap-1.5 py-2 first:pt-0 last:pb-0">
      <div className="flex items-center justify-between gap-3">
        <span className="text-body-sm text-fg-1">
          {prettyMapName(map.map)}
          {part ? <span className="text-fg-3"> · {part}</span> : null}
          <span className="text-fg-3 tabular-nums"> · {map.rounds} rondas</span>
        </span>
        {map.library_match_id ? (
          <Button asChild variant="outline" size="sm">
            <Link href={`/partidas/${encodeURIComponent(map.library_match_id)}?tab=replay`}>
              <MonitorPlay aria-hidden />
              Abrir en el Replay
            </Link>
          </Button>
        ) : (
          <Button type="button" size="sm" disabled={disabled} onClick={onOpen}>
            <MonitorPlay aria-hidden />
            {busy ? 'Importando…' : 'Abrir en el Replay'}
          </Button>
        )}
      </div>
      {map.radar_may_be_off ? (
        <p className="flex items-start gap-1.5 text-meta text-warning">
          <TriangleAlert aria-hidden className="mt-0.5 size-3.5 shrink-0" />
          {RADAR_OFF_NOTE}
        </p>
      ) : null}
    </li>
  );
}
