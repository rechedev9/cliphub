import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type RecordField = { readonly label: string; readonly value: ReactNode };

export type RecordHeaderProps = {
  /** The object type, such as "Partida" or "Equipo". */
  kind: string;
  title: string;
  icon?: LucideIcon;
  subtitle?: ReactNode;
  /** The key facts of the record, shown as one row under the title. */
  fields?: readonly RecordField[];
  actions?: ReactNode;
  className?: string;
};

/** Top of a record page or list: object type, name, actions and its key fields. */
export function RecordHeader({ kind, title, icon: Icon, subtitle, fields, actions, className }: RecordHeaderProps): ReactNode {
  return (
    <header className={cn('flex flex-col gap-4 border-b border-border pb-5', className)}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between sm:gap-6">
        <div className="flex min-w-0 items-start gap-3">
          {Icon ? (
            <span className="grid size-10 shrink-0 place-items-center rounded-md border border-border bg-surface-2 text-fg-2">
              <Icon className="size-5" aria-hidden />
            </span>
          ) : null}
          <div className="flex min-w-0 flex-col gap-0.5">
            <span className="text-meta font-medium text-fg-3">{kind}</span>
            <h1 className="truncate text-section font-semibold text-fg-1">{title}</h1>
            {subtitle ? <div className="text-body-sm text-fg-2">{subtitle}</div> : null}
          </div>
        </div>
        {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
      {fields && fields.length > 0 ? (
        <dl className="flex flex-wrap gap-x-8 gap-y-3">
          {fields.map((field) => (
            <div key={field.label} className="flex min-w-0 flex-col gap-0.5">
              <dt className="text-meta text-fg-3">{field.label}</dt>
              <dd className="text-body-sm font-medium text-fg-1">{field.value}</dd>
            </div>
          ))}
        </dl>
      ) : null}
    </header>
  );
}
