import { Crosshair } from 'lucide-react';
import { cn } from '@/lib/utils';

export type ReelCoverProps = {
  /** Stable key (id, map, title) — derives the cover's hue and light position. */
  seed: string;
  /** Optional faint map/label watermark in the corner. */
  label?: string;
  /** Drop the crosshair + label (when the parent overlays its own icon). */
  plain?: boolean;
  className?: string;
};

const HORIZON = '50%';

function hashSeed(seed: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < seed.length; i += 1) {
    h = Math.imul(h ^ seed.charCodeAt(i), 0x01000193);
  }
  h ^= h >>> 16;
  h = Math.imul(h, 0x85ebca6b);
  h ^= h >>> 13;
  h = Math.imul(h, 0xc2b2ae35);
  h ^= h >>> 16;
  return h >>> 0;
}

export function ReelCover({ seed, label, plain = false, className }: ReelCoverProps) {
  const hash = hashSeed(seed);
  const hue = 190 + (hash % 141);
  const lightX = 26 + ((hash >>> 8) % 49);

  const glow = `hsl(${hue} 92% 62% / 0.3)`;
  const wash = `hsl(${(hue + 32) % 360} 74% 30% / 0.44)`;
  const grid = `hsl(${hue} 80% 62% / 0.38)`;
  const rim = `hsl(${hue} 82% 58% / 0.45)`;
  const hot = `hsl(${hue} 96% 74%)`;

  return (
    <div aria-hidden className={cn('relative size-full overflow-hidden bg-surface-1', className)}>
      <div
        className="pointer-events-none absolute inset-0 forced-colors:hidden"
        style={{
          backgroundImage: [
            `radial-gradient(78% 58% at ${lightX}% ${HORIZON}, ${glow} 0%, transparent 64%)`,
            `radial-gradient(120% 86% at ${100 - lightX}% 2%, ${wash} 0%, transparent 68%)`,
            'radial-gradient(122% 96% at 50% 38%, transparent 40%, var(--surface-0) 100%)',
          ].join(', '),
        }}
      />

      <div
        className="pointer-events-none absolute inset-x-[-30%] bottom-0 origin-bottom forced-colors:hidden"
        style={{
          top: HORIZON,
          transform: 'perspective(300px) rotateX(calc(54deg * var(--shell-depth)))',
          backgroundImage: [
            `repeating-linear-gradient(0deg, ${grid} 0 1px, transparent 1px 20px)`,
            `repeating-linear-gradient(90deg, ${grid} 0 1px, transparent 1px 20px)`,
          ].join(', '),
          WebkitMaskImage: 'linear-gradient(to top, black 0%, transparent 88%)',
          maskImage: 'linear-gradient(to top, black 0%, transparent 88%)',
        }}
      />

      <div
        className="pointer-events-none absolute inset-x-0 h-px forced-colors:hidden"
        style={{
          top: HORIZON,
          backgroundImage: `linear-gradient(90deg, transparent, ${rim} 18%, ${hot} ${lightX}%, ${rim} 82%, transparent)`,
        }}
      />

      <div
        className="pointer-events-none absolute inset-0 forced-colors:hidden"
        style={{
          backgroundImage: 'repeating-linear-gradient(0deg, oklch(1 0 0 / 0.045) 0 1px, transparent 1px 3px)',
        }}
      />

      {!plain ? (
        <>
          <div className="pointer-events-none absolute inset-0 grid place-items-center">
            <Crosshair className="size-10 text-fg-4 forced-colors:hidden" strokeWidth={1} />
          </div>
          {label ? (
            <>
              <div className="pointer-events-none absolute inset-x-0 bottom-0 h-10 bg-[linear-gradient(to_top,var(--surface-0),transparent)] forced-colors:hidden" />
              <span className="pointer-events-none absolute inset-x-2.5 bottom-2.5 flex items-center gap-1.5">
                <span
                  className="h-3 w-0.5 shrink-0 forced-colors:hidden"
                  style={{ backgroundColor: hot }}
                />
                <span className="min-w-0 truncate font-mono text-meta uppercase text-fg-2">
                  {label}
                </span>
              </span>
            </>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
