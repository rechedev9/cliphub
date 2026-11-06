import type { ReactNode } from 'react';
import Link from 'next/link';
import { LIBRARY_ROLES } from '@/lib/api/library';
import type { PrepPlayer, TeamPlaybook, TeamPrep } from '@/lib/api/playbook';
import type { TacticalSide } from '@/lib/api/tactical';
import { buyIcon } from '@/lib/analysis/icons';
import { evidenceRefs } from '@/lib/analysis/playbook';
import { matchupGrid, matchupKey, prepWindowLabel, secondsLabel, tempoRows } from '@/lib/analysis/prep';
import { replayHref } from '@/lib/analysis/replay-link';
import { prettyMapName } from '@/lib/format';
import { ROLE_LABEL } from '@/lib/library/partidas';
import { buyLabel, placeLabel } from '@/lib/tactical-labels';
import { HeatTable, type HeatCell, type HeatColumn, type HeatRow } from '@/components/ui/heat-table';
import { BarList } from '@/components/playbook/bars';
import { CategoryGrid, SiteBars, TempoChart } from '@/components/playbook/playbook-summary';
import { CtPositionsMap, playerColor } from './ct-positions-map';
import type { RatePool } from '@/lib/analysis/library-pool';

const SIDES: readonly TacticalSide[] = ['CT', 'T'];

export function PrepSection({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <section aria-label={title} className="flex flex-col gap-2 print:break-inside-avoid">
      <h2 className="text-body font-semibold text-fg-1">{title}</h2>
      {children}
    </section>
  );
}

function replayTitle(prep: Pick<TeamPrep, 'team_name' | 'map'>, what: string): string {
  return `${prep.team_name} · ${what} · ${prettyMapName(prep.map)}`;
}

/** Each side's win rate per buy matchup: the team's buy per row, the rival's per column. */
export function MatchupTables({ prep, library }: { prep: TeamPrep; library: RatePool | null }): ReactNode {
  return (
    <div className="grid gap-2 @[96rem]/content:grid-cols-2 print:grid-cols-2">
      {SIDES.map((side) => {
        const grid = matchupGrid(prep.matchups, side);
        const columns: HeatColumn[] = [
          { id: 'buy', header: `Su compra en ${side}` },
          ...grid.opponentBuys.map((buy) => ({
            id: buy,
            header: `contra ${buyLabel(buy).toLowerCase()}`,
            align: 'end' as const,
            heat: grid.baselinePct === null ? undefined : { baseline: grid.baselinePct, spread: 20 },
          })),
        ];
        const rows: HeatRow[] = grid.buys.map((buy) => {
          const cells: Record<string, HeatCell> = { buy: { text: buyLabel(buy), tone: side === 'CT' ? 'ct' : 't', icon: buyIcon(buy) } };
          for (const opponentBuy of grid.opponentBuys) {
            const cell = grid.cells.get(matchupKey(buy, opponentBuy));
            if (cell === undefined || cell.rounds === 0) continue;
            const reference = library ? library.pct(`${side}|${buy}|${opponentBuy}`, 6) : undefined;
            cells[opponentBuy] = {
              text: `${Math.round(cell.win_rate.pct)} %`,
              value: cell.win_rate.pct,
              n: cell.rounds,
              reference,
              referenceText: reference === null || reference === undefined ? undefined : `Biblioteca ${Math.round(reference)} %`,
              href:
                cell.refs.length > 0
                  ? replayHref(evidenceRefs(cell.refs), replayTitle(prep, `${buyLabel(buy)} contra ${buyLabel(opponentBuy).toLowerCase()} en ${side}`))
                  : undefined,
            };
          }
          return { id: buy, cells };
        });
        return (
          <HeatTable
            key={side}
            label={`Rondas ganadas en ${side}`}
            columns={columns}
            rows={rows}
            minSample={4}
            sampleNoun="rondas"
            empty={`Sin rondas de ${side} con compra conocida.`}
            reference={library ? 'la biblioteca en el mismo cruce de compras' : 'la media de la tabla'}
          />
        );
      })}
    </div>
  );
}

/** Median tempo per buy on each side; T rows also give the plant. */
export function TempoByBuy({ prep }: { prep: TeamPrep }): ReactNode {
  return (
    <div className="grid gap-2 lg:grid-cols-2 print:grid-cols-2">
      {SIDES.map((side) => {
        const rows = tempoRows(prep.tempo, side);
        return (
          <div key={side} className="studio-panel flex flex-col gap-1.5 px-3 py-2.5">
            <h3 className="text-body-sm font-semibold text-fg-1">
              {side === 'T' ? 'En T, segundos hasta pisar un sitio' : 'En CT, segundos hasta el primer duelo'}
            </h3>
            {rows.length === 0 ? (
              <p className="text-body-sm text-fg-2">Sin rondas con ritmo medido.</p>
            ) : (
              <BarList
                label={`Ritmo por compra en ${side}`}
                items={rows.map((row) => ({
                  id: row.buy,
                  label: buyLabel(row.buy),
                  icon: buyIcon(row.buy),
                  value: row.tempo.median,
                  text: secondsLabel(row.tempo.median),
                  detail:
                    side === 'T' && row.plant.samples > 0
                      ? `${row.tempo.samples} rondas · planta ${secondsLabel(row.plant.median)}`
                      : `${row.tempo.samples} rondas`,
                  href: row.refs.length > 0 ? replayHref(evidenceRefs(row.refs), replayTitle(prep, `${buyLabel(row.buy)} en ${side}`)) : undefined,
                }))}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

function roleText(role: string): string {
  const known = LIBRARY_ROLES.find((entry) => entry === role);
  return known === undefined || known === 'unknown' ? '' : ROLE_LABEL[known];
}

function PlayerPositions({ prep, player, index }: { prep: TeamPrep; player: PrepPlayer; index: number }): ReactNode {
  const role = roleText(player.role);
  return (
    <article aria-label={player.name} className="studio-panel flex flex-col gap-2 px-3 py-2.5 print:break-inside-avoid">
      <header className="flex flex-wrap items-baseline gap-x-2">
        <h3 className="flex items-center gap-1.5 text-body font-semibold text-fg-1">
          <span aria-hidden className="size-2 rounded-full" style={{ backgroundColor: playerColor(index) }} />
          {player.name}
        </h3>
        <p className="text-meta text-fg-3">
          {role === '' ? '' : `${role} · `}
          {player.rounds} rondas de CT
        </p>
      </header>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4 print:grid-cols-4">
        {player.windows.map((window) => (
          <div key={window.from_seconds} className="flex min-w-0 flex-col gap-1">
            <h4 className="text-meta text-fg-3">{prepWindowLabel(window)}</h4>
            {window.spots.length === 0 ? (
              <p className="text-body-sm text-fg-3">Sin datos</p>
            ) : (
              <ol className="flex flex-col gap-0.5">
                {window.spots.map((spot) => (
                  <li key={spot.callout} className="flex items-baseline justify-between gap-2 text-body-sm">
                    {spot.refs.length > 0 ? (
                      <Link
                        href={replayHref(evidenceRefs(spot.refs), replayTitle(prep, `${player.name} en ${placeLabel(spot.callout)}`))}
                        className="truncate text-fg-1 hover:text-primary hover:underline"
                      >
                        {placeLabel(spot.callout)}
                      </Link>
                    ) : (
                      <span className="truncate text-fg-1">{placeLabel(spot.callout)}</span>
                    )}
                    <span className="shrink-0 text-fg-2 tabular-nums">{Math.round(spot.share.pct)} %</span>
                  </li>
                ))}
              </ol>
            )}
          </div>
        ))}
      </div>
    </article>
  );
}

/** Where each CT player stands in each window of the round. */
export function CtPositions({ prep }: { prep: TeamPrep }): ReactNode {
  if (prep.ct_players.length === 0) {
    return (
      <p className="text-body-sm text-fg-2">
        {prep.framed === 0 ? 'Estas partidas no guardan posiciones de los jugadores.' : 'Ningún jugador tiene rondas de CT con posiciones.'}
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-3">
      <p className="text-meta text-fg-3">
        Por cada tramo, el sitio donde más estuvo en cada ronda y en qué parte de sus rondas. Leído en {prep.framed} rondas de CT con posiciones.
      </p>
      <CtPositionsMap prep={prep} />
      <div className="grid gap-2 @[90rem]/content:grid-cols-2 print:grid-cols-1">
        {prep.ct_players.map((player, index) => (
          <PlayerPositions key={player.steamid64} prep={prep} player={player} index={index} />
        ))}
      </div>
    </div>
  );
}

/** One side's playbook in brief: round types, sites and tempo. */
export function PlaybookBrief({ playbook, library }: { playbook: TeamPlaybook; library: RatePool | null }): ReactNode {
  if (playbook.round_count === 0) return <p className="text-body-sm text-fg-2">Sin rondas de {playbook.side} con este filtro.</p>;
  return (
    <div className="flex flex-col gap-3">
      <p className="text-body-sm text-fg-2">
        Gana el {Math.round(playbook.win_rate.pct)} % de sus {playbook.round_count} rondas de {playbook.side}.{library ? ' En cada tipo, ▲▼ frente a la biblioteca en el mismo mapa y lado: cuánto lo usan y cuánto ganan.' : ''}
      </p>
      <CategoryGrid playbook={playbook} compact reference={library} />
      <div className="grid gap-2 lg:grid-cols-2 print:grid-cols-2">
        <div className="studio-panel flex flex-col gap-1.5 px-3 py-2.5">
          <h3 className="text-body-sm font-semibold text-fg-1">Sitios</h3>
          <SiteBars playbook={playbook} />
        </div>
        <div className="studio-panel flex flex-col gap-1.5 px-3 py-2.5">
          <h3 className="text-body-sm font-semibold text-fg-1">Ritmo</h3>
          <TempoChart playbook={playbook} rounds={playbook.rounds} />
        </div>
      </div>
    </div>
  );
}
