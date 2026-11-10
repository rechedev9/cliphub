'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { GitMerge } from 'lucide-react';
import { listLibraryTeams, mergeLibraryTeam, patchLibraryTeam, setTeamProfile, type LibraryTeam } from '@/lib/api/library';
import type { TeamProfile } from '@/lib/api/team-insights';
import { teamHref } from '@/lib/clips/routes';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';

/** The server's highest HLTV rank (`library.MaxHLTVRank`). */
const HLTV_RANK_MAX = 1000;

function rankText(profile: TeamProfile | null): string {
  return profile !== null && profile.hltv_rank > 0 ? String(profile.hltv_rank) : '';
}

export function EditTeamDialog({
  open,
  onOpenChange,
  team,
  profile,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  team: LibraryTeam;
  profile: TeamProfile | null;
  onSaved: () => void;
}): ReactNode {
  const [name, setName] = useState(team.name);
  const [aliases, setAliases] = useState(team.aliases.join(', '));
  const [own, setOwn] = useState(profile?.own ?? false);
  const [rank, setRank] = useState(rankText(profile));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setName(team.name);
    setAliases(team.aliases.join(', '));
    setOwn(profile?.own ?? false);
    setRank(rankText(profile));
    setError(null);
  }, [open, team.name, team.aliases, profile]);
  const rankValue = rank.trim() === '' ? 0 : Number(rank);
  const rankValid = Number.isInteger(rankValue) && rankValue >= 0 && rankValue <= HLTV_RANK_MAX;

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const list = aliases.split(',').map((alias) => alias.trim()).filter((alias) => alias !== '');
      await patchLibraryTeam(team.id, { name: name.trim(), aliases: list });
      if (own !== (profile?.own ?? false) || rankValue !== (profile?.hltv_rank ?? 0)) {
        await setTeamProfile(team.id, { own, hltv_rank: rankValue });
      }
      onOpenChange(false);
      onSaved();
    } catch {
      setError('No se pudo guardar. Un alias que ya tiene otro equipo se resuelve fusionando los dos.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Editar equipo</DialogTitle>
            <DialogDescription>
              Los alias son los nombres de clan que resuelven a este equipo. Las partidas asignadas solas con un alias nuevo pasan a él.
            </DialogDescription>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Nombre
            <Input autoFocus value={name} maxLength={120} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Alias, separados por comas
            <Input value={aliases} maxLength={600} onChange={(event) => setAliases(event.target.value)} />
          </label>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Ranking HLTV, vacío si no tiene
            <Input
              type="number"
              inputMode="numeric"
              min={1}
              max={HLTV_RANK_MAX}
              value={rank}
              aria-invalid={!rankValid}
              onChange={(event) => setRank(event.target.value)}
              className="w-28"
            />
          </label>
          <div className="flex items-center justify-between gap-3 text-body-sm text-fg-2">
            <span id="team-own-label">Es mi equipo: el Replay separa sus partidas de las de otros equipos. Solo hay uno; marcarlo lo quita del anterior.</span>
            <Switch checked={own} onCheckedChange={setOwn} aria-labelledby="team-own-label" />
          </div>
          {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || name.trim() === '' || !rankValid}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function MergeTeamDialog({
  open,
  onOpenChange,
  team,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  team: LibraryTeam;
}): ReactNode {
  const router = useRouter();
  const [teams, setTeams] = useState<LibraryTeam[]>([]);
  const [target, setTarget] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setTarget('');
    setError(null);
    listLibraryTeams()
      .then((list) => setTeams(list.filter((entry) => entry.id !== team.id).sort((a, b) => a.name.localeCompare(b.name, 'es'))))
      .catch(() => setTeams([]));
  }, [open, team.id]);

  async function merge(): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      const into = await mergeLibraryTeam(team.id, target);
      onOpenChange(false);
      router.replace(teamHref(into.id));
    } catch {
      setError('No se pudieron fusionar los equipos.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Fusionar {team.name}</DialogTitle>
          <DialogDescription>
            Sus partidas y sus alias pasan al equipo que elijas, y esta ficha desaparece. Sirve para un mismo equipo que aparece con dos nombres.
          </DialogDescription>
        </DialogHeader>
        {teams.length === 0 ? (
          <p className="text-body-sm text-fg-2">No hay otro equipo con el que fusionarlo.</p>
        ) : (
          <Select value={target} onValueChange={setTarget}>
            <SelectTrigger aria-label="Equipo que se queda" className="h-9 w-full">
              <SelectValue placeholder="Elige el equipo que se queda" />
            </SelectTrigger>
            <SelectContent>
              {teams.map((entry) => (
                <SelectItem key={entry.id} value={entry.id}>
                  {entry.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
        {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
        <DialogFooter>
          <Button type="button" variant="outline" disabled={busy} onClick={() => onOpenChange(false)}>
            Cancelar
          </Button>
          <Button type="button" disabled={busy || target === ''} onClick={() => void merge()}>
            <GitMerge aria-hidden />
            Fusionar
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
