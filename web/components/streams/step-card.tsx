import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/** A titled panel in the active-step column, with an optional control in the head. */
export function StreamStepCard({
  title,
  control,
  children,
  className,
}: {
  title: string;
  control?: ReactNode;
  children: ReactNode;
  className?: string;
}): ReactNode {
  return (
    <section className={cn('studio-panel flex flex-col gap-2.5 p-3.5', className)} aria-label={title}>
      <div className="flex min-h-5 items-center justify-between gap-3">
        <span className="font-mono text-meta uppercase tracking-widest text-fg-3">{title}</span>
        {control}
      </div>
      {children}
    </section>
  );
}

/** On/off pill: magenta track when on. The label is for assistive tech only. */
export function StreamSwitch({
  label,
  checked,
  disabled,
  onChange,
}: {
  label: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}): ReactNode {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        'relative h-5 w-[34px] shrink-0 rounded-full transition-colors duration-(--dur-fast) ease-standard focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50',
        checked ? 'bg-stream' : 'bg-border-strong',
      )}
    >
      <span
        aria-hidden
        className={cn(
          'absolute top-0.5 size-4 rounded-full bg-fg-1 transition-[left] duration-(--dur-fast) ease-standard',
          checked ? 'left-4' : 'left-0.5',
        )}
      />
    </button>
  );
}
