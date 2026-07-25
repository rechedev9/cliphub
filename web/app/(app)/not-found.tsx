import type { ReactElement } from 'react';
import Link from 'next/link';
import { Compass } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';

/**
 * 404 *inside* the shell. The root `not-found.tsx` renders under
 * `app/layout.tsx` only, so an unmatched route inside the app group used to
 * drop the sidebar and the command strip — a mistyped URL ejected you from the
 * product.
 */
export default function AppNotFound(): ReactElement {
  return (
    <div className="flex min-h-[60svh] items-center">
      <section className="studio-panel flex w-full max-w-[42rem] flex-col gap-5 p-7">
        <SectionEyebrow label="Ruta desconocida" />
        <div className="flex items-start gap-4">
          <span className="grid size-11 shrink-0 place-items-center rounded-md border border-border-strong bg-surface-3 text-fg-2">
            <Compass className="size-5" aria-hidden />
          </span>
          <div className="min-w-0">
            <h1 className="font-display text-title font-bold text-fg-1">Aquí no hay nada</h1>
            <p className="mt-2 text-body text-fg-2">
              Esa dirección no corresponde a ninguna pantalla de Studio. Puede que el reel o la partida se
              hayan borrado, o que el enlace esté incompleto.
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Button asChild>
            <Link href="/matches">Ir a partidas</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/videos">Ver biblioteca</Link>
          </Button>
        </div>
      </section>
    </div>
  );
}
