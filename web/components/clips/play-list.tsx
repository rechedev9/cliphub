'use client';

import type { ReactNode } from 'react';
import type { Play } from '@/lib/api/types';
import { Button } from '@/components/ui/button';
import { PlayRow } from './play-row';

export type PlayListProps = {
  /** Highlights in plan order; rows render in this order top to bottom. */
  plays: Play[];
  selectedIds: ReadonlySet<string>;
  /** Panel header eyebrow. */
  title?: string;
  /** Right side of the header; defaults to the selected count. */
  counter?: ReactNode;
  toolbar?: ReactNode;
  onToggle: (id: string) => void;
  onSelectAll: () => void;
  onClear: () => void;
};

/**
 * One bordered row per highlight (PlayRow) under a mono header with the
 * selection counter plus Seleccionar todo / Limpiar; no horizontal scroll.
 * The list owns `@container/reel` because PlayRow keys its breakpoints to the
 * list's width, not the viewport's.
 */
export function PlayList({
  plays,
  selectedIds,
  title = 'Highlights',
  counter,
  toolbar,
  onToggle,
  onSelectAll,
  onClear,
}: PlayListProps): ReactNode {
  const allSelected = plays.length > 0 && selectedIds.size === plays.length;
  // Short order is plan order filtered by membership — the same rule the page
  // uses to build the render payload — so the badge always matches the output.
  const positions = new Map<string, number>();
  for (const play of plays) {
    if (selectedIds.has(play.id)) positions.set(play.id, positions.size + 1);
  }

  return (
    <div className="studio-panel @container/reel flex max-h-[min(28rem,55dvh)] min-h-0 flex-col overflow-hidden @[56rem]/content:max-h-[clamp(20rem,calc(100dvh-20rem),40rem)]">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-3 border-b border-border-subtle bg-surface-3 p-4 @[80rem]/content:p-5">
        <span className="text-body-sm font-semibold text-fg-2">{title}</span>
        <div className="flex flex-wrap items-center gap-3">
          <span className="text-body-sm text-fg-2">
            {counter ?? `${selectedIds.size} ${selectedIds.size === 1 ? 'elegido' : 'elegidos'}`}
          </span>
        </div>
        <div className="flex w-full flex-wrap items-center gap-2">
          {toolbar}
          <Button
            type="button"
            variant="ghost"
            size="xs"
            disabled={allSelected}
            onClick={onSelectAll}
            className="font-mono tracking-wider uppercase"
          >
            Seleccionar todo
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="xs"
            disabled={selectedIds.size === 0}
            onClick={onClear}
            className="font-mono tracking-wider uppercase"
          >
            Limpiar
          </Button>
        </div>
      </div>

      <div tabIndex={0} role="region" aria-label="Jugadas disponibles" className="min-h-0 overflow-y-auto overscroll-y-contain focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring">
        {plays.map((play) => (
          <PlayRow
            key={play.id}
            play={play}
            selected={selectedIds.has(play.id)}
            reelPosition={positions.get(play.id) ?? null}
            onToggle={() => onToggle(play.id)}
          />
        ))}
      </div>
    </div>
  );
}
