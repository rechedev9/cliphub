import type { ReactNode } from 'react';

/** A table cell with no value: a quiet dash on screen, the reason for screen readers and on hover. */
export function EmptyCell({ label }: { label: string }): ReactNode {
  return (
    <span title={label}>
      <span aria-hidden className="text-fg-3">
        —
      </span>
      <span className="sr-only">{label}</span>
    </span>
  );
}
