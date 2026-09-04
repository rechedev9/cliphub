import type { ReactNode } from 'react';
import Link from 'next/link';
import { ArrowUpRight, Clapperboard, MonitorPlay, Smartphone } from 'lucide-react';
import { newDemoHref, PRODUCE_FORMAT } from '@/lib/clips/routes';
import { FOCUS_RING } from '@/components/ui/button';
import { IconTile } from '@/components/studio/icon-tile';
import { cn } from '@/lib/utils';

const PATHS = [
  { title: 'Crear Short', format: 'Vertical · 9:16', source: 'Desde una demo de CS2',
    description: 'Elige jugadas y únelas en un vídeo con tu estilo y música.',
    href: newDemoHref({ format: PRODUCE_FORMAT.short }), icon: Smartphone, tone: 'primary' },
  { title: 'Crear vídeo largo', format: 'Horizontal · 16:9', source: 'Desde una demo de CS2',
    description: 'Todas las rondas desde la vista de un jugador, con HUD y voces del equipo.',
    href: newDemoHref({ format: PRODUCE_FORMAT.full }), icon: MonitorPlay, tone: 'primary' },
  { title: 'Recortar un stream', format: 'Un Short por corte · 9:16', source: 'Desde un enlace o MP4',
    description: 'Marca los cortes, ajusta el encuadre y añade facecam si la necesitas.',
    href: '/streams', icon: Clapperboard, tone: 'stream' },
] as const;

export function CreationPaths(): ReactNode {
  return (
    <section aria-label="Qué quieres crear" className="grid gap-3 @[48rem]/content:grid-cols-3">
      {PATHS.map((path) => (
        <Link key={path.title} href={path.href} className={cn(
          'studio-panel studio-panel-interactive flex min-w-0 flex-col gap-3 border-border-strong p-4', FOCUS_RING,
        )}>
          <div className="flex items-center justify-between gap-3">
            <IconTile icon={path.icon} tone={path.tone} size="sm" />
            <ArrowUpRight aria-hidden className="size-4 text-fg-3" />
          </div>
          <div>
            <h2 className="font-display text-title font-semibold text-fg-1">{path.title}</h2>
            <p className="text-body-sm text-fg-2">{path.source}</p>
          </div>
          <p className="text-body-sm text-fg-2">{path.description}</p>
          <p className="mt-auto pt-2 font-mono text-meta text-fg-3">{path.format}</p>
        </Link>
      ))}
    </section>
  );
}
