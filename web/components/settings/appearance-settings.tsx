'use client';

import type { ReactNode } from 'react';
import { Rows3 } from 'lucide-react';
import { useTheme } from 'next-themes';
import { DENSITY_OPTIONS, parseDensity } from '@/lib/density';
import { setDensity, useDensity } from '@/components/shell/density';
import { THEME_OPTIONS } from '@/components/shell/theme-menu';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { IconTile } from '@/components/studio/icon-tile';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

export function AppearanceSettings(): ReactNode {
  const { theme, setTheme } = useTheme();
  const density = useDensity();
  // useDensity is null until hydration, the same moment next-themes knows the theme.
  const mounted = density !== null;
  const densityHint = DENSITY_OPTIONS.find((option) => option.value === density)?.hint;

  return (
    <section className="studio-panel flex flex-col gap-5 p-4 @[34rem]/content:p-6" aria-labelledby="appearance-settings-title">
      <div className="flex items-center gap-4">
        <IconTile icon={Rows3} size="md" depth="inset" />
        <div className="flex min-w-0 flex-col gap-1">
          <SectionEyebrow label="Interfaz" />
          <h2 id="appearance-settings-title" className="font-display text-title font-bold text-fg-1">
            Apariencia
          </h2>
        </div>
      </div>
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
    </section>
  );
}
