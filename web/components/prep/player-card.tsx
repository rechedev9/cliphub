import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

function initials(name: string): string {
  return name.replace(/[^\p{L}\p{N}]/gu, '').slice(0, 2).toUpperCase();
}

/**
 * The EDGE player card: a header with the initials in a square (Studio has no player photos), the name and one line of
 * facts, then the player's windows. `color` tints the initials when the card matches a dot on a map.
 */
export function PlayerCard({
  name,
  facts,
  color,
  children,
  className,
}: {
  name: string;
  facts: ReactNode;
  color?: string;
  children: ReactNode;
  className?: string;
}): ReactNode {
  return (
    <article aria-label={name} className={cn('studio-panel flex flex-col print:break-inside-avoid', className)}>
      <header className="flex items-center gap-2.5 border-b border-border bg-surface-3 px-3 py-2">
        <span
          aria-hidden
          className="grid size-8 shrink-0 place-items-center rounded-md border border-border-strong bg-surface-2 text-body-sm font-bold text-fg-1"
          style={color === undefined ? undefined : { color, borderColor: color }}
        >
          {initials(name)}
        </span>
        <div className="flex min-w-0 flex-col">
          <h3 className="truncate text-body font-semibold text-fg-1">{name}</h3>
          <p className="truncate text-meta text-fg-3">{facts}</p>
        </div>
      </header>
      <div className="flex flex-col gap-2 px-3 py-2.5">{children}</div>
    </article>
  );
}
