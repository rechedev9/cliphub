'use client';

import Link from 'next/link';
import { Cable, Download, RefreshCw } from 'lucide-react';
import type { ReactElement, ReactNode } from 'react';
import { Wordmark } from '@/components/brand/wordmark';
import { Button } from '@/components/ui/button';
import { useAgentTransport } from './agent-transport';

export function AgentGate({ required, children }: { required: boolean; children: ReactNode }): ReactElement {
  const { state, reconnect } = useAgentTransport();
  if (!required || state.status === 'local' || state.status === 'ready') return <>{children}</>;

  const connecting = state.status === 'connecting';
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-[42rem] flex-col justify-center gap-8 px-6 py-12">
      <Wordmark />
      <section className="studio-panel studio-panel-raised flex flex-col gap-6 p-7 sm:p-9">
        <span className="flex size-12 items-center justify-center rounded-md border border-border-strong bg-surface-3 text-primary shadow-[var(--elev-1)]">
          <Cable aria-hidden className="size-6" />
        </span>
        <div className="flex flex-col gap-2">
          <h1 className="font-display text-display-sm font-bold text-fg-1">
            {connecting ? 'Conectando con este PC' : 'Instala ClipHub Agent'}
          </h1>
          <p className="text-body text-fg-2">
            {state.status === 'error'
              ? state.error
              : 'El agente mantiene HLAE, FFmpeg, las demos y los vídeos dentro de tu equipo.'}
          </p>
        </div>
        <div className="flex flex-col gap-3 sm:flex-row">
          <Button asChild variant="hero" size="lg">
            <Link href="/api/installer"><Download aria-hidden /> INSTALAR PARA WINDOWS</Link>
          </Button>
          <Button type="button" variant="outline" size="lg" onClick={reconnect} disabled={connecting}>
            <RefreshCw aria-hidden /> VOLVER A COMPROBAR
          </Button>
        </div>
      </section>
    </main>
  );
}
