'use client';

import type { ReactNode } from 'react';
import { PRODUCE_FORMAT, type ProduceFormat } from '@/lib/clips/routes';
import { cn } from '@/lib/utils';

const FORMAT_ITEMS: ReadonlyArray<{ value: ProduceFormat; label: string }> = [
  { value: PRODUCE_FORMAT.short, label: 'Short 9:16' },
  { value: PRODUCE_FORMAT.full, label: 'Full POV 16:9' },
];

export type ProduceFormatBarProps = {
  value: ProduceFormat;
  onChange: (format: ProduceFormat) => void;
  disabled?: boolean;
};

/** 48px bar under the command strip: "Formato" + the Short / Full POV segmented control. */
export function ProduceFormatBar({ value, onChange, disabled = false }: ProduceFormatBarProps): ReactNode {
  return (
    <div className="-mx-(--shell-gutter) flex h-12 shrink-0 items-center justify-end gap-2.5 border-b border-border-subtle px-(--shell-gutter)">
      <span className="font-mono text-meta uppercase tracking-wider text-fg-3">Formato</span>
      <div role="group" aria-label="Formato" className="flex font-mono text-meta uppercase tracking-widest">
        {FORMAT_ITEMS.map((item) => {
          const active = item.value === value;
          return (
            <button
              key={item.value}
              type="button"
              aria-pressed={active}
              disabled={disabled}
              onClick={() => onChange(item.value)}
              className={cn(
                'inline-flex h-10 items-center px-3.5 transition-colors duration-(--dur-fast) ease-standard',
                'focus-visible:z-10 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring',
                'disabled:pointer-events-none disabled:opacity-50',
                active
                  ? 'bg-primary text-primary-foreground'
                  : 'border border-border-strong text-fg-2 hover:border-primary/55 hover:text-fg-1 [&+&]:border-l-0',
              )}
            >
              {item.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
