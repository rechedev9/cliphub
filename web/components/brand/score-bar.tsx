import { cn } from '@/lib/utils';

export type ScoreBarProps = {
  /** true = win (cyan accent + glow), false = loss (muted zinc). */
  win: boolean;
  className?: string;
};

/**
 * The lit faces. The bar is a rod edge-lit from the left (--light-angle), so it
 * runs hot → core → shade across its width and reads as a light source rather
 * than a filled rectangle. The rim is a gradient and not a `box-shadow` inset
 * on purpose: it would have to share the property with --glow-primary-md, and
 * that token collapses to `none` under the efficiency profile — `none` inside a
 * comma list invalidates the whole declaration.
 */
const WIN_FACE =
  'linear-gradient(90deg, color-mix(in oklch, var(--primary) 62%, oklch(1 0 0)) 0%, var(--primary) 46%, color-mix(in oklch, var(--primary) 58%, var(--surface-0)) 100%)';
const LOSS_FACE =
  'linear-gradient(90deg, var(--fg-3) 0%, var(--fg-4) 46%, color-mix(in oklch, var(--fg-4) 55%, var(--surface-0)) 100%)';

/**
 * ScoreBar — a thin vertical light source for match rows. Cyan and glowing =
 * win, dim zinc = loss, for a fast win/loss scan down a list. Per the design
 * language, loss is muted zinc, never red/magenta. The glow rides `.neon-glow`
 * so the efficiency profile and forced colours null it through the foundation
 * instead of through a second copy of the rule here; the lit face sits on its
 * own layer so forced colours can drop it without touching the layout.
 */
export function ScoreBar({ win, className }: ScoreBarProps) {
  return (
    <span aria-hidden className={cn('relative block w-1 self-stretch', win && 'neon-glow', className)}>
      <span
        className="absolute inset-0 forced-colors:hidden"
        style={{ backgroundImage: win ? WIN_FACE : LOSS_FACE }}
      />
    </span>
  );
}
