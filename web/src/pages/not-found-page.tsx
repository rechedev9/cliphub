import type { ReactElement } from 'react';
import Link from '@/src/compat/link';
import { Button } from '@/components/ui/button';
import { Wordmark } from '@/components/brand/wordmark';

export default function NotFoundPage(): ReactElement {
  return <main className="flex min-h-svh flex-col items-center justify-center gap-6 px-6"><Wordmark /><h1 className="font-display text-title font-bold">Página no encontrada</h1><Button asChild><Link href="/matches">Volver a partidas</Link></Button></main>;
}
