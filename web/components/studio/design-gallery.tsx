'use client';

import { Check } from 'lucide-react';
import { useId, useRef, useState, type FocusEvent, type KeyboardEvent, type ReactNode } from 'react';
import type { DesignGalleryItem } from '@/lib/design-gallery';
import { CoverImage } from '@/components/studio/cover-image';

/** `video` is a 16:9 frame; `band` is the 628x186 strip of the Short scoreboard. */
export type DesignGalleryShape = 'video' | 'band';

const SHAPE: Readonly<Record<DesignGalleryShape, { grid: string; thumb: string }>> = {
  video: { grid: 'grid-cols-[repeat(auto-fill,minmax(8rem,1fr))]', thumb: 'aspect-video' },
  band: { grid: 'grid-cols-[repeat(auto-fill,minmax(10rem,1fr))]', thumb: 'aspect-[628/186]' },
};
const TONE = {
  primary: { card: 'aria-checked:border-primary aria-checked:bg-primary/12', mark: 'bg-primary text-primary-foreground' },
  full: { card: 'aria-checked:border-format-full aria-checked:bg-format-full/12', mark: 'bg-format-full text-format-full-foreground' },
} as const;

/**
 * A single-choice gallery of design thumbnails. Arrows move the focus and Enter or Space
 * choose, so stepping through it never swaps the design on its own.
 */
export function DesignGallery({ label, items, shape, tone = 'primary', disabled = false, describedBy, backdrop, onSelect }: {
  label: string; items: readonly DesignGalleryItem[]; shape: DesignGalleryShape; tone?: keyof typeof TONE;
  disabled?: boolean; describedBy?: string; backdrop?: ReactNode; onSelect: (id: string) => void;
}): ReactNode {
  const labelId = useId();
  const buttons = useRef<(HTMLButtonElement | null)[]>([]);
  const [active, setActive] = useState<string | null>(null);
  const tabStop = items.some((item) => item.id === active) ? active : (items.find((item) => item.selected) ?? items[0])?.id;

  function move(event: KeyboardEvent<HTMLDivElement>): void {
    const index = buttons.current.findIndex((node) => node !== null && node === event.target);
    if (index < 0 || index >= items.length) return;
    const last = items.length - 1;
    let next: number | null = null;
    if (event.key === 'Home') next = 0;
    if (event.key === 'End') next = last;
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = index === last ? 0 : index + 1;
    if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = index === 0 ? last : index - 1;
    if (next === null) return;
    event.preventDefault();
    buttons.current[next]?.focus();
  }

  function leave(event: FocusEvent<HTMLDivElement>): void {
    const to = event.relatedTarget;
    if (!(to instanceof Node && event.currentTarget.contains(to))) setActive(null);
  }

  return <div className="grid min-w-0 gap-1.5">
    <p id={labelId} className="text-body-sm text-fg-2">{label}</p>
    <div role="radiogroup" aria-labelledby={labelId} aria-describedby={describedBy} aria-disabled={disabled || undefined}
      onKeyDown={move} onBlur={leave} className={`grid min-w-0 gap-2 ${SHAPE[shape].grid}`}>
      {items.map((item, index) => <button key={item.id} ref={(node) => { buttons.current[index] = node; }} type="button" role="radio"
        aria-checked={item.selected} tabIndex={item.id === tabStop ? 0 : -1} disabled={disabled}
        onFocus={() => setActive(item.id)} onClick={() => { if (!item.selected) onSelect(item.id); }}
        className={`flex min-w-0 cursor-pointer flex-col gap-1.5 rounded-lg border-[1.5px] border-border-subtle p-1.5 text-left transition-colors hover:border-border-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:border-border-subtle ${TONE[tone].card}`}>
        <span className={`relative isolate block w-full overflow-hidden rounded-md bg-surface-0 ${SHAPE[shape].thumb}`}>
          {backdrop}
          <CoverImage src={item.image} className="absolute inset-0 object-contain" />
          {item.selected ? <span className={`absolute top-1 right-1 grid size-5 place-items-center rounded-full ${TONE[tone].mark}`}><Check aria-hidden className="size-3.5" /></span> : null}
        </span>
        <span className="flex min-w-0 flex-col px-0.5">
          <span className="truncate text-body-sm font-semibold text-fg-1">{item.label}</span>
          {item.note ? <span className="truncate text-meta text-fg-3">{item.note}</span> : null}
        </span>
      </button>)}
    </div>
  </div>;
}
