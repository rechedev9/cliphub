'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { PanelLeftClose, PanelLeftOpen, PanelRightClose, PanelRightOpen } from 'lucide-react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { entryKey } from '@/lib/replay/playlist';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export type PanelSide = 'left' | 'right';
export type PanelFolds = Record<PanelSide, boolean>;

const FOLDS_KEY = 'cliphub.replay.panels';
const OPEN: PanelFolds = { left: false, right: false };

/** Which side panels are folded, remembered on this PC. */
export function usePanelFolds(): [PanelFolds, (side: PanelSide) => void] {
  const [folds, setFolds] = useState<PanelFolds>(OPEN);
  useEffect(() => {
    try {
      const stored = window.localStorage.getItem(FOLDS_KEY) ?? '';
      setFolds({ left: stored.includes('left'), right: stored.includes('right') });
    } catch {
      // Storage is blocked: both panels start open.
    }
  }, []);
  const toggle = useCallback((side: PanelSide) => {
    setFolds((current) => {
      const next = { ...current, [side]: !current[side] };
      try {
        window.localStorage.setItem(FOLDS_KEY, [next.left ? 'left' : '', next.right ? 'right' : ''].join(' ').trim());
      } catch {
        // Storage is blocked: the fold lasts for this visit.
      }
      return next;
    });
  }, []);
  return [folds, toggle];
}

const FOLD_COPY: Record<PanelSide, { panel: string; key: string }> = {
  left: { panel: 'el panel de la ronda', key: ',' },
  right: { panel: 'el panel de jugadores', key: '.' },
};

/** Folds a side panel into its narrow strip, or opens it again. */
export function PanelFoldButton({ side, folded, onToggle, className }: { side: PanelSide; folded: boolean; onToggle: () => void; className?: string }): ReactNode {
  const copy = FOLD_COPY[side];
  const label = `${folded ? 'Desplegar' : 'Plegar'} ${copy.panel} (${copy.key})`;
  let icon = folded ? <PanelLeftOpen aria-hidden /> : <PanelLeftClose aria-hidden />;
  if (side === 'right') icon = folded ? <PanelRightOpen aria-hidden /> : <PanelRightClose aria-hidden />;
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-expanded={!folded}
      onClick={onToggle}
      className={cn('grid size-7 shrink-0 place-items-center rounded-sm text-fg-3 outline-none hover:bg-surface-3 hover:text-fg-1 [&_svg]:size-4', FOCUS_RING, className)}
    >
      {icon}
    </button>
  );
}

/** The folded left panel: the rounds as a column of numbers. */
export function FoldedRounds({ entries, index, onSelect }: { entries: readonly ReplayRef[]; index: number; onSelect: (index: number) => void }): ReactNode {
  return (
    <ol aria-label="Rondas" className="flex flex-col items-center gap-0.5">
      {entries.map((entry, at) => (
        <li key={entryKey(entry)}>
          <button
            type="button"
            aria-current={at === index ? 'true' : undefined}
            aria-label={`Ronda ${entry.round}`}
            onClick={() => onSelect(at)}
            className={cn(
              'grid h-6 w-8 place-items-center rounded-sm text-meta tabular-nums outline-none',
              at === index ? 'bg-surface-4 font-semibold text-fg-1' : 'text-fg-3 hover:bg-surface-3 hover:text-fg-1',
              FOCUS_RING,
            )}
          >
            {entry.round}
          </button>
        </li>
      ))}
    </ol>
  );
}
