import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { MediaFrame } from '@/components/studio/media-frame';
import { cn } from '@/lib/utils';

export type StreamOutputNoteTone = 'stream' | 'success' | 'neutral';

const NOTE_TONE_CLASS = {
  stream: 'text-stream-text',
  success: 'text-success',
  neutral: 'text-fg-3',
} as const satisfies Record<StreamOutputNoteTone, string>;

export type StreamOutputNote = {
  icon: LucideIcon;
  text: string;
  tone?: StreamOutputNoteTone;
};

export type StreamOutputAsideProps = {
  /** Mono eyebrow on the left of the header, e.g. "SALIDA". */
  heading: string;
  /** Mono spec on the right, e.g. "9:16 · 1080p". Real output facts only. */
  spec: string;
  /** Glyph painted inside the shape proxy. */
  icon: LucideIcon;
  notes?: readonly StreamOutputNote[];
  /** Extra rows under the notes, e.g. a live stage readout. */
  children?: ReactNode;
  className?: string;
};

/**
 * The output summary aside: a 9:16 shape proxy plus the facts about what this
 * screen will produce. design.md's Stream clips contract asks for exactly this
 * ("on wide screens, use otherwise-empty space for a small output/processing
 * summary"), and it was the one piece of the 3000-line screen worth keeping —
 * so it is now a component the source panel, the acquiring stage and the render
 * stage all share instead of a block of markup inside one card.
 *
 * The proxy is a real `MediaFrame`, so the vertical shape, the scanline pitch
 * and the well surface are the same object the Library and the render results
 * use.
 */
export function StreamOutputAside({
  heading,
  spec,
  icon: Icon,
  notes,
  children,
  className,
}: StreamOutputAsideProps): ReactNode {
  return (
    <aside
      className={cn(
        'flex flex-col gap-5 border border-stream/25 bg-surface-1 p-5 shadow-[var(--elev-0)]',
        className,
      )}
    >
      <div className="flex items-center justify-between gap-3 font-mono text-meta uppercase tracking-wider">
        <span className="text-fg-3">{heading}</span>
        <span className="text-stream-text tabular-nums">{spec}</span>
      </div>

      <MediaFrame
        aspect="9:16"
        scanline
        className="mx-auto max-w-[8.5rem] border border-stream/30 studio-rim"
        fallback={
          <span className="grid size-full place-items-center bg-[linear-gradient(180deg,color-mix(in_oklch,var(--stream)_14%,transparent),transparent_70%)]">
            <Icon className="size-7 text-stream-text" aria-hidden />
          </span>
        }
      />

      {notes && notes.length > 0 ? (
        <ul className="flex flex-col gap-2.5 text-body-sm text-fg-2">
          {notes.map((note) => (
            <li key={note.text} className="flex gap-2.5">
              <note.icon
                aria-hidden
                className={cn('mt-0.5 size-4 shrink-0', NOTE_TONE_CLASS[note.tone ?? 'neutral'])}
              />
              {note.text}
            </li>
          ))}
        </ul>
      ) : null}

      {children}
    </aside>
  );
}
