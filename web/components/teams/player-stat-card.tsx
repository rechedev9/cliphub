'use client';

import type { ReactNode } from 'react';
import { LIBRARY_ROLES } from '@/lib/api/library';
import type { ReportSide } from '@/lib/api/scout';
import type { SideStatsPlayer } from '@/lib/api/side-stats';
import { mapRoleChips } from '@/lib/analysis/map-role';
import { RATING_FORMULA_TEXT, RATING_LABEL } from '@/lib/analysis/rating';
import { formatDecimal } from '@/lib/format';
import { ROLE_LABEL } from '@/lib/library/partidas';
import { InfoTip } from '@/components/analysis/info-tip';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { RoleChip } from '@/components/ui/role-chip';

function roleText(role: string): string {
  const known = LIBRARY_ROLES.find((entry) => entry === role);
  return known === undefined ? ROLE_LABEL.unknown : ROLE_LABEL[known];
}

function SideLabel({ side }: { side: ReportSide }): ReactNode {
  if (side === 'CT') return <span className="font-semibold text-side-ct">CT</span>;
  if (side === 'T') return <span className="font-semibold text-side-t">T</span>;
  return (
    <span className="font-semibold">
      <span className="text-side-ct">CT</span> y <span className="text-side-t">T</span>
    </span>
  );
}

function RoleChips({ player, side }: { player: SideStatsPlayer; side: ReportSide }): ReactNode {
  const chips = mapRoleChips(player.map_roles, side);
  if (chips.length === 0) {
    return (
      <p className="flex flex-wrap items-center gap-1 text-meta text-fg-2">
        <SideLabel side={side} />
        {player.role === 'unknown' ? null : <RoleChip label={roleText(player.role)} />}
      </p>
    );
  }
  return (
    <p className="flex flex-wrap items-center gap-1">
      {chips.map((chip) => (
        <RoleChip key={chip.side} side={chip.side} label={chip.label} />
      ))}
      {player.role === 'unknown' ? null : <RoleChip label={roleText(player.role)} />}
    </p>
  );
}

/**
 * A player's card in the EDGE Jugadores row: the nick and the avatar (FACEIT's when known, else the
 * initial), the role chips, the big rating and the 2x2 of K/D, ADR, KPR and DPR.
 */
export function PlayerStatCard({ player, side }: { player: SideStatsPlayer; side: ReportSide }): ReactNode {
  const name = player.name || player.steamid64;
  const lines = [
    { label: 'K/D', value: formatDecimal(player.kd, 2) },
    { label: 'ADR', value: formatDecimal(player.adr, 1) },
    { label: 'KPR', value: formatDecimal(player.kpr, 2) },
    { label: 'DPR', value: formatDecimal(player.dpr, 2) },
  ];
  return (
    <li className="flex min-w-0 flex-col gap-2 border border-border bg-surface-2 p-3">
      <div className="flex min-w-0 items-start justify-between gap-2">
        <div className="flex min-w-0 flex-col gap-1">
          <p className="truncate text-body font-bold text-fg-1">{name}</p>
          <RoleChips player={player} side={side} />
        </div>
        <PlayerAvatar nickname={name} playerID={player.steamid64} size={32} />
      </div>
      <p className="flex items-baseline gap-1.5">
        <span className="text-stat leading-none font-bold text-fg-1 tabular-nums">{formatDecimal(player.rating, 2)}</span>
        <span className="text-meta font-medium tracking-wider text-fg-3 uppercase">{RATING_LABEL}</span>
        <InfoTip label={RATING_LABEL} text={RATING_FORMULA_TEXT} />
      </p>
      <dl className="grid grid-cols-2 gap-x-3 gap-y-1.5">
        {lines.map((line) => (
          <div key={line.label} className="flex flex-col">
            <dt className="text-meta font-medium tracking-wider text-fg-3 uppercase">{line.label}</dt>
            <dd className="text-body font-bold text-fg-1 tabular-nums">{line.value}</dd>
          </div>
        ))}
      </dl>
      <p className="text-meta text-fg-3 tabular-nums">{player.rounds === 1 ? '1 ronda' : `${player.rounds} rondas`}</p>
    </li>
  );
}
