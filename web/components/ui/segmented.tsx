'use client';

import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/**
 * A row of mutually exclusive buttons. The active segment is tinted with the
 * accent, as in EDGE; the border stays grey.
 */
export function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: readonly { value: T; label: ReactNode; disabled?: boolean; /** Why it is disabled, shown on hover. */ reason?: string }[];
  onChange: (value: T) => void;
}): ReactNode {
  return (
    <div role="group" aria-label={label} className="inline-flex flex-wrap overflow-hidden rounded-md border border-border-strong bg-surface-1">
      {options.map((option) => (
        <span key={option.value} title={option.disabled ? option.reason : undefined} className="inline-flex border-l border-border-strong first:border-l-0">
          <button
            type="button"
            aria-pressed={value === option.value}
            title={option.disabled ? option.reason : undefined}
            disabled={option.disabled}
            onClick={() => onChange(option.value)}
            className={cn(
              'h-6.5 rounded-none px-2.5 text-body-sm whitespace-nowrap text-fg-2 outline-none transition-colors',
              'hover:bg-surface-3 hover:text-fg-1 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-45',
              value === option.value && 'bg-primary/16 font-semibold text-primary hover:bg-primary/16 hover:text-primary',
            )}
          >
            {option.label}
          </button>
        </span>
      ))}
    </div>
  );
}
