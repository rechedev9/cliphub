'use client';

import type { ReactNode } from 'react';
import { Globe, Loader2 } from 'lucide-react';
import type { PublicSyncState } from '@/hooks/use-public-team';
import { publicSyncLine } from '@/lib/analysis/public-team';
import { Button } from '@/components/ui/button';

export function PublicSyncNote({ state, onRetry }: { state: PublicSyncState; onRetry: () => void }): ReactNode {
  if (state.kind === 'idle') return null;
  if (state.kind === 'failed') {
    return (
      <div role="status" className="flex flex-col items-start gap-1.5 text-meta text-fg-3">
        <p>No se pudieron traer las partidas públicas de {state.name}. La búsqueda usa solo lo que ya está en tu biblioteca.</p>
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          Reintentar
        </Button>
      </div>
    );
  }
  const syncing = state.kind === 'syncing';
  const Icon = syncing ? Loader2 : Globe;
  return (
    <p role="status" className="flex items-start gap-1.5 text-meta text-fg-3">
      <Icon aria-hidden className={syncing ? 'mt-0.5 size-3.5 shrink-0 animate-spin motion-reduce:animate-none' : 'mt-0.5 size-3.5 shrink-0'} />
      <span>{publicSyncLine(state.progress, !syncing)}</span>
    </p>
  );
}
