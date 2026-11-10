'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { TriangleAlert } from 'lucide-react';
import { ATTACK_SITES, ATTACK_TEMPOS } from '@/lib/api/attack';
import type { AttackCell, AttackPlayer, TeamAttack } from '@/lib/api/attack';
import { getTeamAttack } from '@/lib/api/library';
import { BAND_LABEL, DEFAULT_TRAMO, attackHref, lastMatchText, siteLabel, siteZones, tempoLabel, warningText, windowTramo } from '@/lib/analysis/attack';
import { prepWindowLabel, secondsLabel } from '@/lib/analysis/prep';
import { prettyMapName } from '@/lib/format';
import { placeLabel } from '@/lib/tactical-labels';
import { Button } from '@/components/ui/button';
import { HeatTable, type HeatCell, type HeatColumn, type HeatRow } from '@/components/ui/heat-table';
import { Skeleton } from '@/components/ui/skeleton';
import { PrepSection } from './prep-sections';
import type { PrepQuery } from './prep-body';

const pct = (value: number): string => `${Math.round(value)} %`;

function cellOf({ attack, cell, what, zones }: { attack: TeamAttack; cell: AttackCell; what: string; zones: string[] }): HeatCell {
  return {
    text: `${pct(cell.share.pct)} · gana ${pct(cell.win_rate.pct)}`,
    n: cell.rounds,
    href: attackHref(attack, { refs: cell.refs, what, zones }),
  };
}

/** Sites or Bombsite Tempo per buy matchup: how often each one comes and how it ends. */
function CrossTable({ attack, kind }: { attack: TeamAttack; kind: 'sites' | 'tempo' }): ReactNode {
  const keys: readonly string[] = kind === 'sites' ? ATTACK_SITES : ATTACK_TEMPOS;
  const label = kind === 'sites' ? siteLabel : tempoLabel;
  const zones = (key: string): string[] => (kind === 'sites' ? siteZones(key) : []);
  const columns: HeatColumn[] = [{ id: 'buy', header: 'Compras' }, ...keys.map((key) => ({ id: key, header: label(key), align: 'end' as const }))];
  const total: Record<string, HeatCell> = { buy: { text: 'Todas las rondas', tone: 't' } };
  for (const cell of kind === 'sites' ? attack.sites : attack.tempo) {
    if (cell.rounds > 0) total[cell.key] = cellOf({ attack, cell, what: label(cell.key), zones: zones(cell.key) });
  }
  const rows: HeatRow[] = [{ id: 'all', cells: total }];
  for (const matchup of attack.matchups) {
    const matchupText = `${BAND_LABEL[matchup.buy]} contra ${BAND_LABEL[matchup.opponent_buy].toLowerCase()}`;
    const cells: Record<string, HeatCell> = { buy: { text: matchupText, tone: 't' } };
    for (const cell of kind === 'sites' ? matchup.sites : matchup.tempo) {
      if (cell.rounds > 0) cells[cell.key] = cellOf({ attack, cell, what: `${matchupText} · ${label(cell.key)}`, zones: zones(cell.key) });
    }
    rows.push({ id: `${matchup.buy}|${matchup.opponent_buy}`, cells });
  }
  return (
    <HeatTable
      label={kind === 'sites' ? 'Sitio por compras' : 'Ritmo por compras'}
      columns={columns}
      rows={rows}
      minSample={4}
      sampleNoun="rondas"
      empty="Sin rondas de T con compra conocida."
    />
  );
}

function Defaults({ attack }: { attack: TeamAttack }): ReactNode {
  if (attack.defaults.length === 0) return <p className="text-body-sm text-fg-2">Sin defaults con posiciones en estas rondas.</p>;
  return (
    <ul className="grid gap-2 lg:grid-cols-2 print:grid-cols-2">
      {attack.defaults.map((entry) => {
        const zones = entry.zones.map((zone) => zone.zone);
        const href = attackHref(attack, { refs: entry.refs, what: `default ${entry.formation}`, zones, tramo: DEFAULT_TRAMO });
        return (
          <li key={entry.formation} className="studio-panel flex flex-col gap-1 px-3 py-2.5">
            <div className="flex items-baseline justify-between gap-2">
              {href === undefined ? (
                <span className="text-body-sm font-semibold text-fg-1">A, medio y B: {entry.formation}</span>
              ) : (
                <Link href={href} className="text-body-sm font-semibold text-fg-1 hover:text-primary hover:underline">
                  A, medio y B: {entry.formation}
                </Link>
              )}
              <span className="text-meta text-fg-2 tabular-nums">
                {pct(entry.share.pct)} de las rondas · gana {pct(entry.win_rate.pct)}
              </span>
            </div>
            <p className="text-meta text-fg-3">{entry.zones.map((zone) => zone.label || placeLabel(zone.zone)).join(', ') || 'Sin zona fija'}</p>
          </li>
        );
      })}
    </ul>
  );
}

function Executes({ attack }: { attack: TeamAttack }): ReactNode {
  if (attack.executes.length === 0) return <p className="text-body-sm text-fg-2">Sin ejecuciones a un sitio en estas rondas.</p>;
  return (
    <ol className="flex flex-col gap-1">
      {attack.executes.map((entry) => {
        const route = entry.route === '' ? '' : ` por ${entry.route_label || placeLabel(entry.route)}`;
        const what = `${BAND_LABEL[entry.buy]} contra ${BAND_LABEL[entry.opponent_buy].toLowerCase()} · ${siteLabel(entry.site)}${route}`;
        const zones = [...(entry.route === '' ? [] : [entry.route]), ...siteZones(entry.site)];
        const href = attackHref(attack, { refs: entry.refs, what, zones });
        return (
          <li key={`${entry.buy}|${entry.opponent_buy}|${entry.site}|${entry.route}`} className="flex flex-wrap items-baseline justify-between gap-x-3 text-body-sm">
            {href === undefined ? <span className="text-fg-1">{what}</span> : <Link href={href} className="text-fg-1 hover:text-primary hover:underline">{what}</Link>}
            <span className="text-fg-2 tabular-nums">
              {pct(entry.share.pct)} · gana {pct(entry.win_rate.pct)} · rush {pct(entry.rushes.pct)} · planta {pct(entry.plants.pct)}{entry.contact_seconds === null ? '' : ` · llega en ${secondsLabel(entry.contact_seconds)}`}
            </span>
          </li>
        );
      })}
    </ol>
  );
}

function PlayerColumn({ attack, player }: { attack: TeamAttack; player: AttackPlayer }): ReactNode {
  return (
    <article aria-label={player.name} className="studio-panel flex flex-col gap-2 px-3 py-2.5 print:break-inside-avoid">
      <header className="flex flex-wrap items-baseline gap-x-2">
        <h3 className="text-body font-semibold text-fg-1">{player.name}</h3>
        <p className="text-meta text-fg-3">{player.rounds} rondas de T</p>
      </header>
      {player.windows.map((window) => (
        <div key={window.from_seconds} className="flex flex-col gap-0.5">
          <h4 className="text-meta text-fg-3">
            {prepWindowLabel(window)} · quieto {pct(window.pace.static.pct)}, lento {pct(window.pace.slow.pct)}, rápido {pct(window.pace.fast.pct)}
          </h4>
          {window.spots.length === 0 ? (
            <p className="text-body-sm text-fg-3">Sin datos</p>
          ) : (
            <ol className="flex flex-col gap-0.5">
              {window.spots.map((spot) => {
                const href = attackHref(attack, {
                  refs: spot.refs,
                  what: `${player.name} en ${placeLabel(spot.callout)}`,
                  zones: [spot.callout],
                  focus: [player.steamid64],
                  tramo: windowTramo(window),
                });
                return (
                  <li key={spot.callout} className="flex items-baseline justify-between gap-2 text-body-sm">
                    {href === undefined ? (
                      <span className="truncate text-fg-1">{placeLabel(spot.callout)}</span>
                    ) : (
                      <Link href={href} className="truncate text-fg-1 hover:text-primary hover:underline">
                        {placeLabel(spot.callout)}
                      </Link>
                    )}
                    <span className="shrink-0 text-fg-2 tabular-nums">
                      {pct(spot.share.pct)} · llega en {secondsLabel(spot.arrival_seconds)}
                    </span>
                  </li>
                );
              })}
            </ol>
          )}
        </div>
      ))}
    </article>
  );
}

/** The T side match prep of a rival on one map; every figure opens its rounds in the Replay with the zones marked. */
export function AttackPrep({ query }: { query: PrepQuery & { matchIds?: readonly string[] } }): ReactNode {
  const { teamId, map, from, to } = query;
  // A saved report reads only its own matches; the keys keep the effect from refiring on a new array.
  const matchKey = (query.matchIds ?? []).join(',');
  const lineupKey = (query.lineup ?? []).join(',');
  const [attack, setAttack] = useState<TeamAttack | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setAttack(null);
    setFailed(false);
    const list = (key: string): string[] | undefined => (key === '' ? undefined : key.split(','));
    getTeamAttack(teamId, { map, from, to, match_ids: list(matchKey), lineup: list(lineupKey) })
      .then((loaded) => {
        if (active) setAttack(loaded);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, from, to, matchKey, lineupKey, attempt]);

  if (failed) {
    return (
      <PrepSection title="Ataque en T">
        <div className="flex flex-col items-start gap-2">
          <p role="alert" className="text-body-sm text-destructive">No se pudo leer el ataque de este rival.</p>
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        </div>
      </PrepSection>
    );
  }
  if (attack === null) return <Skeleton role="status" aria-label="Leyendo el ataque" className="h-56 w-full" />;
  if (attack.round_count === 0) return null;
  return (
    <>
      <header className="flex flex-wrap items-baseline gap-x-3 gap-y-1 border-t border-border pt-4">
        <h2 className="text-body font-semibold text-fg-1">Ataque en T</h2>
        <p className="text-meta text-fg-2">
          {lastMatchText(attack.days_since_last_match)} · {attack.matches} {attack.matches === 1 ? 'partida' : 'partidas'} · {attack.round_count} rondas de T
        </p>
      </header>
      {attack.warnings.length === 0 ? null : (
        <ul role="note" aria-label="Avisos del ataque" className="flex flex-col gap-1 rounded-md border border-border-strong bg-surface-2 px-3 py-2">
          {attack.warnings.map((warning) => (
            <li key={warning.kind} className="flex items-start gap-2 text-body-sm text-fg-1">
              <TriangleAlert className="mt-0.5 size-4 shrink-0 text-fg-2" aria-hidden />
              {warningText(warning)}
            </li>
          ))}
        </ul>
      )}
      <PrepSection title="Sitio por compras en T" hint="Qué parte de las rondas va a cada sitio y cuántas gana. Cada cifra abre sus rondas con la zona marcada.">
        <CrossTable attack={attack} kind="sites" />
      </PrepSection>
      <PrepSection title="Ritmo del sitio por compras" hint={`Rush, estrategia preparada, media ronda o sin pisar sitio, en ${prettyMapName(attack.map)}.`}>
        <CrossTable attack={attack} kind="tempo" />
      </PrepSection>
      <PrepSection title="Defaults preferidos" hint="Dónde están a los 25 s en las rondas lentas.">
        <Defaults attack={attack} />
      </PrepSection>
      <PrepSection title="Ejecuciones por compra">
        <Executes attack={attack} />
      </PrepSection>
      <PrepSection title="Jugadores en T" hint="Por tramo, dónde está cada uno, cuándo llega y a qué ritmo se mueve.">
        {attack.players.length === 0 ? (
          <p className="text-body-sm text-fg-2">Estas partidas no guardan posiciones de los jugadores.</p>
        ) : (
          <div className="grid gap-2 @[90rem]/content:grid-cols-2 print:grid-cols-1">
            {attack.players.map((player) => (
              <PlayerColumn key={player.steamid64} attack={attack} player={player} />
            ))}
          </div>
        )}
      </PrepSection>
    </>
  );
}
