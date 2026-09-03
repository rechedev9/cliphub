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
    <div className="studio-panel @container/reel flex flex-col overflow-hidden">
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1.5 border-b border-border-subtle bg-surface-3 px-3.5 py-2.5">
        <span className="font-mono text-meta uppercase tracking-ultra text-fg-3">{title}</span>
        <div className="flex items-center gap-3">
          <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
            {counter ?? `${selectedIds.size} ${selectedIds.size === 1 ? 'elegido' : 'elegidos'}`}
          </span>
          <div className="flex items-center gap-1">
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
      </div>

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
  );
}
