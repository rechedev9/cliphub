'use client';

import { createContext, useContext, type ReactNode } from 'react';
import { Users } from 'lucide-react';
import { Button } from '@/components/ui/button';

/** Drops a team view's lineup so it counts every player; null where no lineup narrows the rounds. */
type ShowEveryone = (() => void) | null;

const SampleLineup = createContext<ShowEveryone>(null);

export function SampleLineupProvider({ showEveryone, children }: { showEveryone: ShowEveryone; children: ReactNode }): ReactNode {
  return <SampleLineup value={showEveryone}>{children}</SampleLineup>;
}

/**
 * What a view says when its sample came back empty: its own message, or, while a
 * lineup narrows the rounds, that the lineup left none, with the way to count everyone.
 */
export function EmptySample({
  children,
  hiddenLocally = false,
}: {
  /** The view's own message, for a sample no lineup narrows. */
  children: ReactNode;
  /** The sample has rows that only the view's own controls hide, so the lineup is not why it is empty. */
  hiddenLocally?: boolean;
}): ReactNode {
  const showEveryone = useContext(SampleLineup);
  if (showEveryone === null || hiddenLocally) return children;
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <p className="text-body-sm text-fg-2">La alineación elegida no deja ninguna ronda con este filtro.</p>
      <Button type="button" variant="outline" size="sm" onClick={showEveryone}>
        <Users aria-hidden />
        Todos los jugadores
      </Button>
    </div>
  );
}
