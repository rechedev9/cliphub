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
 * A 9:16 plate showing the facecam bands over the gameplay plane. `md` floats
 * the cam bands in a perspective box (multiplied by `--shell-depth`, so reduced
 * motion and the efficiency profile flatten it); `sm` is the flat 9×16 mark
 * for a segmented control. Purely decorative: the control carries the label.
 */
export function LayoutGlyph({
  variant,
  selected,
  size = 'md',
}: {
  variant: StreamVariant;
  selected: boolean;
  size?: 'sm' | 'md';
}): ReactNode {
  const bands: readonly GlyphBand[] = VARIANT_BANDS[variant];

  if (size === 'sm') {
    return (
      <span aria-hidden className="flex h-4 w-[9px] shrink-0 flex-col overflow-hidden border border-current">
        {bands.map((band, index) => (
          <span
            key={`${band.kind}-${index}`}
            style={{ height: `${band.share}%` }}
            className={cn('block', band.kind === 'cam' && 'bg-current')}
          />
        ))}
      </span>
    );
  }

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
