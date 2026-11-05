'use client';

import { useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Play, X } from 'lucide-react';
import type { TeamPlaybook } from '@/lib/api/playbook';
import type { TacticalSite } from '@/lib/api/tactical';
import {
  EMPTY_ZONE_FILTER,
  filterRoundsByZones,
  roundRefs,
  toggleZone,
  winPct,
  zoneFilterSentence,
  zoneRoundCounts,
  type ZoneFilter,
} from '@/lib/analysis/playbook';
import { replayHref } from '@/lib/analysis/replay-link';
import { siteLabel } from '@/lib/tactical-labels';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { Toggle } from '@/components/ui/toggle';
import { playbookReplayTitle } from './playbook-summary';
import { ZoneMap } from './zone-map';

const SITES: readonly { value: TacticalSite | ''; label: string }[] = [
  { value: '', label: 'Cualquier final' },
  { value: 'A', label: 'Acaba en A' },
  { value: 'B', label: 'Acaba en B' },
  { value: 'mid', label: 'Acaba en mid' },
];

/**
 * The zones the side held, as chips that combine: every chip chosen must be
 * held, and the site control narrows to where the round ended. The rounds are
 * filtered here over the playbook's per-round rows.
 */
export function ZoneExplorer({ teamId, playbook }: { teamId: string; playbook: TeamPlaybook }): ReactNode {
  const [filter, setFilter] = useState<ZoneFilter>(EMPTY_ZONE_FILTER);
  const labels = useMemo(() => new Map(playbook.zones.map((zone) => [zone.zone, zone.label])), [playbook.zones]);
  const shown = useMemo(() => filterRoundsByZones(playbook.rounds, filter), [playbook.rounds, filter]);
  const counts = useMemo(() => zoneRoundCounts(shown), [shown]);
  const sentence = zoneFilterSentence(
    filter.zones.map((zone) => labels.get(zone) ?? zone),
    filter.site === '' ? '' : siteLabel(filter.site),
  );
  const won = winPct(shown);
  const active = filter.zones.length > 0 || filter.site !== '';
  const title = playbookReplayTitle(playbook, sentence === '' ? 'Todas las rondas' : sentence);

  if (playbook.zones.length === 0) {
    return (
      <p className="text-body-sm text-fg-2">
        {playbook.framed === 0
          ? 'Estas partidas no guardan posiciones, así que no se puede leer qué zonas controlan.'
          : `En ninguna ronda sujetaron una zona con ${playbook.thresholds.map_control_players} o más durante ${playbook.thresholds.map_control_seconds} s.`}
      </p>
    );
  }

  return (
    <div className="grid gap-4 md:grid-cols-[minmax(0,20rem)_minmax(0,1fr)]">
      <ZoneMap teamId={teamId} map={playbook.map} zones={playbook.zones.map((zone) => zone.zone)} labels={labels} selected={filter.zones} />
      <div className="flex min-w-0 flex-col gap-3">
        <div role="group" aria-label="Zonas controladas" className="flex flex-wrap gap-2">
          {playbook.zones.map((zone) => {
            const on = filter.zones.includes(zone.zone);
            const n = counts.get(zone.zone) ?? 0;
            return (
              <Toggle
                key={zone.zone}
                variant="filter"
                pressed={on}
                disabled={!on && n === 0}
                onPressedChange={() => setFilter((current) => toggleZone(current, zone.zone))}
                aria-label={`${zone.label}, ${n} rondas`}
              >
                {zone.label}
                <span aria-hidden className="text-fg-3 tabular-nums">{n}</span>
              </Toggle>
            );
          })}
        </div>
        <Segmented label="Dónde acaba la ronda" value={filter.site} options={SITES} onChange={(site) => setFilter((current) => ({ ...current, site }))} />
        <div className="flex flex-wrap items-center gap-3 border-t border-border-subtle pt-3">
          <p aria-live="polite" className="text-body-sm text-fg-1">
            {sentence === '' ? 'Todas las rondas' : sentence}:{' '}
            <span className="tabular-nums">
              {shown.length} {shown.length === 1 ? 'ronda' : 'rondas'}
              {won === null ? '' : `, gana ${won} %`}
            </span>
          </p>
          <div className="ml-auto flex items-center gap-2">
            {active ? (
              <Button type="button" variant="ghost" size="xs" onClick={() => setFilter(EMPTY_ZONE_FILTER)}>
                <X aria-hidden />
                Quitar zonas
              </Button>
            ) : null}
            {shown.length > 0 ? (
              <Button asChild size="sm">
                <Link href={replayHref(roundRefs(shown), title)}>
                  <Play aria-hidden />
                  Reproducir {shown.length} {shown.length === 1 ? 'ronda' : 'rondas'}
                </Link>
              </Button>
            ) : (
              <Button type="button" size="sm" disabled>
                <Play aria-hidden />
                Reproducir 0 rondas
              </Button>
            )}
          </div>
        </div>
        {playbook.framed < playbook.round_count ? (
          <p className="text-meta text-fg-3">
            Zonas leídas en {playbook.framed} de {playbook.round_count} rondas; las demás no guardan posiciones.
          </p>
        ) : null}
      </div>
    </div>
  );
}
