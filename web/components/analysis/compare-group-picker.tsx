'use client';

import type { ReactNode } from 'react';
import { Check, Scale } from 'lucide-react';
import { toggleCompareTeam } from '@/lib/analysis/side-stats';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import type { ScopeOption } from './scope-bar';

export type CompareGroupPickerProps = {
  /** Team ids picked; empty compares with every other team of the library. */
  value: readonly string[];
  teams: readonly ScopeOption[];
  onChange: (ids: string[]) => void;
};

function pickedLabel(value: readonly string[], teams: readonly ScopeOption[]): string {
  if (value.length === 0) return 'Toda la biblioteca';
  if (value.length === 1) return teams.find((team) => team.value === value[0])?.label ?? '1 equipo';
  return `${value.length} equipos`;
}

/** "Comparar con": the whole library, or the teams checked by hand, pro teams included once they are in it. */
export function CompareGroupPicker({ value, teams, onChange }: CompareGroupPickerProps): ReactNode {
  const label = pickedLabel(value, teams);
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-meta whitespace-nowrap text-fg-3">Comparar con</span>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="sm"
            aria-label={`Comparar con: ${label}`}
            className={cn('h-8 max-w-56 gap-1.5 px-2.5 text-body-sm', value.length > 0 && 'border-fg-3 text-fg-1')}
          >
            <Scale aria-hidden />
            <span className="truncate">{label}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="max-h-96 w-72 overflow-y-auto">
          <DropdownMenuItem onSelect={() => onChange([])}>
            <span className="flex-1">Toda la biblioteca</span>
            {value.length === 0 ? <Check aria-label="Elegida" className="text-fg-1" /> : null}
          </DropdownMenuItem>
          {teams.length > 0 ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuLabel>Equipos de la biblioteca</DropdownMenuLabel>
              {teams.map((team) => (
                <DropdownMenuCheckboxItem
                  key={team.value}
                  checked={value.includes(team.value)}
                  onSelect={(event) => {
                    // Keep the menu open so several teams can be checked in a row.
                    event.preventDefault();
                  }}
                  onCheckedChange={() => onChange(toggleCompareTeam(value, team.value))}
                >
                  <span className="min-w-0 flex-1 truncate">{team.label}</span>
                </DropdownMenuCheckboxItem>
              ))}
            </>
          ) : null}
          <DropdownMenuSeparator />
          <p className="px-2 py-1.5 text-meta text-fg-3">
            La media cuenta a cada equipo por igual, con el mismo mapa, lado, fechas y compras. Los equipos pro que sigues entran cuando sus partidas están en la biblioteca.
          </p>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
