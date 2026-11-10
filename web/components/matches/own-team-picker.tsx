'use client';

import { useId, type ReactNode } from 'react';
import type { LibraryTeam } from '@/lib/api/library';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

// Radix refuses an empty item value, so "no team" travels as this sentinel.
const NONE = '__none__';

/** Mi equipo: the team whose results colour the edge of every row. Picked once, kept by the library. */
export function OwnTeamPicker({
  teams,
  value,
  disabled,
  onChange,
}: {
  teams: readonly LibraryTeam[];
  value: string | null;
  disabled: boolean;
  onChange: (teamId: string | null) => void;
}): ReactNode {
  const labelId = useId();
  if (teams.length === 0) return null;
  return (
    <span className="flex items-center gap-2">
      <span id={labelId} className="text-body-sm text-fg-2">Mi equipo</span>
      <Select value={value ?? NONE} disabled={disabled} onValueChange={(next) => onChange(next === NONE ? null : next)}>
        <SelectTrigger aria-labelledby={labelId} className="h-9 w-48">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={NONE}>Sin elegir</SelectItem>
          {teams.map((team) => (
            <SelectItem key={team.id} value={team.id}>
              {team.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </span>
  );
}
