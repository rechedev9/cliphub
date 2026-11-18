'use client';

import type { ReactNode } from 'react';
import { RotateCw } from 'lucide-react';
import {
  RANKING_PAGE, RANKING_SIZE, rankingChoiceLabel, rankingErrorMessage, rankingHasMore, rankingNote,
  type RankingChoice, type RankingListState, type RankingOption,
} from '@/lib/faceit-ranking';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PlayerRows } from './player-rows';
import { RankingPicker } from './ranking-picker';

const READ_AT = new Intl.DateTimeFormat('es-ES', { dateStyle: 'short', timeStyle: 'short' });

/** The Ranking tab: the selector, then the top of the chosen entry 20 at a time, and when it was read. */
export function RankingPanel({ options, choice, onChoose, list, onMore, onRetry, selectedID, onSelect, followedIDs }: {
  options: readonly RankingOption[];
  /** Null until the countries and the system country are known. */
  choice: RankingChoice | null;
  onChoose: (choice: RankingChoice) => void;
  list: RankingListState;
  onMore: () => void;
  onRetry: () => void;
  selectedID: string | null;
  onSelect: (id: string) => void;
  followedIDs: ReadonlySet<string>;
}): ReactNode {
  const label = choice === null ? '' : rankingChoiceLabel(options, choice);
  let body: ReactNode;
  if (choice === null || list.status === 'loading' || list.status === 'idle') {
    body = <div role="status" aria-label="Cargando ranking" className="space-y-2 border-t border-border p-4">
      <Skeleton className="h-12 w-full" /><Skeleton className="h-12 w-full" /><Skeleton className="h-12 w-full" />
    </div>;
  } else if (list.status === 'error') {
    body = <div role="alert" className="space-y-3 border-t border-border p-4 text-body-sm text-fg-2">
      <p>{rankingErrorMessage(list.errorCode)}</p>
      <Button variant="outline" size="sm" onClick={onRetry}><RotateCw aria-hidden /> Reintentar</Button>
    </div>;
  } else {
    const shown = Math.min(list.total, RANKING_SIZE);
    body = <PlayerRows key={`${choice.kind}:${choice.code}`} players={list.players} label={label} selectedID={selectedID} onSelect={onSelect}
      followedIDs={followedIDs} empty={`No hay jugadores de ${label} en el ranking de FACEIT.`}
      footer={<div className="space-y-3 border-t border-border p-4 text-body-sm">
        {rankingHasMore(list) ? <Button variant="outline" size="sm" className="w-full" onClick={onMore}
          loading={list.loadingMore} loadingText="Cargando…">Cargar {Math.min(RANKING_PAGE, shown - list.players.length)} más</Button> : null}
        {list.moreErrorCode !== undefined ? <p role="alert" className="text-destructive">{rankingErrorMessage(list.moreErrorCode)}</p> : null}
        <p className="text-fg-3">{shown === 0 ? null : `Puestos 1 a ${list.players.length} de ${shown}. `}
          <span className={cn(list.stale && 'text-warning')}>{rankingNote(list, READ_AT.format(list.fetchedAt))}</span>
        </p>
      </div>} />;
  }
  return (
    <>
      <div className="px-4 pb-4">
        {choice === null ? <Skeleton className="h-10 w-full" />
          : <RankingPicker options={options} value={choice} onChange={onChoose} />}
      </div>
      {body}
    </>
  );
}
