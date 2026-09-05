import type { ReactElement } from 'react';
import type { Metadata } from 'next';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Wordmark } from '@/components/brand/wordmark';

export const metadata: Metadata = { title: 'Ruta desconocida' };

/**
 * The 404 for routes outside the app group. Two corrections over v3: the wash
 * and the eyebrow no longer use `--destructive` — red signals
 * "error, failure, delete", and a stale bookmark is none of those, so painting
 * one red makes a mistyped URL look like data loss — and the two actions are
 * real `Button`s instead of hand-rolled 40px anchors with no focus ring.
 */
export default function NotFound(): ReactElement {
  return (
    <main className="relative flex min-h-svh flex-col items-center justify-center gap-6 px-6 text-center">
      <Link href="/" className="absolute top-6 left-6 rounded-md">
        <Wordmark />
      </Link>

      <p className="font-mono text-meta tracking-ultra text-fg-3 uppercase tabular-nums">404</p>
      <h1 className="font-display text-display-sm font-bold text-fg-1 uppercase sm:text-display">
        Esta página ha sido fraggeada
      </h1>
      <p className="max-w-md text-body text-fg-2">
        No encontramos esa página. Vuelve y elige una partida para forjarla en un reel.
      </p>

      <div className="mt-2 flex flex-wrap items-center justify-center gap-3">
        <Button asChild size="lg">
          <Link href="/matches">Volver a partidas</Link>
        </Button>
        <Button asChild variant="outline" size="lg">
          <Link href="/">Inicio</Link>
        </Button>
      </div>
    </main>
  );
}
