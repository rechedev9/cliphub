import type { ReactElement, ReactNode } from 'react';
import { Clapperboard, UploadCloud, UserSearch } from 'lucide-react';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { EntryDoor } from '@/components/onboarding/entry-door';
import { cn } from '@/lib/utils';

/** What ClipHub needs from the user before it can do anything, per route. */
const DOORS = [
  {
    href: '/upload',
    icon: UploadCloud,
    title: 'Sube una demo',
    description: 'Un .dem de CS2, o un .rar/.zip con la serie entera.',
    emphasis: 'primary',
  },
  {
    href: '/streams',
    icon: Clapperboard,
    title: 'Corta un stream',
    description: 'Una URL de Twitch o Kick, o un MP4 del disco.',
    tone: 'stream',
  },
  {
    href: '/players',
    icon: UserSearch,
    title: 'Busca un jugador',
    description: 'Un nick de FACEIT, para elegir demo antes de bajarla.',
  },
] as const;

/** The four stages every job walks, whichever door it came through. */
const STAGES = [
  { key: 'COLA', text: 'El trabajo entra en la cola local y espera turno.' },
  { key: 'CAPTURA', text: 'CS2 y HLAE graban las jugadas en este PC.' },
  { key: 'EDICIÓN', text: 'FFmpeg monta, recorta y aplica los efectos.' },
  { key: 'LISTO', text: 'El MP4, la portada y el texto quedan en Biblioteca.' },
] as const;

/** Real plates, as a fraction of the 1792×1008 frame, covering the baked art. */
const PLATES = {
  left: '@[64rem]/content:left-[4.4%] @[64rem]/content:top-[19.2%] @[64rem]/content:w-[28.8%] @[64rem]/content:min-h-[57.4%]',
  right: '@[64rem]/content:left-[62.6%] @[64rem]/content:top-[18.6%] @[64rem]/content:w-[33.4%] @[64rem]/content:min-h-[58.4%]',
} as const;

/** Inicio hero: one plate per hand, stacked below the bust on narrow widths. */
export function GuideStage(): ReactElement {
  return (
    <div className="relative flex flex-col gap-4 @[64rem]/content:block">
      <img
        src="/brand/onboarding-guide.webp"
        width={1792}
        height={1008}
        alt=""
        aria-hidden
        // Decorative bust crop; plates carry the real text so alt stays empty.
        className="mx-auto aspect-[1/2] w-full max-w-56 object-cover object-[47%_50%] [mask-image:linear-gradient(to_bottom,black_74%,transparent)] @[64rem]/content:aspect-auto @[64rem]/content:h-auto @[64rem]/content:max-w-none @[64rem]/content:object-fill"
      />

      <Plate
        className={PLATES.left}
        title="QUÉ PUEDES HACER"
        footer="Todas acaban en Biblioteca"
      >
        <div className="flex flex-col gap-2">
          {DOORS.map((door) => (
            <EntryDoor key={door.href} {...door} />
          ))}
        </div>
      </Plate>

      <Plate
        className={PLATES.right}
        title="QUÉ PASA DESPUÉS"
        footer="Sigue el progreso en la barra superior"
      >
        <ol className="flex flex-col gap-3">
          {STAGES.map((stage, index) => (
            <li key={stage.key} className="flex items-baseline gap-3">
              <span className="w-5 shrink-0 font-mono text-meta tabular-nums text-fg-3">
                {String(index + 1).padStart(2, '0')}
              </span>
              <span className="flex min-w-0 flex-col gap-0.5">
                <span
                  className={cn(
                    'font-mono text-meta uppercase tracking-wider',
                    // CAPTURA is the REC stage, so it carries the stream signal
                    // here for the same reason PipelineSteps gives it one.
                    stage.key === 'CAPTURA' ? 'text-stream-text' : 'text-primary',
                  )}
                >
                  {stage.key}
                </span>
                <span className="text-meta text-fg-2 tracking-normal">{stage.text}</span>
              </span>
            </li>
          ))}
        </ol>
      </Plate>
    </div>
  );
}

function Plate({
  title,
  footer,
  className,
  children,
}: {
  title: string;
  footer: string;
  className: string;
  children: ReactNode;
}): ReactElement {
  return (
    <section
      data-slot="guide-plate"
      className={cn(
        'studio-panel flex flex-col gap-3 p-4',
        'border-border-accent [box-shadow:var(--elev-2),var(--glow-primary-sm)]',
        // min-h, not h: the plate must cover the baked one underneath, but a
        // fixed height would clip a door row rather than let the plate grow.
        '@[64rem]/content:absolute',
        className,
      )}
    >
      <SectionEyebrow label={title} />
      {children}
      {/*
        mt-auto, not a spacer: the plate is taller than its content because it
        has to cover the baked art, so the slack is pushed to one deliberate
        closing line instead of leaving a half-empty panel.
      */}
      <p className="mt-auto pt-2 font-mono text-meta uppercase tracking-wider text-fg-3">{footer}</p>
    </section>
  );
}
