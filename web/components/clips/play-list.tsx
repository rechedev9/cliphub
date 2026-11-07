'use client';

import type { ReactNode } from 'react';
import type { Play } from '@/lib/api/types';
import { Button } from '@/components/ui/button';
import type { FormatTone } from '@/components/studio/format-tone';
import { roundOrdinals } from '@/lib/produce/round-ordinals';
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
  /** Filter controls under the toolbar; with them, only `visibleIds` render as rows. */
  filters?: ReactNode;
  visibleIds?: ReadonlySet<string>;
  /** Shown in place of the rows when the filters hide every play. */
  emptyFiltered?: ReactNode;
  /** Multi-POV only: play id → the player whose chosen play it overlaps. */
  clashes?: ReadonlyMap<string, string>;
  /** The format colour of the flow, for the selected cards. */
  tone?: FormatTone;
  onToggle: (id: string) => void;
  onSelectAll: () => void;
  onClear: () => void;
};

/**
 * One selectable card per highlight (PlayRow) under a heading with the
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
  filters,
  visibleIds,
  emptyFiltered,
  clashes,
  tone = 'short',
  onToggle,
  onSelectAll,
  onClear,
}: PlayListProps): ReactNode {
  const rows = visibleIds ? plays.filter((play) => visibleIds.has(play.id)) : plays;
  const allSelected = rows.length > 0 && rows.every((play) => selectedIds.has(play.id));
  // Short order is plan order filtered by membership — the same rule the page
  // uses to build the render payload — so the badge always matches the output.
  const ordinals = roundOrdinals(plays);
  const positions = new Map<string, number>();
  for (const play of plays) {
    if (selectedIds.has(play.id)) positions.set(play.id, positions.size + 1);
  }

  return (
    <section aria-label={title} className="@container/reel flex min-w-0 flex-col gap-2.5">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <h2 className="flex-1 font-display text-title font-extrabold text-fg-1">
          {title}{' '}
          <span className="text-body-sm font-medium text-fg-3">
            {counter ?? `${selectedIds.size} ${selectedIds.size === 1 ? 'elegido' : 'elegidos'}`}
          </span>
        </h2>
        <div className="flex flex-wrap items-center gap-2">
          {toolbar}
          <Button
            type="button"
            variant="ghost"
            size="xs"
            disabled={allSelected || rows.length === 0}
            onClick={onSelectAll}
          >
            Seleccionar todo
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="xs"
            disabled={selectedIds.size === 0}
            onClick={onClear}
          >
            Limpiar
          </Button>
        </div>
      </div>
      {filters}

      <div role="region" aria-label="Jugadas disponibles" className="flex flex-col gap-1.5">
        {rows.length === 0 && plays.length > 0 ? emptyFiltered : null}
        {rows.map((play) => (
          <PlayRow
            key={play.id}
            play={play}
            selected={selectedIds.has(play.id)}
            reelPosition={positions.get(play.id) ?? null}
            ordinal={ordinals.get(play.id)}
            clashWith={clashes?.get(play.id)}
            tone={tone}
            onToggle={() => onToggle(play.id)}
          />
        ))}
      </div>
    </section>
  );
}
