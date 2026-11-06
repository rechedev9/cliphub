'use client';

import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/**
 * A row of mutually exclusive buttons. The active one is marked with the
 * accent edge, never filled with it.
 */
export function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: readonly { value: T; label: string; disabled?: boolean }[];
  onChange: (value: T) => void;
}): ReactNode {
  return (
    <div role="group" aria-label={label} className="inline-flex flex-wrap rounded-md border border-border-strong bg-surface-1 p-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={value === option.value}
          disabled={option.disabled}
          onClick={() => onChange(option.value)}
          className={cn(
            'h-6.5 rounded-sm px-2 text-body-sm whitespace-nowrap text-fg-2 outline-none transition-colors',
            'hover:text-fg-1 focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-45',
            value === option.value && 'bg-surface-3 font-medium text-fg-1 shadow-[inset_0_-2px_0_0_var(--primary)]',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
