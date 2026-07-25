'use client';

import type { Play } from '@/lib/api/types';
import { Button } from '@/components/ui/button';
import { PlayRow } from './play-row';

export type PlayListProps = {
  /** Highlights in plan order; rows render in this order top to bottom. */
  plays: Play[];
  selectedIds: ReadonlySet<string>;
  onToggle: (id: string) => void;
  onSelectAll: () => void;
  onClear: () => void;
};

/**
 * PlayList — the vertical, scroll-with-the-page successor to the horizontal
 * Filmstrip of PlayTiles. One bordered row per highlight (PlayRow), a compact
 * mono header with the selection count plus Seleccionar todo / Limpiar, and no
 * horizontal scroll at any width.
 *
 * The list owns `@container/reel` because PlayRow keys its own breakpoints to
 * the list's width, not the viewport's: the same row renders inside the wide
 * one-column layout and inside the narrow two-pane one.
 */
export function PlayList({ plays, selectedIds, onToggle, onSelectAll, onClear }: PlayListProps) {
  const allSelected = plays.length > 0 && selectedIds.size === plays.length;
  // Reel order is plan order filtered by membership — the same rule the page
  // uses to build the render payload — so the badge always matches the output.
  const reelPositions = new Map<string, number>();
  for (const play of plays) {
    if (selectedIds.has(play.id)) reelPositions.set(play.id, reelPositions.size + 1);
  }

  return (
    <div className="studio-panel @container/reel flex flex-col overflow-hidden">
      <div className="flex items-center justify-between gap-3 border-b border-border-subtle bg-surface-3 px-3 py-2">
        <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
          {selectedIds.size > 0
            ? `${selectedIds.size} ${selectedIds.size === 1 ? 'SELECCIONADA' : 'SELECCIONADAS'}`
            : 'TOCA PARA SELECCIONAR'}
        </span>
        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={allSelected}
            onClick={onSelectAll}
            className="font-mono text-meta tracking-wider"
          >
            SELECCIONAR TODO
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={selectedIds.size === 0}
            onClick={onClear}
            className="font-mono text-meta tracking-wider"
          >
            LIMPIAR
          </Button>
        </div>
      </div>

      {plays.map((play) => (
        <PlayRow
          key={play.id}
          play={play}
          selected={selectedIds.has(play.id)}
          reelPosition={reelPositions.get(play.id) ?? null}
          onToggle={() => onToggle(play.id)}
        />
      ))}
    </div>
  );
}
