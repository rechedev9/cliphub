'use client';

import { CircleAlert, PlugZap, SearchX } from 'lucide-react';
import type { ReactNode } from 'react';
import { loadErrorKind, offlineHint, offlineTitle, RETRY_HINT, type LoadErrorKind } from '@/lib/load-error';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';

type Failure = { error: unknown; kind?: never } | { kind: LoadErrorKind; error?: never };

export type LoadErrorStateProps = Failure & {
  title: string;
  notFoundTitle?: string;
  detail?: string;
  onRetry?: () => void;
  actions?: ReactNode;
  className?: string;
};

export function LoadErrorState({ title, notFoundTitle, detail, onRetry, actions, className, ...failure }: LoadErrorStateProps): ReactNode {
  const kind = failure.kind ?? loadErrorKind(failure.error);
  const retry =
    onRetry && kind !== 'not_found' ? (
      <Button type="button" variant="outline" size="sm" onClick={onRetry}>
        Reintentar
      </Button>
    ) : null;
  const buttons = retry || actions ? (
    <>
      {retry}
      {actions}
    </>
  ) : undefined;
  if (kind === 'offline') {
    return <EmptyState role="alert" icon={PlugZap} title={offlineTitle()} description={`${title}. ${offlineHint()}`} actions={buttons} className={className} />;
  }
  if (kind === 'not_found') {
    return (
      <EmptyState
        role="status"
        icon={SearchX}
        title={notFoundTitle ?? 'Ya no está en la biblioteca'}
        description="Puede que se haya borrado."
        actions={buttons}
        className={className}
      />
    );
  }
  return <EmptyState role="alert" icon={CircleAlert} title={title} description={detail ?? RETRY_HINT} actions={buttons} className={className} />;
}
