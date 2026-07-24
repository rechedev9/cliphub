import type { ReactNode } from 'react';
import type { StreamVariant } from '@/lib/api/streams';
import { cn } from '@/lib/utils';

type GlyphBand = { share: number; kind: 'cam' | 'game' };

/**
 * The band stack each variant produces, in output order. Shares mirror the
 * proportions the preview and the render registry use, so the glyph is a scale
 * model of the output rather than a decorative icon.
 */
const VARIANT_BANDS = {
  'streamer-vertical-stack-40-60': [
    { share: 40, kind: 'cam' },
    { share: 60, kind: 'game' },
  ],
  'streamer-vertical-stack': [
    { share: 26, kind: 'cam' },
    { share: 48, kind: 'game' },
    { share: 26, kind: 'cam' },
  ],
  'streamer-fullframe-nocam': [{ share: 100, kind: 'game' }],
} as const satisfies Record<StreamVariant, readonly GlyphBand[]>;

/**
 * A 9:16 plate in a perspective box, with the facecam bands floating above the
 * gameplay plane. The layout choice is otherwise abstract — three names and two
 * lines of copy — and turning it into a physical object is what makes it
 * legible at a glance.
 *
 * Every transform is multiplied by `--shell-depth`, so reduced motion, forced
 * colours, an inactive window and the desktop efficiency profile all flatten it
 * to a plain stacked diagram without a second rule. Purely decorative: the card
 * around it carries the label.
 */
export function LayoutGlyph({ variant, selected }: { variant: StreamVariant; selected: boolean }): ReactNode {
  const bands: readonly GlyphBand[] = VARIANT_BANDS[variant];

  return (
    <span aria-hidden className="grid h-14 w-12 shrink-0 place-items-center [perspective:340px]">
      <span
        className={cn(
          'flex h-12 w-[1.6875rem] flex-col border bg-surface-0 transition-[border-color,transform] duration-(--dur-base) ease-standard [transform-style:preserve-3d]',
          selected ? 'border-stream/70' : 'border-border-strong',
        )}
        style={{
          transform:
            'rotateY(calc(var(--shell-depth) * -20deg)) rotateX(calc(var(--shell-depth) * 9deg))',
        }}
      >
        {bands.map((band, index) => (
          <span
            key={`${band.kind}-${index}`}
            style={{
              height: `${band.share}%`,
              transform: band.kind === 'cam' ? 'translateZ(calc(var(--shell-depth) * 6px))' : undefined,
            }}
            className={cn(
              'block transition-colors duration-(--dur-fast) ease-standard',
              band.kind === 'game' && 'bg-surface-3',
              band.kind === 'cam' && (selected ? 'bg-stream shadow-[var(--glow-stream-md)]' : 'bg-fg-4'),
            )}
          />
        ))}
      </span>
    </span>
  );
}
