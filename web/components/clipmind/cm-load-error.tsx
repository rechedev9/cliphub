'use client';

import Link from 'next/link';
import type { ReactNode } from 'react';
import { LoadErrorState } from '@/components/records/load-error-state';
import { Button } from '@/components/ui/button';
import type { CmResult } from '@/lib/clipmind/client';
import { loadErrorKind } from '@/lib/load-error';

export function CmLoadError({ failure, title, onRetry }: { failure: CmResult<unknown>; title: string; onRetry: () => void }): ReactNode {
  if (failure.ok) return null;
  if (loadErrorKind(failure) === 'offline') return <LoadErrorState kind="offline" title={title} onRetry={onRetry} />;
  if (failure.code === 'unauthorized') {
    return (
      <LoadErrorState
        kind="error"
        title={title}
        detail={failure.message}
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href="/iniciar-sesion">Iniciar sesión</Link>
          </Button>
        }
      />
    );
  }
  return <LoadErrorState kind="error" title={title} detail={failure.message} onRetry={onRetry} />;
}
