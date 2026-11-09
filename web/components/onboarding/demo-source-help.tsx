import type { ReactNode } from 'react';

const SOURCES = [
  {
    name: 'FACEIT',
    how: 'Descarga la demo desde la sala de la partida y suelta aquí el archivo.',
  },
  {
    name: 'Matchmaking y Premier',
    how: 'Descarga la repetición en CS2, o pega su código de compartir en «Importar desde Steam». No hace falta conectar Steam si ya tienes el archivo.',
  },
] as const;

/**
 * Where a first demo comes from: two alternative sources on one surface. `titled`
 * is false where a disclosure summary already asks the question.
 */
export function DemoSourceHelp({ titled = true }: { titled?: boolean }): ReactNode {
  return (
    <section aria-label="Dónde consigo una demo" className="flex flex-col gap-3 self-start rounded-xl bg-surface-2 p-4">
      <div className="flex flex-col gap-1">
        {titled ? <h2 className="text-body font-semibold text-fg-1">Dónde consigo una demo</h2> : null}
        <p className="text-meta text-fg-3">El archivo .dem de una partida de CS2 guarda todas sus rondas.</p>
      </div>
      <ul className="flex flex-col divide-y divide-border-subtle">
        {SOURCES.map((source) => (
          <li key={source.name} className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0">
            <span className="text-body-sm font-semibold text-fg-1">{source.name}</span>
            <span className="text-body-sm text-fg-2">{source.how}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
