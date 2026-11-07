'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { Pencil, Radar } from 'lucide-react';
import {
  assignLibraryMatchTeam,
  listLibraryTeams,
  type LibraryMatch,
  type LibraryMatchPlayer,
  type LibraryMatchTeam,
  type LibraryTeam,
} from '@/lib/api/library';
import { playerHref, teamHref } from '@/lib/clips/routes';
import { adrLabel, teamLabel } from '@/lib/library/partidas';
import { MATCH_TAB_QUERY, MATCH_TACTICAL_TAB } from '@/lib/library/rounds';
import { tacticalFilterToQuery } from '@/lib/tactical-filter';
import { Badge } from '@/components/ui/badge';
import { Button, FOCUS_RING } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

const ASSIGNMENT_NOTE: Record<LibraryMatchTeam['assignment'], string> = {
  auto: 'Asignado por el nombre de clan de la demo.',
  manual: 'Corregido a mano; un nuevo análisis lo respeta.',
  none: 'La demo no trae nombre de clan; asígnalo a mano.',
};

/** Average damage per round, 0 for a player with no rounds, the number the tables sort and bar by. */
export function playerAdr(player: Pick<LibraryMatchPlayer, 'damage' | 'rounds'>): number {
  return player.rounds > 0 ? player.damage / player.rounds : 0;
}

/** One team of a partida: its players by ADR, the team record it is filed under and its correction. */
export function MatchTeamPanel({
  match,
  team,
  pathname,
  bestAdr,
  onMatchChange,
  onOpenTactical,
}: {
  match: LibraryMatch;
  team: LibraryMatchTeam;
  pathname: string;
  /** The match's best ADR: the bars' full width and the player the badge goes to. */
  bestAdr: { steamid64: string; adr: number } | null;
  onMatchChange: (match: LibraryMatch) => void;
  onOpenTactical: () => void;
}): ReactNode {
  const [editing, setEditing] = useState(false);
  const players = (match.players ?? [])
    .filter((player) => player.team_key === team.key)
    .sort((a, b) => playerAdr(b) - playerAdr(a));
  const name = teamLabel(team);

  return (
    <section aria-label={name} className="studio-panel flex min-w-0 flex-col overflow-hidden">
      <div className="flex flex-wrap items-start gap-2.5 px-4 pt-4 pb-3">
        <div className="flex min-w-0 grow flex-col gap-0.5">
          <h2 className="truncate text-title font-semibold text-fg-1">{name}</h2>
          <p className="text-meta text-fg-2">
            <span className={cn('font-semibold', team.start_side === 'CT' ? 'text-side-ct' : 'text-side-t')}>Empezó {team.start_side}</span>
            {` · ${team.score} ${team.score === 1 ? 'ronda ganada' : 'rondas ganadas'}`}
          </p>
          <p className="text-meta text-fg-3">
            {ASSIGNMENT_NOTE[team.assignment]}
            {team.clan_name ? ` Clan en la demo: ${team.clan_name}.` : ''}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <Button type="button" variant="ghost" size="xs" aria-label={`Corregir el equipo ${name}`} onClick={() => setEditing(true)}>
            <Pencil aria-hidden />
            Corregir
          </Button>
          {team.team_id ? (
            <Button asChild variant="outline" size="xs">
              <Link href={teamHref(team.team_id)}>Ficha del equipo</Link>
            </Button>
          ) : null}
        </div>
      </div>
      {players.length === 0 ? (
        <p className="border-t border-border-subtle px-4 py-3 text-body-sm text-fg-3">Sin jugadores identificados.</p>
      ) : (
        <table aria-label={`Jugadores de ${name}`} className="w-full table-fixed border-collapse text-left text-body-sm tabular-nums">
          <thead className="text-meta text-fg-3">
            <tr className="border-t border-border-subtle">
              <th scope="col" className="h-8 px-4 font-medium">Jugador</th>
              {['K', 'D', 'A', '+/-'].map((header) => (
                <th key={header} scope="col" className={cn('h-8 px-2 text-right font-medium', header === '+/-' ? 'w-12' : 'w-10')}>
                  {header}
                </th>
              ))}
              <th scope="col" className="h-8 w-36 pr-4 pl-2 text-right font-medium">ADR</th>
            </tr>
          </thead>
          <tbody>
            {players.map((player) => (
              <PlayerRow
                key={player.steamid64}
                player={player}
                best={bestAdr?.steamid64 === player.steamid64}
                maxAdr={bestAdr?.adr ?? 0}
                pathname={pathname}
                onOpenTactical={onOpenTactical}
              />
            ))}
          </tbody>
        </table>
      )}
      <TeamAssignmentDialog
        open={editing}
        onOpenChange={setEditing}
        match={match}
        team={team}
        onSaved={(next) => {
          onMatchChange(next);
          setEditing(false);
        }}
      />
    </section>
  );
}

function PlayerRow({
  player,
  best,
  maxAdr,
  pathname,
  onOpenTactical,
}: {
  player: LibraryMatchPlayer;
  best: boolean;
  maxAdr: number;
  pathname: string;
  onOpenTactical: () => void;
}): ReactNode {
  const name = player.name || player.steamid64;
  const diff = player.kills - player.deaths;
  const bar = maxAdr > 0 ? Math.round((playerAdr(player) / maxAdr) * 100) : 0;
  // The player's rounds in the replay: the same rounds these sums were read from.
  const rounds = `${pathname}?${MATCH_TAB_QUERY}=${MATCH_TACTICAL_TAB}&${tacticalFilterToQuery({ slots: [player.slot] })}`;
  return (
    <tr className="border-t border-border-subtle hover:bg-surface-3">
      <td className="h-10 px-4">
        <span className="flex min-w-0 items-center gap-2">
          <Link href={playerHref(player.steamid64)} className="truncate font-medium text-fg-1 hover:text-primary hover:underline">
            {name}
          </Link>
          {best ? <Badge variant="outline">Mejor ADR</Badge> : null}
          <Link
            href={rounds}
            onClick={onOpenTactical}
            aria-label={`Ver las ${player.rounds} rondas de ${name} en Táctica`}
            title={`Ver sus ${player.rounds} rondas en Táctica`}
            className={cn('ml-auto grid size-7 shrink-0 place-items-center rounded-sm text-fg-3 hover:bg-surface-4 hover:text-fg-1', FOCUS_RING)}
          >
            <Radar aria-hidden className="size-3.5" />
          </Link>
        </span>
      </td>
      <td className="px-2 text-right">{player.kills}</td>
      <td className="px-2 text-right text-fg-2">{player.deaths}</td>
      <td className="px-2 text-right text-fg-2">{player.assists}</td>
      <td className={cn('px-2 text-right font-medium', diff > 0 && 'text-outcome-win', diff < 0 && 'text-outcome-loss', diff === 0 && 'text-fg-3')}>
        {diff > 0 ? `+${diff}` : diff}
      </td>
      <td className="pr-4 pl-2">
        <span className="flex items-center justify-end gap-2">
          <span aria-hidden className="h-1 w-16 overflow-hidden rounded-full bg-surface-4">
            <span className="block h-full bg-fg-3" style={{ width: `${bar}%` }} />
          </span>
          <span className="w-11 text-right font-medium">{adrLabel(player.damage, player.rounds)}</span>
        </span>
      </td>
    </tr>
  );
}

function TeamAssignmentDialog({
  open,
  onOpenChange,
  match,
  team,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  match: LibraryMatch;
  team: LibraryMatchTeam;
  onSaved: (match: LibraryMatch) => void;
}): ReactNode {
  const [name, setName] = useState('');
  const [remember, setRemember] = useState(true);
  const [teams, setTeams] = useState<LibraryTeam[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const listId = `teams-${match.id}-${team.key}`;

  useEffect(() => {
    if (!open) return;
    setName(team.team_name ?? team.clan_name);
    setError(null);
    listLibraryTeams().then(setTeams).catch(() => setTeams([]));
  }, [open, team.team_name, team.clan_name]);

  async function save(input: { team_name?: string; remember_alias?: boolean }): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      onSaved(await assignLibraryMatchTeam(match.id, team.key, input));
    } catch {
      setError('No se pudo guardar el equipo. Si el nombre ya es un alias de otro equipo, elige ese equipo.');
    } finally {
      setBusy(false);
    }
  }

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    if (name.trim() === '') return;
    void save({ team_name: name.trim(), remember_alias: team.clan_name !== '' && remember });
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !busy && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={submit} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Equipo que empezó {team.start_side}</DialogTitle>
            <DialogDescription>
              Escribe el nombre de un equipo de la biblioteca o uno nuevo. La corrección se guarda en esta partida.
            </DialogDescription>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Equipo
            <Input autoFocus list={listId} value={name} maxLength={120} onChange={(event) => setName(event.target.value)} />
            <datalist id={listId}>
              {teams.map((entry) => (
                <option key={entry.id} value={entry.name} />
              ))}
            </datalist>
          </label>
          {team.clan_name ? (
            <label className="flex items-start gap-2 text-body-sm text-fg-2">
              <input type="checkbox" className="mt-1" checked={remember} onChange={(event) => setRemember(event.target.checked)} />
              <span>
                Recordar que el clan «{team.clan_name}» es este equipo. Las demás partidas con ese clan cambian con él.
              </span>
            </label>
          ) : null}
          {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
          <DialogFooter>
            <Button type="button" variant="ghost" disabled={busy || team.team_id === null} onClick={() => void save({})}>
              Quitar equipo
            </Button>
            <Button type="submit" disabled={busy || name.trim() === ''}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
