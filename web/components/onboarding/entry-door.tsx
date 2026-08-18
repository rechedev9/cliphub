import Link from 'next/link';
import { ChevronRight, type LucideIcon } from 'lucide-react';
import type { ReactElement } from 'react';
import { FOCUS_RING } from '@/components/ui/button';
import { IconTile, type IconTileTone } from '@/components/studio/icon-tile';
import { cn } from '@/lib/utils';

export type EntryDoorProps = {
  href: string;
  icon: LucideIcon;
  title: string;
  description: string;
  tone?: IconTileTone;
  /** The lit door. Exactly one per screen — it is the recommended way in. */
  emphasis?: 'primary' | 'default';
};

/** Whole-card entry link; the card is the control, not a button inside it. */
export function EntryDoor({
  href,
  icon,
  title,
  description,
  tone = 'neutral',
  emphasis = 'default',
}: EntryDoorProps): ReactElement {
  const primary = emphasis === 'primary';

  return (
    <Link
      href={href}
      className={cn(
        'studio-panel studio-panel-interactive group flex min-h-14 items-center gap-3 p-3',
        FOCUS_RING,
        // Raised + glow must share one box-shadow; they cannot stack as classes.
        primary &&
          'studio-panel-raised [box-shadow:var(--elev-3),var(--glow-primary-md)] hover:[box-shadow:var(--elev-3),var(--glow-primary-lg)]',
      )}
    >
      {/* inset: a raised tile on the lit panel would share --surface-3 and lose its edge */}
      <IconTile icon={icon} size="sm" tone={primary ? 'primary' : tone} depth="inset" />

      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="font-display text-body-sm font-semibold tracking-wide text-fg-1 uppercase">
          {title}
        </span>
        <span className="text-meta text-fg-2 normal-case tracking-normal">{description}</span>
      </span>

      <ChevronRight
        aria-hidden
        className={cn(
          'size-4 shrink-0 transition-transform duration-(--dur-fast) ease-standard group-hover:translate-x-0.5',
          primary ? 'text-primary' : 'text-fg-3',
        )}
      />
    </Link>
  );
}
