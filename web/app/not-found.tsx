import type { ReactElement } from 'react';
import type { Metadata } from 'next';
import Link from 'next/link';
import { Wordmark } from '@/components/brand/wordmark';
import { NotFoundAction } from '@/components/shell/not-found-action';

export const metadata: Metadata = { title: 'Página no encontrada' };

export default function NotFound(): ReactElement {
  return (
    <main className="relative flex min-h-svh flex-col items-center justify-center gap-6 px-6 text-center">
      <Link href="/" className="absolute top-6 left-6 rounded-md">
        <Wordmark />
      </Link>

      <p className="text-meta font-medium text-fg-3 tabular-nums">404</p>
      <h1 className="font-display text-display-sm font-bold text-fg-1 sm:text-display">Esta página no existe</h1>
      <p className="max-w-md text-body text-fg-2">Puede que se haya borrado o que el enlace esté incompleto.</p>

      <div className="mt-2 flex flex-wrap items-center justify-center gap-3">
        <NotFoundAction />
      </div>
    </main>
  );
}
