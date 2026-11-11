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

/** Top of a record page in one flat row, EDGE style: name, key facts as tiny caps labels over values, actions at the right. */
export function RecordHeader({ kind, title, icon: Icon, subtitle, fields, actions, className }: RecordHeaderProps): ReactNode {
  return (
    <header className={cn('flex flex-wrap items-center gap-x-6 gap-y-2 pb-1', className)}>
      <div className="flex min-w-0 items-center gap-2.5">
        {Icon ? <Icon className="size-4.5 shrink-0 text-fg-3" aria-hidden /> : null}
        <div className="flex min-w-0 flex-col">
          <span className="sr-only">{kind}: </span>
          <h1 className="truncate text-title leading-tight font-bold text-fg-1">{title}</h1>
          {subtitle ? <div className="truncate text-meta text-fg-3">{subtitle}</div> : null}
        </div>
      </div>
      {fields && fields.length > 0 ? (
        <dl className="flex flex-wrap items-center gap-x-5 gap-y-1">
          {fields.map((field) => (
            <div key={field.label} className="flex min-w-0 flex-col">
              <dt className="text-meta leading-tight font-medium tracking-wider whitespace-nowrap text-fg-3 uppercase">{field.label}</dt>
              <dd className="text-body-sm leading-tight font-medium whitespace-nowrap text-fg-1">{field.value}</dd>
            </div>
          ))}
        </dl>
      ) : null}
      {actions ? <div className="ml-auto flex shrink-0 flex-wrap items-center gap-1">{actions}</div> : null}
    </header>
  );
}
