'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { listLibraryTeams, type LibraryTeam } from '@/lib/api/library';
import { TACTICAL_SITE_ORDER, type TacticalSide, type TacticalSite } from '@/lib/api/tactical';
import { DEFAULT_MAP_POOL } from '@/lib/api/veto';
import { prettyMapName } from '@/lib/format';
import { playKindLabel, playKinds } from '@/lib/stratbook/plays';
import { siteLabel } from '@/lib/tactical-labels';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

/** Radix Select refuses an empty value, so "no team" travels as this sentinel. */
export const NO_TEAM = 'none';

/** The library's teams by name; an empty list while loading or offline. */
export function useLibraryTeams(): LibraryTeam[] {
  const [teams, setTeams] = useState<LibraryTeam[]>([]);
  useEffect(() => {
    let active = true;
    listLibraryTeams()
      .then((list) => {
        if (active) setTeams(list.sort((a, b) => a.name.localeCompare(b.name, 'es')));
      })
      .catch(() => {
        if (active) setTeams([]);
      });
    return () => {
      active = false;
    };
  }, []);
  return teams;
}

/** The active duty pool plus any map already in use, by name. */
export function mapChoices(extra: Iterable<string> = []): string[] {
  return [...new Set([...DEFAULT_MAP_POOL, ...extra])].sort((a, b) => prettyMapName(a).localeCompare(prettyMapName(b), 'es'));
}

export function TeamField({
  value,
  onChange,
  teams,
  currentName,
}: {
  value: string;
  onChange: (teamId: string) => void;
  teams: readonly LibraryTeam[];
  currentName?: string;
}): ReactNode {
  return (
    <Field label="Equipo">
      {(control) => (
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger {...control} className="h-9 w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={NO_TEAM}>Sin equipo</SelectItem>
            {teams.map((team) => (
              <SelectItem key={team.id} value={team.id}>
                {team.name}
              </SelectItem>
            ))}
            {value !== NO_TEAM && !teams.some((team) => team.id === value) ? (
              <SelectItem value={value}>{currentName ?? 'Equipo actual'}</SelectItem>
            ) : null}
          </SelectContent>
        </Select>
      )}
    </Field>
  );
}

export function MapField({ value, onChange, maps }: { value: string; onChange: (map: string) => void; maps: readonly string[] }): ReactNode {
  return (
    <Field label="Mapa">
      {(control) => (
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger {...control} className="h-9 w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {maps.map((map) => (
              <SelectItem key={map} value={map}>
                {prettyMapName(map)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </Field>
  );
}

export function SideField({ value, onChange }: { value: TacticalSide; onChange: (side: TacticalSide) => void }): ReactNode {
  return (
    <Field label="Lado">
      {(control) => (
        <Select value={value} onValueChange={(next) => (next === 'T' || next === 'CT') && onChange(next)}>
          <SelectTrigger {...control} className="h-9 w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="T">T</SelectItem>
            <SelectItem value="CT">CT</SelectItem>
          </SelectContent>
        </Select>
      )}
    </Field>
  );
}

export function KindField({ side, value, onChange }: { side: TacticalSide; value: string; onChange: (kind: string) => void }): ReactNode {
  return (
    <Field label="Tipo">
      {(control) => (
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger {...control} className="h-9 w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {playKinds(side).map((kind) => (
              <SelectItem key={kind} value={kind}>
                {playKindLabel(kind)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </Field>
  );
}

const isSite = (value: string): value is TacticalSite => TACTICAL_SITE_ORDER.some((entry) => entry === value);

export function SiteField({ value, onChange }: { value: TacticalSite; onChange: (site: TacticalSite) => void }): ReactNode {
  return (
    <Field label="Sitio">
      {(control) => (
        <Select value={value} onValueChange={(next) => isSite(next) && onChange(next)}>
          <SelectTrigger {...control} className="h-9 w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {TACTICAL_SITE_ORDER.map((site) => (
              <SelectItem key={site} value={site}>
                {siteLabel(site)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </Field>
  );
}

export function TagsField({ value, onChange }: { value: string; onChange: (text: string) => void }): ReactNode {
  return (
    <Field label="Etiquetas" hint="Separadas por comas.">
      {(control) => <Input {...control} value={value} onChange={(event) => onChange(event.target.value)} placeholder="eco, rápida, anti-eco" />}
    </Field>
  );
}

export function TitleField({ value, onChange }: { value: string; onChange: (title: string) => void }): ReactNode {
  return (
    <Field label="Título">
      {(control) => <Input {...control} value={value} maxLength={120} onChange={(event) => onChange(event.target.value)} />}
    </Field>
  );
}
