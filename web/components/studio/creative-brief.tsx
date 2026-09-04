import type { ReactNode } from 'react';
import { BRIEF_APPROVAL_LABEL, type CreativeBriefItem } from '@/lib/reel-brief';
import { cn } from '@/lib/utils';

/** The exact decisions a render will honor, one `Label: value` per item. */
export function CreativeBriefList({ items, className }: { items: readonly CreativeBriefItem[]; className?: string }): ReactNode {
  return (
    <dl className={cn('grid gap-x-6 gap-y-1.5 text-body-sm', className)}>
      {items.map((item) => (
        <div key={item.label} className="flex min-w-0 gap-1.5">
          <dt className="shrink-0 text-fg-3">{item.label}:</dt>
          <dd className="truncate text-fg-1" title={item.value}>
            {item.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

const ACCENT_CLASS = {
  primary: 'accent-primary',
  stream: 'accent-success',
} as const;

/** The one checkbox that approves a brief; the caller resets it on any plan change. */
export function BriefApprovalCheckbox({
  checked,
  disabled,
  accent = 'primary',
  className,
  onChange,
}: {
  checked: boolean;
  disabled: boolean;
  accent?: keyof typeof ACCENT_CLASS;
  className?: string;
  onChange: (checked: boolean) => void;
}): ReactNode {
  return (
    <label className={cn('flex min-h-10 items-center gap-2.5 text-body-sm text-fg-1', disabled && 'opacity-50', className)}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className={cn('size-5 shrink-0 cursor-pointer disabled:cursor-not-allowed', ACCENT_CLASS[accent])}
      />
      {BRIEF_APPROVAL_LABEL}
    </label>
  );
}
