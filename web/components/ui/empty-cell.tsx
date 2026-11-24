import type { ReactNode } from 'react';

export function EmptyCell({ label }: { label: string }): ReactNode {
  return (
    <span title={label}>
      <span aria-hidden className="text-fg-3">
        -
      </span>
      <span className="sr-only">{label}</span>
    </span>
  );
}
