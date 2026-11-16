'use client';

import { Smartphone } from 'lucide-react';
import { useId, type ReactNode } from 'react';
import type { EditConfig } from '@/lib/api/types';
import { CUSTOM_HUD_THEMES, verticalHudTheme } from '@/lib/custom-hud';
import { verticalHudPreviewUrl } from '@/lib/hud-themes';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

export type VerticalHudOptionsProps = {
  value: EditConfig;
  onChange: (next: EditConfig) => void;
  disabled: boolean;
  /** False when the chosen style records CS2's own HUD (Full HUD). */
  allowed: boolean;
};

/** The 9:16 scoreboard of a Short: the switch, its catalog design and the band the video draws. */
export function VerticalHudOptions({ value, onChange, disabled, allowed }: VerticalHudOptionsProps): ReactNode {
  const designId = useId();
  const on = allowed && value.verticalHud === true;
  const theme = verticalHudTheme(value.verticalHudTheme);

  function toggle(items: string[]): void {
    const next: EditConfig = { ...value, verticalHud: items.includes('vertical-hud') };
    if (!next.verticalHud) delete next.verticalHudTheme;
    onChange(next);
  }

  function chooseDesign(id: string): void {
    const picked = CUSTOM_HUD_THEMES.find((entry) => entry.id === id);
    if (picked) onChange({ ...value, verticalHudTheme: picked.id });
  }

  return (
    <>
      <ToggleGroup
        type="multiple"
        value={on ? ['vertical-hud'] : []}
        onValueChange={toggle}
        disabled={disabled || !allowed}
        variant="outline"
        className="flex-wrap"
      >
        <ToggleGroupItem value="vertical-hud" aria-label="HUD vertical">
          <Smartphone className="size-4" />
          HUD vertical
        </ToggleGroupItem>
      </ToggleGroup>
      {allowed ? null : (
        <p className="text-body-sm text-fg-3">
          Full HUD ya graba el HUD de CS2 en el vídeo. Elige otro estilo para usar el HUD vertical.
        </p>
      )}
      {on ? (
        <div className="grid min-w-0 gap-2">
          <label htmlFor={designId} className="text-body-sm text-fg-2">Diseño</label>
          <Select value={theme.id} onValueChange={chooseDesign} disabled={disabled}>
            <SelectTrigger id={designId} className="h-10 w-full max-w-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {CUSTOM_HUD_THEMES.map((entry) => (
                <SelectItem key={entry.id} value={entry.id}>{entry.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <img
            src={verticalHudPreviewUrl(theme.id)}
            alt={`Vista previa del HUD vertical ${theme.name}`}
            width={628}
            height={186}
            className="h-auto w-full max-w-md rounded-md bg-surface-0"
          />
          <p className="text-body-sm text-fg-3">
            Arriba del Short, el marcador de la ronda con el reloj y los vivos de cada bando, leídos de la demo, y debajo tu kill feed centrada. Se oculta durante el título, la repetición desde la víctima y la tarjeta final.
          </p>
        </div>
      ) : null}
    </>
  );
}
