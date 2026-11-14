'use client';

import Link from 'next/link';
import { useSyncExternalStore, type ReactElement } from 'react';
import { hasDemoCookie } from '@/lib/clipmind/demo';

function subscribe(): () => void {
  return () => {};
}

/** Tells a visitor of the public demo that it is read-only and how to start their own. */
export function DemoBanner(): ReactElement | null {
  const inDemo = useSyncExternalStore(subscribe, () => hasDemoCookie(document.cookie), () => false);
  if (!inDemo) return null;
  return (
    <div role="status" className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1 border-b border-border bg-card px-4 py-2 text-sm">
      <span>Estás viendo una demo de solo lectura con partidos pro reales.</span>
      <Link href="/crear-cuenta" className="font-semibold text-primary underline-offset-4 hover:underline">
        Crea tu cuenta gratis para analizar tus demos
      </Link>
    </div>
  );
}
