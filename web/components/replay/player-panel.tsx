'use client';

import type { ReactNode } from 'react';
import { Bomb, Eye, EyeOff } from 'lucide-react';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import { prettifyWeapon } from '@/lib/api/map';
import { utilityLine } from '@/lib/replay/player-cards';
import type { PlayerCard } from '@/lib/replay/player-cards';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

const money = new Intl.NumberFormat('es-ES', { maximumFractionDigits: 0 });

function Card({ card, hidden, onToggle }: { card: PlayerCard; hidden: boolean; onToggle: () => void }): ReactNode {
  const ct = card.side === TACTICAL_SIDES.ct;
  const facts = [
    card.kills > 0 ? `${card.kills} ${card.kills === 1 ? 'baja' : 'bajas'}` : '',
    card.lastWeapon === null ? '' : prettifyWeapon(card.lastWeapon),
    utilityLine(card.utility),
  ].filter((part) => part !== '');
  return (
    <li className={cn('flex items-start gap-2 border-l-2 py-2 pr-1 pl-2.5', ct ? 'border-l-side-ct' : 'border-l-side-t', !card.alive && 'opacity-60')}>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className={cn('min-w-0 flex-1 truncate text-body-sm font-medium text-fg-1', !card.alive && 'line-through')}>{card.name}</span>
          {card.hasBomb ? <Bomb className="size-3.5 text-destructive" aria-label="Lleva la bomba" /> : null}
          <span className="shrink-0 text-meta tabular-nums text-fg-2">${money.format(card.money)}</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-surface-0" aria-hidden>
            <div className={cn('h-full', ct ? 'bg-side-ct' : 'bg-side-t')} style={{ width: `${card.health ?? 0}%` }} />
          </div>
          <span className="w-12 text-right text-meta tabular-nums text-fg-2">
            {card.alive ? `${card.health ?? '–'} HP` : 'Muerto'}
          </span>
        </div>
        <span className="truncate text-meta text-fg-3">
          Equipo ${money.format(card.equipValue)}
          {facts.length > 0 ? ` · ${facts.join(' · ')}` : ''}
        </span>
      </div>
      <Button
        type="button"
        size="icon-sm"
        variant="ghost"
        aria-pressed={!hidden}
        aria-label={hidden ? `Mostrar a ${card.name} en el mapa` : `Ocultar a ${card.name} del mapa`}
        onClick={onToggle}
      >
        {hidden ? <EyeOff aria-hidden /> : <Eye aria-hidden />}
      </Button>
    </li>
  );
}

/** Both sides at the playhead; the eye takes a player off the map without leaving the list. */
export function PlayerPanel({
  sides,
  hiddenSlots,
  onTogglePlayer,
}: {
  sides: { ct: readonly PlayerCard[]; t: readonly PlayerCard[] };
  hiddenSlots: readonly number[];
  onTogglePlayer: (slot: number) => void;
}): ReactNode {
  const groups = [
    { side: TACTICAL_SIDES.ct, cards: sides.ct },
    { side: TACTICAL_SIDES.t, cards: sides.t },
  ];
  return (
    <div className="flex flex-col gap-3">
      {groups.map(({ side, cards }) => (
        <section key={side} aria-label={`Jugadores ${side}`} className="flex flex-col gap-1">
          <h3 className={cn('text-meta font-semibold', side === TACTICAL_SIDES.ct ? 'text-side-ct' : 'text-side-t')}>
            {side} · {cards.filter((card) => card.alive).length} vivos
          </h3>
          <ul className="flex flex-col divide-y divide-border-subtle">
            {cards.map((card) => (
              <Card key={card.slot} card={card} hidden={hiddenSlots.includes(card.slot)} onToggle={() => onTogglePlayer(card.slot)} />
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
