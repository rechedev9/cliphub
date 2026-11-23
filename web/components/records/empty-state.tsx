import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type EmptyStateProps = {
  icon: LucideIcon;
  title: string;
  description: ReactNode;
  preview?: readonly string[];
  actions?: ReactNode;
  note?: ReactNode;
  role?: 'status' | 'alert';
  className?: string;
};

export function EmptyState({ icon: Icon, title, description, preview, actions, note, role, className }: EmptyStateProps): ReactNode {
  return (
    <section
      aria-label={title}
      role={role}
      data-slot="records-empty"
      className={cn('flex flex-col gap-4 rounded-lg border border-dashed border-border-strong px-5 py-8 sm:px-8', className)}
    >
      <div className="flex items-start gap-4">
        <span className="grid size-9 shrink-0 place-items-center rounded-md border border-border bg-surface-3 text-fg-2">
          <Icon className="size-4" aria-hidden />
        </span>
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="text-body font-semibold text-fg-1">{title}</h2>
          <div className="measure-read text-body-sm text-fg-2">{description}</div>
        </div>
      </div>
      {preview && preview.length > 0 ? (
        <ul className="ml-13 flex flex-col gap-1.5 text-body-sm text-fg-2">
          {preview.map((item) => (
            <li key={item} className="flex gap-2">
              <span aria-hidden className="mt-2 size-1 shrink-0 rounded-full bg-fg-4" />
              {item}
            </li>
          ))}
        </ul>
      ) : null}
      {actions ? <div className="ml-13 flex flex-wrap gap-2">{actions}</div> : null}
      {note ? <div className="ml-13 text-meta text-fg-3">{note}</div> : null}
    </section>
  );
}
