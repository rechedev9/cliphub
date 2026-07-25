'use client';

import { useEffect, useState, type ReactElement } from 'react';
import Link from 'next/link';
import { RotateCcw, TriangleAlert } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';

/**
 * Route-level error boundary. FragForge ships packaged with `devTools: false`,
 * so before this existed a thrown render error put the user on Next's unstyled
 * "Application error: a client-side exception has occurred" — no brand, no
 * recovery path, and nothing to send anyone. Studio already keeps a `studio.log`
 * tail, so the least this can do is hand over a copyable digest.
 */
export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}): ReactElement {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    // The packaged app has no console to read, so the log is the only trail.
    console.error('[fragforge] route error', error);
  }, [error]);

  const diagnostics = [
    `mensaje: ${error.message}`,
    error.digest === undefined ? null : `digest: ${error.digest}`,
    `ruta: ${typeof window === 'undefined' ? '' : window.location.pathname}`,
  ]
    .filter((line) => line !== null)
    .join('\n');

  return (
    <div className="flex min-h-[60svh] items-center">
      <section
        role="alert"
        className="studio-panel studio-panel-raised flex w-full max-w-[42rem] flex-col gap-5 p-7"
      >
        <SectionEyebrow label="Error de la aplicación" />
        <div className="flex items-start gap-4">
          <span className="grid size-11 shrink-0 place-items-center rounded-md border border-destructive/45 bg-destructive/10 text-destructive">
            <TriangleAlert className="size-5" aria-hidden />
          </span>
          <div className="min-w-0">
            <h1 className="font-display text-title font-bold text-fg-1">Esta pantalla se ha caído</h1>
            <p className="mt-2 text-body text-fg-2">
              El resto de Studio sigue en pie y no se ha perdido ningún trabajo: las partidas, capturas y
              renders viven en el orquestador local, no en esta vista.
            </p>
          </div>
        </div>

        <pre className="max-h-40 overflow-auto rounded-md border border-border bg-surface-0 p-3 font-mono text-meta tracking-normal text-fg-2">
          {diagnostics}
        </pre>

        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" onClick={reset}>
            <RotateCcw aria-hidden />
            Reintentar
          </Button>
          <Button asChild variant="outline">
            <Link href="/matches">Volver a partidas</Link>
          </Button>
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              void navigator.clipboard.writeText(diagnostics).then(() => setCopied(true));
            }}
          >
            {copied ? 'Diagnóstico copiado' : 'Copiar diagnóstico'}
          </Button>
        </div>
      </section>
    </div>
  );
}
