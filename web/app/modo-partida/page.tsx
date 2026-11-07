import type { Metadata } from 'next';
import { Suspense, type ReactNode } from 'react';
import { MatchMode } from '@/components/libreta/match-mode';

export const metadata: Metadata = { title: 'Modo partida' };

/** The compact window Studio opens from the Libreta's Tabla, without the app shell. */
export default function MatchModePage(): ReactNode {
  return (
    <div className="min-h-dvh bg-surface-0">
      <Suspense>
        <MatchMode />
      </Suspense>
    </div>
  );
}
