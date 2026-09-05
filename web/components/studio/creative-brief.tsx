import type { ReactNode } from 'react';
import type { CreativeBriefItem } from '@/lib/reel-brief';
import { cn } from '@/lib/utils';

/** The exact decisions a render will honor, one `Label: value` per item. */
export function CreativeBriefList({ items, className }: { items: readonly CreativeBriefItem[]; className?: string }): ReactNode {
  return (
    <dl className={cn('grid gap-x-6 gap-y-1.5 text-body-sm', className)}>
      {items.map((item) => (
        <div key={item.label} className="flex min-w-0 flex-wrap gap-x-1.5">
          <dt className="shrink-0 text-fg-3">{item.label}:</dt>
          <dd className="min-w-0 break-words text-fg-1">
            {item.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}
