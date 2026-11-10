'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Copy } from 'lucide-react';
import { Button } from '@/components/ui/button';

/** Where the chip sits: left of the hovered row, so the player stays readable. */
export type ChipAnchor = { readonly slot: number; readonly top: number; readonly left: number; readonly height: number };

const LEAVE_MS = 180;

/**
 * The row under the pointer, kept a moment after the pointer leaves so it can reach the chip.
 * `hold` and `release` are for the chip itself.
 */
export function useHoverAnchor(): {
  anchor: ChipAnchor | null;
  enter: (slot: number, element: HTMLElement) => void;
  leave: () => void;
  hold: () => void;
} {
  const [anchor, setAnchor] = useState<ChipAnchor | null>(null);
  const timer = useRef<number | undefined>(undefined);
  const hold = useCallback(() => window.clearTimeout(timer.current), []);
  const enter = useCallback(
    (slot: number, element: HTMLElement) => {
      hold();
      const rect = element.getBoundingClientRect();
      setAnchor({ slot, top: rect.top, left: rect.left, height: rect.height });
    },
    [hold],
  );
  const leave = useCallback(() => {
    hold();
    timer.current = window.setTimeout(() => setAnchor(null), LEAVE_MS);
  }, [hold]);
  useEffect(() => hold, [hold]);
  return { anchor, enter, leave, hold };
}

/** EDGE's setpos on hover: the console line of the hovered player and a button that copies it. */
export function SetposHoverChip({ anchor, command, onCopy, onHold, onLeave }: {
  anchor: ChipAnchor;
  command: string;
  onCopy: () => void;
  onHold: () => void;
  onLeave: () => void;
}): ReactNode {
  return (
    <div
      role="group"
      aria-label="Setpos del jugador"
      onMouseEnter={onHold}
      onMouseLeave={onLeave}
      className="fixed z-50 flex max-w-80 items-center gap-1 rounded-md border border-border-strong bg-surface-2 py-0.5 pr-0.5 pl-2 shadow-lg"
      style={{ top: anchor.top + anchor.height / 2, left: anchor.left - 8, transform: 'translate(-100%, -50%)' }}
    >
      <code className="min-w-0 truncate font-mono text-meta text-fg-1">{command}</code>
      <Button type="button" variant="ghost" size="icon-xs" aria-label="Copiar setpos" title="Copiar setpos" onClick={onCopy}>
        <Copy aria-hidden />
      </Button>
    </div>
  );
}
