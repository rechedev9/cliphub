'use client';

import type { ReactNode } from 'react';
import { LIBRARY_ROLES } from '@/lib/api/library';
import type { ReportSide } from '@/lib/api/scout';
import type { SideStatsPlayer } from '@/lib/api/side-stats';
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
  const { t, ct } = player.map_roles ?? {};
  const tLabel = side !== 'CT' && t !== undefined ? t.label : '';
  const ctLabel = side !== 'T' && ct !== undefined ? ct.label : '';
  if (tLabel === '' && ctLabel === '') {
    return (
      <p className="flex flex-wrap items-center gap-1 text-meta text-fg-2">
        <SideLabel side={side} />
        <RoleChip label={roleText(player.role)} />
      </p>
    );
  }
  return (
    <p className="flex flex-wrap items-center gap-1">
      {tLabel === '' ? null : <RoleChip side="t" label={tLabel} />}
      {ctLabel === '' ? null : <RoleChip side="ct" label={ctLabel} />}
      {player.role === 'unknown' ? null : <RoleChip label={roleText(player.role)} />}
    </p>
  );
}

/** A player's card in Estadísticas: role chips, rating and the per-round figures. */
export function PlayerStatCard({ player, side }: { player: SideStatsPlayer; side: ReportSide }): ReactNode {
  const name = player.name || player.steamid64;
  const lines = [
    { label: 'K/D', value: formatDecimal(player.kd, 2) },
    { label: 'ADR', value: formatDecimal(player.adr, 1) },
    { label: 'KPR', value: formatDecimal(player.kpr, 2) },
    { label: 'DPR', value: formatDecimal(player.dpr, 2) },
  ];
  return (
    <li className="studio-panel flex min-w-0 flex-col gap-2 p-3">
      <div className="flex min-w-0 items-center gap-2">
        <PlayerAvatar nickname={name} playerID={player.steamid64} size={32} />
        <div className="flex min-w-0 flex-col gap-0.5">
          <p className="truncate text-body font-semibold text-fg-1">{name}</p>
          <RoleChips player={player} side={side} />
        </div>
      </div>
      <p className="flex items-baseline gap-1.5">
        <span className="text-title font-semibold text-fg-1 tabular-nums">{formatDecimal(player.rating, 2)}</span>
        <span className="text-meta text-fg-3">{RATING_LABEL}</span>
        <InfoTip label={RATING_LABEL} text={RATING_FORMULA_TEXT} />
      </p>
      <dl className="grid grid-cols-4 gap-2">
        {lines.map((line) => (
          <div key={line.label} className="flex flex-col">
            <dt className="text-meta text-fg-3">{line.label}</dt>
            <dd className="text-body-sm font-medium text-fg-1 tabular-nums">{line.value}</dd>
          </div>
        ))}
      </dl>
      <p className="text-meta text-fg-3 tabular-nums">{player.rounds === 1 ? '1 ronda' : `${player.rounds} rondas`}</p>
    </li>
  );
}
