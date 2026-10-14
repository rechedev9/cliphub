import Link from 'next/link';
import { ChevronLeft } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

const BACK_LINK_CLASS =
  'inline-flex min-h-10 w-fit items-center gap-1.5 pr-2 font-mono text-meta uppercase text-fg-3 transition-colors duration-(--dur-fast) ease-standard hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring';

type StudioBackLinkBase = {
  children: ReactNode;
  className?: string;
};

/** Exactly one of `href` (a real navigation) or `onClick` (an in-page return). */
export type StudioBackLinkProps =
  | (StudioBackLinkBase & { href: string; onClick?: undefined })
  | (StudioBackLinkBase & { onClick: () => void; href?: undefined });

export function StudioBackLink(props: StudioBackLinkProps): ReactNode {
  const content = (
    <>
      <ChevronLeft aria-hidden className="size-4 shrink-0" />
      {props.children}
    </>
  );

  const className = cn(BACK_LINK_CLASS, props.className);

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
