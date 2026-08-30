'use client';

import type { ReactNode } from 'react';
import { MonitorPlay, ShieldCheck, Twitch } from 'lucide-react';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import type { JobProgress } from '@/lib/api/types';
import { LongOperation } from '@/components/studio/long-operation';
import { StreamOutputAside } from '@/components/streams/output-aside';
import { useElapsedSeconds } from '@/components/streams/use-elapsed-seconds';

const NOTES = [
  { icon: Twitch, text: 'Descarga con yt-dlp en este PC', tone: 'stream' as const },
  { icon: ShieldCheck, text: 'El vídeo no sale de tu máquina', tone: 'success' as const },
];

/** Stage 2: yt-dlp is fetching the source; live bytes come from the worker. */
export function StreamAcquiringCard({ title, progress }: { title?: string; progress?: JobProgress }): ReactNode {
  const elapsed = useElapsedSeconds(true);

  return (
    <div className="@container/acquire studio-panel studio-panel-raised max-w-4xl p-5 sm:p-7">
      <div className="grid gap-8 @[40rem]/acquire:grid-cols-[minmax(0,1fr)_15rem]">
        <div className="flex flex-col gap-4">
          <SectionEyebrow label="ADQUIRIENDO" accent="magenta" />
          <h2 className="font-display text-title font-bold text-fg-1">
            Descargando {title || 'el clip'}…
          </h2>
          <p className="text-body text-fg-2">
            Descargando y analizando el vídeo de origen. En cuanto termine se abre el editor con el
            plan guardado.
          </p>
          <LongOperation
            stage="DESCARGA + PROBE"
            detail="SIN CORTES TODAVÍA"
            progress={progress}
            elapsedSec={elapsed}
            tone="stream"
            className="mt-1"
          />
        </div>

        <StreamOutputAside heading="Salida" spec="9:16 · 1080p" icon={MonitorPlay} notes={NOTES} />
      </div>
    </div>
  );
}
