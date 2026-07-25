import Link from 'next/link';
import { ChevronLeft } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/**
 * 40px tall with a real focus ring. The back affordance this replaces was an
 * 11px bare `<button>` labelled with a `◂` text glyph — under the target-size
 * minimum, with no focus indicator, and rendered differently per font fallback.
 */
const BACK_LINK_CLASS =
  'inline-flex min-h-10 w-fit items-center gap-1.5 pr-2 font-mono uppercase text-fg-3 transition-colors duration-(--dur-fast) ease-standard hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring';

/*
 * Concatenated rather than merged: tailwind-merge classifies a custom `--text-*`
 * theme key as a text COLOUR, so putting the step inside cn() next to `text-fg-3`
 * deletes the size.
 */
const BACK_LINK_TYPE_CLASS = 'text-meta';

type StudioBackLinkBase = {
  children: ReactNode;
  className?: string;
};

/** Exactly one of `href` (a real navigation) or `onClick` (an in-page return). */
export type StudioBackLinkProps =
  | (StudioBackLinkBase & { href: string; onClick?: undefined })
  | (StudioBackLinkBase & { onClick: () => void; href?: undefined });

/** Mono eyebrow back link for detail routes. */
export function StudioBackLink(props: StudioBackLinkProps): ReactNode {
  const content = (
    <>
      <ChevronLeft aria-hidden className="size-4 shrink-0" />
      {props.children}
    </>
  );

  const className = `${BACK_LINK_TYPE_CLASS} ${cn(BACK_LINK_CLASS, props.className)}`;

  if (props.href !== undefined) {
    return (
      <Link href={props.href} className={className}>
        {content}
      </Link>
    );
  }

  return (
    <button type="button" onClick={props.onClick} className={className}>
      {content}
    </button>
  );
}
