'use client';

import type { ReactNode } from 'react';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';

/** `or`: other keys that do the same, shown after an "o". */
type Shortcut = { keys: readonly string[]; or?: readonly string[]; does: string };

const PLAYBACK: readonly Shortcut[] = [
  { keys: ['Espacio'], does: 'Reproducir o pausar' },
  { keys: ['←', '→'], does: 'Retroceder o avanzar 1 segundo' },
  { keys: ['Mayús', '←', '→'], does: 'Saltar al evento anterior o siguiente' },
  { keys: ['[', ']'], or: ['J', 'K'], does: 'Ronda anterior o siguiente' },
  { keys: ['M'], does: 'Superponer rondas, o dejar de hacerlo' },
  { keys: ['B'], does: 'Añadir la ronda a la última playlist elegida, o quitarla' },
  { keys: [',', '.'], does: 'Plegar el panel de la ronda o el de jugadores' },
  { keys: ['P'], does: 'Pantalla completa; otra vez o Esc para salir' },
  { keys: ['?'], does: 'Mostrar estos atajos' },
];

const MAP: readonly Shortcut[] = [
  { keys: ['Clic'], does: 'Dejar a un jugador en primer plano; los demás quedan translúcidos' },
  { keys: ['Mayús', 'Clic'], does: 'Sumar otro jugador al primer plano' },
  { keys: ['F'], does: 'Centrar el mapa en el jugador elegido; otra vez para soltarlo' },
  { keys: ['S'], or: ['Clic derecho'], does: 'Copiar el setpos del jugador bajo el cursor' },
  { keys: ['G'], does: 'Abrir el GIF del tramo; otra vez para exportarlo' },
  { keys: ['Esc'], does: 'Dejar de seguir y volver a todo el equipo' },
  { keys: ['Rueda'], does: 'Acercar o alejar' },
  { keys: ['Arrastrar'], does: 'Mover el mapa' },
];

function Keys({ keys }: { keys: readonly string[] }): ReactNode {
  return keys.map((key) => (
    <kbd key={key} className="min-w-6 rounded-sm border border-border-strong bg-surface-2 px-1.5 text-center text-meta leading-5 font-semibold text-fg-1">
      {key}
    </kbd>
  ));
}

function Group({ title, items }: { title: string; items: readonly Shortcut[] }): ReactNode {
  return (
    <section className="flex flex-col gap-1.5">
      <h3 className="text-meta font-semibold text-fg-3">{title}</h3>
      <dl className="flex flex-col gap-1">
        {items.map((item) => (
          <div key={item.does} className="flex items-center justify-between gap-4">
            <dt className="text-body-sm text-fg-1">{item.does}</dt>
            <dd className="flex shrink-0 items-center gap-1">
              <Keys keys={item.keys} />
              {item.or === undefined ? null : (
                <>
                  <span className="text-meta text-fg-3">o</span>
                  <Keys keys={item.or} />
                </>
              )}
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
          <DialogDescription>Funcionan mientras el Replay está en pantalla y no escribes en un campo.</DialogDescription>
        </DialogHeader>
        <Group title="Reproducción" items={PLAYBACK} />
        <Group title="Mapa" items={MAP} />
      </DialogContent>
    </Dialog>
  );
}
