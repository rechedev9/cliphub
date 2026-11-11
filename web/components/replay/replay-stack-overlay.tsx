'use client';

import { useState, type ReactNode } from 'react';
import { ChevronUp, SlidersHorizontal } from 'lucide-react';
import { Button } from '@/components/ui/button';

export type ReplayStack = {
  /** One line naming the scope while the stack is folded, such as the team, the map and the rounds. */
  summary: string;
  content: ReactNode;
};

/**
 * The scope controls over the top left of the map, open as in EDGE. They fold into one button that
 * names the scope in use when the players under them need the room.
 */
export function ReplayStackOverlay({ stack }: { stack: ReplayStack }): ReactNode {
  const [open, setOpen] = useState(true);
  if (!open) {
    return (
      <Button
        type="button"
        variant="secondary"
        size="sm"
        className="absolute top-2 left-2 z-20 max-w-[calc(100%-1rem)] normal-case"
        aria-expanded={false}
        onClick={() => setOpen(true)}
      >
        <SlidersHorizontal aria-hidden />
        <span className="truncate">{stack.summary}</span>
      </Button>
    );
  }
  return (
    <div className="absolute top-2 left-2 z-20 flex max-h-[calc(100%-1rem)] flex-col gap-1 overflow-y-auto rounded-md border border-border bg-surface-1/95 p-2 [scrollbar-width:thin]">
      <Button type="button" variant="ghost" size="xs" className="self-end" aria-expanded onClick={() => setOpen(false)}>
        <ChevronUp aria-hidden />
        Ocultar filtros
      </Button>
      {stack.content}
    </div>
  );
}
