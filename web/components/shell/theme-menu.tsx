'use client';

import { useSyncExternalStore, type ReactElement } from 'react';
import { Check, Monitor, Moon, Sun, SunMoon, type LucideIcon } from 'lucide-react';
import { useTheme } from 'next-themes';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const THEME_OPTIONS: ReadonlyArray<{ value: string; label: string; icon: LucideIcon }> = [
  { value: 'light', label: 'Claro', icon: Sun },
  { value: 'dark', label: 'Oscuro', icon: Moon },
  { value: 'system', label: 'Sistema', icon: Monitor },
];

const subscribeNever = (): (() => void) => () => undefined;

/** Light, dark or follow Windows. The choice is a per-PC preference kept by next-themes. */
export function ThemeMenu(): ReactElement {
  const { theme, resolvedTheme, setTheme } = useTheme();
  // The stored theme is unknown on the server; a neutral icon until hydration avoids a mismatch.
  const mounted = useSyncExternalStore(subscribeNever, () => true, () => false);
  let TriggerIcon: LucideIcon = SunMoon;
  if (mounted) TriggerIcon = resolvedTheme === 'light' ? Sun : Moon;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Tema de la interfaz" title="Tema">
          <TriggerIcon aria-hidden />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40">
        <DropdownMenuLabel>Tema</DropdownMenuLabel>
        {THEME_OPTIONS.map((option) => {
          const Icon = option.icon;
          const current = mounted && theme === option.value;
          return (
            <DropdownMenuItem key={option.value} onSelect={() => setTheme(option.value)}>
              <Icon aria-hidden />
              <span className="flex-1">{option.label}</span>
              {current ? <Check aria-label="Seleccionado" className="text-primary" /> : null}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
