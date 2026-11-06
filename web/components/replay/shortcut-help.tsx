'use client';

import type { ReactNode } from 'react';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';

type Shortcut = { keys: readonly string[]; does: string };

const PLAYBACK: readonly Shortcut[] = [
  { keys: ['Espacio'], does: 'Reproducir o pausar' },
  { keys: ['←', '→'], does: 'Retroceder o avanzar 1 segundo' },
  { keys: ['Mayús', '←', '→'], does: 'Saltar al evento anterior o siguiente' },
  { keys: ['[', ']'], does: 'Ronda anterior o siguiente' },
  { keys: ['M'], does: 'Activar o quitar Multi ronda' },
  { keys: ['?'], does: 'Mostrar estos atajos' },
];

const MAP: readonly Shortcut[] = [
  { keys: ['F'], does: 'Seguir al jugador seleccionado; otra vez para soltarlo' },
  { keys: ['Esc'], does: 'Dejar de seguir y quitar la selección' },
  { keys: ['Rueda'], does: 'Acercar o alejar' },
  { keys: ['Arrastrar'], does: 'Mover el mapa (en 3D, girar la cámara)' },
  { keys: ['Clic derecho'], does: 'En 3D, desplazar la cámara' },
];

const THREE_D: readonly Shortcut[] = [
  { keys: ['1'], does: 'Cámara en órbita' },
  { keys: ['2'], does: 'Cámara cenital' },
  { keys: ['3'], does: 'Cámara del jugador' },
];

function Group({ title, items }: { title: string; items: readonly Shortcut[] }): ReactNode {
  return (
    <section className="flex flex-col gap-1.5">
      <h3 className="text-meta font-semibold text-fg-3">{title}</h3>
      <dl className="flex flex-col gap-1">
        {items.map((item) => (
          <div key={item.does} className="flex items-center justify-between gap-4">
            <dt className="text-body-sm text-fg-1">{item.does}</dt>
            <dd className="flex shrink-0 gap-1">
              {item.keys.map((key) => (
                <kbd key={key} className="min-w-6 rounded-sm border border-border-strong bg-surface-2 px-1.5 text-center text-meta leading-5 font-semibold text-fg-1">
                  {key}
                </kbd>
              ))}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

/** The replay's keyboard shortcuts, opened with "?" or the header button. */
export function ShortcutHelp({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-4 sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Atajos de teclado</DialogTitle>
          <DialogDescription>Funcionan mientras la repetición está en pantalla y no escribes en un campo.</DialogDescription>
        </DialogHeader>
        <Group title="Reproducción" items={PLAYBACK} />
        <Group title="Mapa" items={MAP} />
        <Group title="Vista 3D" items={THREE_D} />
      </DialogContent>
    </Dialog>
  );
}
