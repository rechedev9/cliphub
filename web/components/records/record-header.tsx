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
  /** The key facts of the record, beside the title. */
  fields?: readonly RecordField[];
  actions?: ReactNode;
  className?: string;
};

/** Top of a record page or list in one row, Skybox style: name, key fields beside it, actions at the right. */
export function RecordHeader({ kind, title, icon: Icon, subtitle, fields, actions, className }: RecordHeaderProps): ReactNode {
  return (
    <header className={cn('flex flex-wrap items-center gap-x-6 gap-y-2 border-b border-border pb-2', className)}>
      <div className="flex min-w-0 items-center gap-2.5">
        {Icon ? (
          <span className="grid size-8 shrink-0 place-items-center rounded-md border border-border bg-surface-2 text-fg-2">
            <Icon className="size-4.5" aria-hidden />
          </span>
        ) : null}
        <div className="flex min-w-0 flex-col">
          <span className="sr-only">{kind}: </span>
          <h1 className="truncate text-title leading-tight font-semibold text-fg-1">{title}</h1>
          {subtitle ? <div className="truncate text-meta text-fg-3">{subtitle}</div> : null}
        </div>
      </div>
      {fields && fields.length > 0 ? (
        <dl className="flex flex-wrap items-center gap-x-5 gap-y-1">
          {fields.map((field) => (
            <div key={field.label} className="flex min-w-0 flex-col">
              <dt className="text-meta leading-tight whitespace-nowrap text-fg-3">{field.label}</dt>
              <dd className="text-body-sm leading-tight font-medium whitespace-nowrap text-fg-1">{field.value}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      {actions ? <div className="ml-auto flex shrink-0 flex-wrap items-center gap-1">{actions}</div> : null}
    </header>
  );
}
