import type { ReactNode } from 'react';
import Link from 'next/link';
import { ChevronDown } from 'lucide-react';
import { FOCUS_RING } from '@/components/ui/button';

export function DemoSourceHelp(): ReactNode {
  return (
    <details className="group/source studio-panel px-4 py-1">
      <summary className={`flex min-h-11 cursor-pointer list-none items-center justify-between gap-3 text-body-sm font-semibold text-fg-1 [&::-webkit-details-marker]:hidden ${FOCUS_RING}`}>
        ¿Qué es una demo y dónde la consigo?
        <ChevronDown aria-hidden className="size-4 shrink-0 text-fg-3 transition-transform group-open/source:rotate-180" />
      </summary>
      <div className="flex flex-col gap-3 border-t border-border-subtle py-4 text-body-sm text-fg-2">
        <p>Una demo es el archivo de una partida de CS2 (.dem). Contiene las rondas y las jugadas; ClipHub lo usa para grabar desde la vista del jugador que elijas.</p>
        <ul className="list-disc space-y-2 pl-5">
          <li><strong className="font-medium text-fg-1">FACEIT:</strong> descarga la demo desde la sala de la partida y carga aquí el archivo.</li>
          <li><strong className="font-medium text-fg-1">CS2:</strong> descarga la repetición de una partida o usa su código de compartir en la opción «Importar desde Steam».</li>
        </ul>
        <p>Puedes cargar .dem, .dem.zst, .zip o .rar. No necesitas conectar Steam si ya tienes el archivo.</p>
        <p>Si ya tienes un vídeo MP4, ve a <Link href="/streams" className={`text-primary underline underline-offset-4 ${FOCUS_RING}`}>Clips de stream</Link> para recortarlo.</p>
      </div>
    </details>
  );
}
