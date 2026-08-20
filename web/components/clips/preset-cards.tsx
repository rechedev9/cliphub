'use client';

import { Clapperboard, Eye, Crosshair, Check, Tv, type LucideIcon } from 'lucide-react';
import type { Preset } from '@/lib/api/types';
import { presetDescription } from '@/lib/preset-copy';
import { IconTile } from '@/components/studio/icon-tile';
import { SelectableCard } from '@/components/studio/selectable-card';
import { StatusTag } from '@/components/studio/status-tag';
import { cn } from '@/lib/utils';

export type PresetCardsProps = {
  presets: Preset[];
  /** Chosen preset name (== render variant), or null when none is picked. */
  value: string | null;
  onChange: (variant: string) => void;
  /** Disable interaction (e.g. no play selected, or a render is in flight). */
  disabled?: boolean;
};

const PRESET_ICONS: Record<string, LucideIcon> = {
  'clean-pov-60': Eye,
  'viral-60-clean': Crosshair,
  'full-hud-60': Clapperboard,
  'gameplay-pov-60': Tv,
};

/** Registry-driven reel style picker; auto-fit so the grid survives the narrow build column. */
export function PresetCards({ presets, value, onChange, disabled = false }: PresetCardsProps) {
  return (
    <div className="@container/presets">
      <div className="grid gap-3 [grid-template-columns:repeat(auto-fit,minmax(min(100%,15rem),1fr))]">
        {presets.map((preset) => (
          <PresetCard
            key={preset.name}
            icon={PRESET_ICONS[preset.name] ?? Clapperboard}
            title={preset.label}
            pitch={presetDescription(preset)}
            hud={preset.hudMode}
            isDefault={Boolean(preset.default)}
            selected={value === preset.name}
            disabled={disabled}
            onSelect={() => onChange(preset.name)}
          />
        ))}
      </div>
    </div>
  );
}

type PresetCardProps = {
  icon: LucideIcon;
  title: string;
  pitch: string;
  hud?: string;
  isDefault: boolean;
  selected: boolean;
  disabled: boolean;
  onSelect: () => void;
};

function PresetCard({ icon, title, pitch, hud, isDefault, selected, disabled, onSelect }: PresetCardProps) {
  return (
    <SelectableCard
      selected={selected}
      onSelect={onSelect}
      disabled={disabled}
      label={`Preset ${title}`}
      className="gap-3 p-4"
    >
      <div className="flex w-full items-start justify-between gap-3">
        <IconTile icon={icon} tone={selected ? 'primary' : 'neutral'} depth={selected ? 'raised' : 'inset'} />
        <span
          aria-hidden
          className={cn(
            'flex size-5 shrink-0 items-center justify-center border transition-colors duration-(--dur-fast) ease-standard',
            selected
              ? 'border-primary bg-primary text-primary-foreground'
              : 'border-border-strong bg-transparent text-transparent',
          )}
        >
          <Check className="size-3.5" />
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <span className="font-display text-title font-bold uppercase text-fg-1">{title}</span>
        {isDefault ? (
          <StatusTag tone="primary" size="sm">
            POR DEFECTO
          </StatusTag>
        ) : null}
      </div>

      <span className="text-body-sm text-fg-2">{pitch}</span>

      {hud ? (
        <StatusTag tone="neutral" size="sm" className="mt-auto">
          HUD · {hud}
        </StatusTag>
      ) : null}
    </SelectableCard>
  );
}
