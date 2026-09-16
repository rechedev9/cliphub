'use client';

import type { ReactNode } from 'react';
import { PRODUCE_FORMAT, type ProduceFormat } from '@/lib/clips/routes';
import { Button } from '@/components/ui/button';

const FORMAT_ITEMS = [
  { value: PRODUCE_FORMAT.short, label: 'Short 9:16', description: 'Jugadas seleccionadas · un vídeo vertical · estilo y música a tu elección' },
  { value: PRODUCE_FORMAT.full, label: 'Vídeo largo 16:9', description: 'Todas las rondas · vista de un jugador · HUD y voces del equipo' },
] as const;

export type ProduceFormatBarProps = {
  value: ProduceFormat;
  onChange: (format: ProduceFormat) => void;
  disabled?: boolean;
};

export function ProduceFormatBar({ value, onChange, disabled = false }: ProduceFormatBarProps): ReactNode {
  return (
    <div className="flex min-w-0 flex-col gap-2 @[40rem]/content:flex-row @[40rem]/content:items-center @[40rem]/content:gap-4">
      <div role="group" aria-label="Tipo de vídeo" className="grid shrink-0 grid-cols-2 gap-2">
        {FORMAT_ITEMS.map((item) => <Button key={item.value} size="sm" variant={value === item.value ? 'outline-primary' : 'outline'}
          aria-pressed={value === item.value} disabled={disabled} onClick={() => onChange(item.value)}>{item.label}</Button>)}
      </div>
      <p className="min-w-0 text-body-sm text-fg-2">{FORMAT_ITEMS.find((item) => item.value === value)?.description}</p>
    </div>
  );
}
