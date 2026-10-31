'use client';

import { useSyncExternalStore, type ReactElement } from 'react';
import { Check, Monitor, Moon, Rows3, Rows4, Sun, SunMoon, type LucideIcon } from 'lucide-react';
import { useTheme } from 'next-themes';
import { DENSITY_OPTIONS, type Density } from '@/lib/density';
import { setDensity, useDensity } from '@/components/shell/density';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

export const THEME_OPTIONS: ReadonlyArray<{ value: string; label: string; icon: LucideIcon }> = [
  { value: 'light', label: 'Claro', icon: Sun },
  { value: 'dark', label: 'Oscuro', icon: Moon },
  { value: 'system', label: 'Sistema', icon: Monitor },
];

const DENSITY_ICON: Record<Density, LucideIcon> = { comfortable: Rows3, compact: Rows4 };

const subscribeNever = (): (() => void) => () => undefined;

/** Theme and density, both per-PC preferences: next-themes keeps the theme, localStorage the density. */
export function ThemeMenu(): ReactElement {
  const { theme, resolvedTheme, setTheme } = useTheme();
  const density = useDensity();
  // The stored theme is unknown on the server; a neutral icon until hydration avoids a mismatch.
  const mounted = useSyncExternalStore(subscribeNever, () => true, () => false);
  let TriggerIcon: LucideIcon = SunMoon;
  if (mounted) TriggerIcon = resolvedTheme === 'light' ? Sun : Moon;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Apariencia" title="Apariencia">
          <TriggerIcon aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-44">
        <DropdownMenuLabel>Tema</DropdownMenuLabel>
        {THEME_OPTIONS.map((option) => {
          const Icon = option.icon;
          const current = mounted && theme === option.value;
          return (
            <DropdownMenuItem key={option.value} onSelect={() => setTheme(option.value)}>
              <Icon aria-hidden />
              <span className="flex-1">{option.label}</span>
              {current ? <Check aria-label="Seleccionado" className="text-fg-1" /> : null}
            </DropdownMenuItem>
          );
        })}
        <DropdownMenuSeparator />
        <DropdownMenuLabel>Densidad</DropdownMenuLabel>
        {DENSITY_OPTIONS.map((option) => {
          const Icon = DENSITY_ICON[option.value];
          return (
            <DropdownMenuItem key={option.value} onSelect={() => setDensity(option.value)}>
              <Icon aria-hidden />
              <span className="flex-1">{option.label}</span>
              {density === option.value ? <Check aria-label="Seleccionado" className="text-fg-1" /> : null}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
