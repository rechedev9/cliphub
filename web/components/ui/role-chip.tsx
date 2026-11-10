'use client';

import type { ReactNode } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { ChartNoAxesColumn } from 'lucide-react';

import { FOCUS_RING } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

const roleChipVariants = cva(
  'inline-flex w-fit max-w-full shrink-0 items-center gap-1 rounded-sm border px-1.5 py-0.5 text-meta font-medium whitespace-nowrap',
  {
    variants: {
      side: {
        ct: 'border-side-ct/40 bg-side-ct/10 text-side-ct',
        t: 'border-side-t/40 bg-side-t/10 text-side-t',
        // Library roles (AWPer, Entry) belong to no side.
        neutral: 'border-border bg-surface-3 text-fg-2',
      },
    },
    defaultVariants: { side: 'neutral' },
  },
);

const SIDE_LETTER = { ct: 'CT', t: 'T', neutral: '' } as const;

export type RoleChipProps = {
  side?: NonNullable<VariantProps<typeof roleChipVariants>['side']>;
  /** The role, such as "Lurk de B". */
  label: string;
  /** Opens a popover with this text, such as "Lurk de B 62 % · Canal 21 %", from a small bars button. */
  breakdown?: string;
  className?: string;
};

/** A player's role on one side as a T (orange) or CT (blue) chip; `neutral` is for roles that belong to no side. */
export function RoleChip({ side = 'neutral', label, breakdown, className }: RoleChipProps): ReactNode {
  const letter = SIDE_LETTER[side];
  const classes = cn(roleChipVariants({ side }), className);
  const content = (
    <>
      {letter === '' ? null : <span className="font-semibold">{letter}</span>}
      <span className="truncate">{label}</span>
    </>
  );
  if (breakdown === undefined || breakdown === '') {
    return (
      <span data-slot="role-chip" data-side={side} className={classes}>
        {content}
      </span>
    );
  }
  const name = `Desglose de ${letter} ${label}`.replace('  ', ' ');
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          data-slot="role-chip"
          data-side={side}
          aria-label={name}
          title="Ver el desglose"
          className={cn(classes, 'min-h-6 cursor-pointer hover:brightness-110', FOCUS_RING)}
        >
          {content}
          <ChartNoAxesColumn aria-hidden className="size-3 opacity-80" />
        </button>
      </PopoverTrigger>
      <PopoverContent aria-label={name} className="w-auto max-w-xs p-2 text-body-sm">
        {breakdown}
      </PopoverContent>
    </Popover>
  );
}
