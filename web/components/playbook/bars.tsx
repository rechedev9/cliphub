import type { ReactNode } from 'react';
import Link from 'next/link';
import type { EquipmentIconName } from '@/lib/api/game-assets';
import { scaleBars } from '@/lib/analysis/playbook';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { cn } from '@/lib/utils';

export type BarItem = {
  readonly id: string;
  readonly label: string;
  /** What the bar length reads. */
  readonly value: number;
  /** The number printed beside the bar, such as "42 % · 18". */
  readonly text: string;
  /** Secondary fact under the number, such as the win rate. */
  readonly detail?: string;
  /** Opens the rounds behind the bar; absent when there are none. */
  readonly href?: string;
  /** A weapon or grenade silhouette before the label, such as the buy's gun. */
  readonly icon?: EquipmentIconName | null;
};

/** Horizontal bars, each scaled to the longest, label on the left and its numbers on the right. */
export function BarList({ label, items }: { label: string; items: readonly BarItem[] }): ReactNode {
  const widths = scaleBars(items.map((item) => item.value));
  return (
    <ul aria-label={label} className="flex flex-col gap-0.5">
      {items.map((item, index) => {
        const body = (
          <>
            <span className="flex w-28 shrink-0 items-center gap-1.5 truncate text-body-sm text-fg-1">
              {item.icon ? <EquipmentIcon name={item.icon} height={11} className="text-fg-2" /> : null}
              {item.label}
            </span>
            <span aria-hidden className="h-2 min-w-0 flex-1 rounded-sm bg-surface-3">
              <span className="block h-full rounded-sm bg-primary/70" style={{ width: `${widths[index] ?? 0}%` }} />
            </span>
            <span className="flex w-40 shrink-0 flex-col items-end text-right leading-tight">
              <span className="text-body-sm text-fg-1 tabular-nums">{item.text}</span>
              {item.detail ? <span className="text-meta text-fg-3 tabular-nums">{item.detail}</span> : null}
            </span>
          </>
        );
        return (
          <li key={item.id}>
            {item.href ? (
              <Link
                href={item.href}
                className={cn(
                  'flex min-h-8 items-center gap-3 rounded-sm px-2 outline-none transition-[background-color]',
                  'hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50',
                )}
              >
                {body}
              </Link>
            ) : (
              <div className="flex min-h-8 items-center gap-3 px-2 text-fg-3">{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}

export type ColumnItem = {
  readonly id: string;
  readonly label: string;
  readonly value: number;
  readonly href?: string;
  readonly title: string;
  /** Printed on top of the bar, such as its share. */
  readonly top?: string;
};

/** Vertical bars over a time axis, each scaled to the tallest; a bar with rounds opens them. */
export function ColumnChart({ label, items, axisEnd }: { label: string; items: readonly ColumnItem[]; axisEnd: string }): ReactNode {
  const heights = scaleBars(items.map((item) => item.value));
  return (
    <figure aria-label={label} className="flex flex-col gap-1">
      <div className="flex h-28 items-end gap-1 pt-4">
        {items.map((item, index) => {
          const bar = (
            <span className="flex w-full flex-col items-center justify-end gap-0.5" style={{ height: `${heights[index] ?? 0}%` }}>
              {item.top ? <span className="-mt-4 text-meta leading-none whitespace-nowrap text-fg-2 tabular-nums">{item.top}</span> : null}
              <span className="block w-full flex-1 rounded-t-sm bg-primary/70" />
            </span>
          );
          return item.href ? (
            <Link
              key={item.id}
              href={item.href}
              title={item.title}
              aria-label={item.title}
              className="flex h-full min-w-0 flex-1 items-end rounded-sm outline-none hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50"
            >
              {bar}
            </Link>
          ) : (
            <span key={item.id} title={item.title} className="flex h-full min-w-0 flex-1 items-end">
              {bar}
            </span>
          );
        })}
      </div>
      <figcaption className="flex justify-between border-t border-border pt-1 text-meta text-fg-3 tabular-nums">
        <span>0 s</span>
        <span>{axisEnd}</span>
      </figcaption>
    </figure>
  );
}
