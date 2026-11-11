import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/** The EDGE document column: about 950px, centred, with empty margins either side. */
export const DOCUMENT_COLUMN = 'mx-auto flex w-full max-w-[59.375rem] flex-col gap-4';

/** A section title in the EDGE form: a coloured slash, then the title in bold. */
export function SlashTitle({ children, as: Tag = 'h2', className }: { children: ReactNode; as?: 'h1' | 'h2' | 'h3'; className?: string }): ReactNode {
  return (
    <Tag className={cn('flex items-baseline gap-2 text-title font-semibold text-fg-1', className)}>
      <span aria-hidden className="text-slash">
        /
      </span>
      {children}
    </Tag>
  );
}

/** A document page: slash title, one line under it and the page actions at the right. */
export function DocumentPage({
  title,
  description,
  actions,
  children,
  className,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}): ReactNode {
  return (
    <div className={cn(DOCUMENT_COLUMN, className)}>
      <header className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2">
        <div className="flex min-w-0 flex-col gap-1.5">
          <SlashTitle as="h1">{title}</SlashTitle>
          {description ? <div className="text-body-sm text-fg-2">{description}</div> : null}
        </div>
        {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
      </header>
      {children}
    </div>
  );
}
