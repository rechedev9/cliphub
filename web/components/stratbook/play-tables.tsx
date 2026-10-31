import type { ReactNode } from 'react';
import type { PlayGrenade, PlayPosition } from '@/lib/api/stratbook';
import { GRENADE_LABEL, playSecondsLabel, shareLabel } from '@/lib/stratbook/plays';
import { placeLabel } from '@/lib/tactical-labels';

const CELL = 'px-2 py-1.5 text-left font-normal';

function coords(pos: readonly [number, number, number]): string {
  return `${Math.round(pos[0])}, ${Math.round(pos[1])}`;
}

/** A play's grenades in landing order: kind, second, callout, thrower and catalog lineup. */
export function PlayGrenadeTable({ grenades, showShare = false }: { grenades: readonly PlayGrenade[]; showShare?: boolean }): ReactNode {
  if (grenades.length === 0) return <p className="text-body-sm text-fg-2">Sin granadas antes de la plantada.</p>;
  return (
    <div className="overflow-x-auto rounded-md border border-border">
      <table className="w-full min-w-[32rem] text-body-sm tabular-nums">
        <caption className="sr-only">Granadas de la jugada</caption>
        <thead className="bg-surface-1 text-meta text-fg-3">
          <tr>
            <th scope="col" className={CELL}>Granada</th>
            <th scope="col" className={CELL}>Cae a</th>
            <th scope="col" className={CELL}>Zona</th>
            <th scope="col" className={CELL}>Jugador</th>
            {showShare ? <th scope="col" className={CELL}>Rondas</th> : null}
            <th scope="col" className={CELL}>Lineup</th>
          </tr>
        </thead>
        <tbody>
          {grenades.map((grenade, index) => (
            <tr key={`${grenade.kind}:${index}`} className="border-t border-border-subtle">
              <td className={CELL}>{GRENADE_LABEL[grenade.kind]}</td>
              <td className={CELL}>{playSecondsLabel(grenade.seconds)}</td>
              <td className={CELL}>{placeLabel(grenade.callout ?? '') || <span className="text-fg-3">{coords(grenade.pos)}</span>}</td>
              <td className={CELL}>{grenade.player || <span className="text-fg-3">Sin dato</span>}</td>
              {showShare ? <td className={CELL}>{shareLabel(grenade.share)}</td> : null}
              <td className={CELL}>
                {grenade.lineup ? (
                  <span title={grenade.lineup_id}>{grenade.lineup}</span>
                ) : (
                  <span className="text-fg-3">{grenade.kind === 'smoke' ? 'Fuera del catálogo' : 'Solo humos'}</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Where the play's players stood at its second. */
export function PlayPositionTable({ positions, showShare = false }: { positions: readonly PlayPosition[]; showShare?: boolean }): ReactNode {
  if (positions.length === 0) return <p className="text-body-sm text-fg-2">Sin posiciones: la partida no tiene el fichero de posiciones de esa ronda.</p>;
  return (
    <div className="overflow-x-auto rounded-md border border-border">
      <table className="w-full min-w-[24rem] text-body-sm tabular-nums">
        <caption className="sr-only">Posiciones de la jugada</caption>
        <thead className="bg-surface-1 text-meta text-fg-3">
          <tr>
            <th scope="col" className={CELL}>Jugador</th>
            <th scope="col" className={CELL}>Zona</th>
            {showShare ? <th scope="col" className={CELL}>Rondas</th> : null}
          </tr>
        </thead>
        <tbody>
          {positions.map((position, index) => (
            <tr key={`${position.steamid64 ?? position.player ?? ''}:${index}`} className="border-t border-border-subtle">
              <td className={CELL}>{position.player || <span className="text-fg-3">Sin dato</span>}</td>
              <td className={CELL}>{placeLabel(position.callout ?? '') || <span className="text-fg-3">{coords(position.pos)}</span>}</td>
              {showShare ? <td className={CELL}>{shareLabel(position.share)}</td> : null}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
