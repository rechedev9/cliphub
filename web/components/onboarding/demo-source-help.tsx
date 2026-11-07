import type { ReactNode } from 'react';
import type { ProduceFormat } from '@/lib/clips/routes';
import { cn } from '@/lib/utils';
import { FORMAT_TONE } from '@/components/studio/format-tone';

/** Where a first demo comes from, as two short cards beside the empty drop zone. */
export function DemoSourceHelp({ format }: { format: ProduceFormat }): ReactNode {
  const number = cn('font-display text-section font-extrabold leading-none', FORMAT_TONE[format].text);
  return (
    <ol aria-label="Dónde consigo una demo" className="flex flex-col gap-2.5">
      <li className="flex gap-3 rounded-[14px] bg-surface-2 p-4">
        <span aria-hidden className={number}>1</span>
        <div className="flex flex-col gap-1.5 text-body-sm text-fg-2">
          <p>
            <strong className="font-semibold text-fg-1">FACEIT:</strong> descarga la demo desde la sala de la partida y suelta aquí el archivo.
          </p>
          <p className="text-meta text-fg-3">Una demo es el archivo .dem de una partida de CS2: guarda todas sus rondas, y ClipHub graba desde la vista del jugador que elijas.</p>
        </div>
      </li>
      <li className="flex gap-3 rounded-[14px] bg-surface-2 p-4">
        <span aria-hidden className={number}>2</span>
        <div className="flex flex-col gap-1.5 text-body-sm text-fg-2">
          <p>
            <strong className="font-semibold text-fg-1">Matchmaking y Premier:</strong> descarga la repetición en CS2, o pega su código de compartir en «Importar desde Steam».
          </p>
          <p className="text-meta text-fg-3">No necesitas conectar Steam si ya tienes el archivo.</p>
        </div>
      </li>
    </ol>
  );
}
