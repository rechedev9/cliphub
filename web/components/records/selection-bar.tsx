'use client';

import type { ReactNode } from 'react';
import { X } from 'lucide-react';
import { selectedCount, type CountNoun } from '@/lib/records/selection';
import { DeleteChosenButton } from '@/components/records/delete-chosen-button';
import { Button } from '@/components/ui/button';

type SelectionBarProps = {
  count: number;
  noun: CountNoun;
  busy: boolean;
  /** Extra text after the count, such as the size the selection frees. */
  detail?: string;
  confirmDescription: string;
  onDelete: () => void;
  onClear: () => void;
};

/**
 * Actions for the rows picked in a page that is not one table. Render it last on the page: it floats at the
 * bottom of the window while the page scrolls, and nothing above it moves when it appears.
 */
export function SelectionBar({ count, noun, busy, detail, confirmDescription, onDelete, onClear }: SelectionBarProps): ReactNode {
  return (
    <div
      role="region"
      aria-label="Acciones de la selección"
      className="studio-panel studio-panel-raised sticky bottom-4 z-20 flex flex-wrap items-center gap-x-3 gap-y-2 px-3 py-2"
    >
      <span aria-live="polite" className="text-body-sm font-medium text-fg-1 tabular-nums">
        {selectedCount(count, noun)}
        {detail ? <span className="font-normal text-fg-2">{` · ${detail}`}</span> : null}
      </span>
      <DeleteChosenButton count={count} noun={noun} loading={busy} description={confirmDescription} onConfirm={onDelete} />
      <Button type="button" variant="ghost" size="xs" className="ml-auto" disabled={busy} onClick={onClear}>
        <X aria-hidden />
        Quitar la selección
      </Button>
    </div>
  );
}
