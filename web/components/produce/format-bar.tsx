'use client';

import type { ReactNode } from 'react';
import { PRODUCE_FORMAT, type ProduceFormat } from '@/lib/clips/routes';
import { SelectableCard } from '@/components/studio/selectable-card';

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
    <div role="group" aria-label="Tipo de vídeo" className="grid gap-3 @[40rem]/content:grid-cols-2">
      {FORMAT_ITEMS.map((item) => (
        <SelectableCard key={item.value} selected={value === item.value} onSelect={() => onChange(item.value)}
          label={item.label} disabled={disabled} tilt={false} className="gap-1 p-4">
          <span className="font-display text-body-lg font-semibold text-fg-1">{item.label}</span>
          <span className="text-body-sm text-fg-2">{item.description}</span>
        </SelectableCard>
      ))}
    </div>
  );
}
