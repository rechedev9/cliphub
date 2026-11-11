'use client';

import type { ReactNode } from 'react';
import { LIBRARY_ROLES } from '@/lib/api/library';
import type { ReportSide } from '@/lib/api/scout';
import type { SideStatsPlayer } from '@/lib/api/side-stats';
import { mapRoleChips } from '@/lib/analysis/map-role';
import { RATING_FORMULA_TEXT } from '@/lib/analysis/rating';
import { formatDecimal } from '@/lib/format';
import { ROLE_LABEL } from '@/lib/library/partidas';
import { InfoTip } from '@/components/analysis/info-tip';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { RoleChip } from '@/components/ui/role-chip';

function roleText(role: string): string {
  const known = LIBRARY_ROLES.find((entry) => entry === role);
  return known === undefined ? ROLE_LABEL.unknown : ROLE_LABEL[known];
}

/** The map role of the side in scope, as EDGE's "T Mid"; without one, the library role. */
function RoleChips({ player, side }: { player: SideStatsPlayer; side: ReportSide }): ReactNode {
  const chips = mapRoleChips(player.map_roles, side);
  if (chips.length > 0) {
    return (
      <p className="flex flex-wrap items-center gap-1">
        {chips.map((chip) => (
          <RoleChip key={chip.side} side={chip.side} label={chip.label} />
        ))}
      </p>
    );
  }
  if (player.role === 'unknown') return null;
  return <RoleChip label={roleText(player.role)} />;
}

/**
 * A player's card in EDGE's Players row: the nick over the picture (FACEIT's avatar when known, else
 * the initial), then the role, the big rating and the 2x2 of K/D, ADR, KPR and DPR.
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
    <li
      title={player.rounds === 1 ? '1 ronda' : `${player.rounds} rondas`}
      className="relative isolate flex min-h-52 min-w-0 flex-col justify-between gap-3 overflow-hidden border border-border bg-surface-2 p-3"
    >
      <div aria-hidden className="absolute top-6 -right-3 -z-10 opacity-45">
        <PlayerAvatar nickname={name} playerID={player.steamid64} size={112} />
      </div>
      <div aria-hidden className="absolute inset-0 -z-10 bg-linear-to-t from-surface-2 via-surface-2/80 to-transparent" />
      <p className="truncate text-body font-bold text-fg-1">{name}</p>
      <div className="flex flex-col gap-2">
        <RoleChips player={player} side={side} />
        <p className="flex items-center gap-1.5">
          <span className="font-num text-stat leading-none font-bold text-fg-1 tabular-nums">{formatDecimal(player.rating, 2)}</span>
          <span className="text-[0.5625rem] leading-none font-bold tracking-wider text-fg-3 uppercase">
            Rating
            <br />
            ClipHub
          </span>
          <InfoTip label="Rating de ClipHub" text={RATING_FORMULA_TEXT} />
        </p>
        <dl className="grid grid-cols-2 gap-x-3 gap-y-1.5">
          {lines.map((line) => (
            <div key={line.label} className="flex flex-col">
              <dt className="text-meta font-medium tracking-wider text-fg-3 uppercase">{line.label}</dt>
              <dd className="font-num text-body font-bold text-fg-1 tabular-nums">{line.value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </li>
  );
}
