'use client';

import type { ReactNode } from 'react';
import { PRODUCE_FORMAT, type ProduceFormat } from '@/lib/clips/routes';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';

const FORMAT_ITEMS = [
  { value: PRODUCE_FORMAT.full, label: 'Vídeo largo 16:9', description: 'Todas las rondas · vista de un jugador · HUD y voces del equipo' },
  { value: PRODUCE_FORMAT.short, label: 'Short 9:16', description: 'Jugadas seleccionadas · un vídeo vertical · estilo y música a tu elección' },
] as const;

/** Selected: primary fill and border. Unselected: quiet until hovered, so the choice never reads inverted. */
const ITEM_CLASS = {
  selected: 'border-primary bg-primary/15 text-primary hover:bg-primary/20 hover:text-primary',
  idle: 'border-border-subtle bg-transparent font-medium text-fg-3 hover:border-border-strong hover:bg-surface-3 hover:text-fg-1',
} as const;

export type ProduceFormatBarProps = {
  value: ProduceFormat;
  onChange: (format: ProduceFormat) => void;
  disabled?: boolean;
};

export function ProduceFormatBar({ value, onChange, disabled = false }: ProduceFormatBarProps): ReactNode {
  return (
    <div className="flex min-w-0 flex-col gap-2 @[40rem]/content:flex-row @[40rem]/content:items-center @[40rem]/content:gap-4">
      <div role="group" aria-label="Tipo de vídeo" className="grid shrink-0 grid-cols-2 gap-2">
        {FORMAT_ITEMS.map((item) => {
          const selected = value === item.value;
          return <Button key={item.value} size="sm" variant="ghost" aria-pressed={selected} disabled={disabled}
            className={cn('border', selected ? ITEM_CLASS.selected : ITEM_CLASS.idle)} onClick={() => onChange(item.value)}>{item.label}</Button>;
        })}
      </div>
      <p className="min-w-0 text-body-sm text-fg-2">{FORMAT_ITEMS.find((item) => item.value === value)?.description}</p>
    </div>
  );
}
