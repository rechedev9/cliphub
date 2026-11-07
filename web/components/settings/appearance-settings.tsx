'use client';

import type { ReactNode } from 'react';
import { useTheme } from 'next-themes';
import { DENSITY_OPTIONS, parseDensity } from '@/lib/density';
import { setDensity, useDensity } from '@/components/shell/density';
import { THEME_OPTIONS } from '@/components/shell/theme-menu';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { SettingsSection } from './settings-section';

export function AppearanceSettings(): ReactNode {
  const { theme, setTheme } = useTheme();
  const density = useDensity();
  // useDensity is null until hydration, the same moment next-themes knows the theme.
  const mounted = density !== null;
  const densityHint = DENSITY_OPTIONS.find((option) => option.value === density)?.hint;

  return (
    <SettingsSection id="appearance" title="Apariencia" description="El tema y el tamaño de la interfaz. Se guardan en este PC.">
      <div className="flex flex-col gap-2">
        <span id="appearance-theme" className="text-label font-medium text-fg-2">Tema</span>
        <ToggleGroup
          type="single"
          variant="filter"
          spacing={2}
          aria-labelledby="appearance-theme"
          value={mounted ? (theme ?? '') : ''}
          onValueChange={(next: string) => { if (next) setTheme(next); }}
          className="flex-wrap"
        >
          {THEME_OPTIONS.map((option) => (
            <ToggleGroupItem key={option.value} value={option.value}>{option.label}</ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>
      <div className="flex flex-col gap-2">
        <span id="appearance-density" className="text-label font-medium text-fg-2">Densidad</span>
        <ToggleGroup
          type="single"
          variant="filter"
          spacing={2}
          aria-labelledby="appearance-density"
          value={density ?? ''}
          onValueChange={(next: string) => { if (next) setDensity(parseDensity(next)); }}
          className="flex-wrap"
        >
          {DENSITY_OPTIONS.map((option) => (
            <ToggleGroupItem key={option.value} value={option.value}>{option.label}</ToggleGroupItem>
          ))}
        </ToggleGroup>
        <p className="text-body-sm text-fg-2">
          {densityHint ? `${densityHint} ` : null}Compacta reduce márgenes, alturas y texto a la vez, y rinde mejor en portátiles y pantallas 1080p.
        </p>
      </div>
    </SettingsSection>
  );
}
