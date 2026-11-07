import { useId, type ReactNode } from 'react';
import Link from 'next/link';
import { Radar } from 'lucide-react';
import type { StoryFact } from '@/lib/library/match-story';
import { MATCH_TAB_QUERY, MATCH_TACTICAL_TAB } from '@/lib/library/rounds';
import { Button } from '@/components/ui/button';

/** "Cómo fue la partida": the facts the rounds prove, each beside the rounds it is about. */
export function MatchStoryPanel({
  facts,
  pathname,
  onOpenTactical,
}: {
  facts: readonly StoryFact[];
  pathname: string;
  onOpenTactical: () => void;
}): ReactNode {
  const titleId = useId();
  return (
    <section aria-labelledby={titleId} className="studio-panel flex flex-col gap-3.5 p-5">
      <h2 id={titleId} className="text-title font-semibold text-fg-1">
        Cómo fue la partida
      </h2>
      <ol className="flex flex-col gap-3">
        {facts.map((fact) => (
          <li key={`${fact.rounds}-${fact.text}`} className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-baseline gap-3">
            <span className="text-meta font-medium text-fg-3">{fact.rounds}</span>
            <span className="text-body-sm text-fg-1">{fact.text}</span>
          </li>
        ))}
      </ol>
      <div className="mt-1 flex flex-wrap gap-2">
        <Button asChild variant="outline" size="sm">
          <Link href={`${pathname}?${MATCH_TAB_QUERY}=${MATCH_TACTICAL_TAB}`} onClick={onOpenTactical}>
            <Radar aria-hidden />
            Abrir en Táctica
          </Link>
        </Button>
      </div>
    </section>
  );
}
