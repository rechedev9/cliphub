'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Pencil, RefreshCw, Trash2 } from 'lucide-react';
import {
  LIBRARY_SOURCES,
  analyzeLibraryDemo,
  assignLibraryMatchTeam,
  deleteLibraryDemo,
  listLibraryTeams,
  patchLibraryDemo,
  type LibraryDemo,
  type LibraryMatch,
  type LibraryMatchTeam,
  type LibrarySource,
  type LibraryTeam,
} from '@/lib/api/library';
import { MATCHES_HREF, playerHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { PARSE_STATE_LABEL, SOURCE_LABEL, adrLabel, demoErrorLabel, teamLabel } from '@/lib/library/partidas';
import { tacticalFilterToQuery } from '@/lib/tactical-filter';
import { MATCH_TAB_QUERY, MATCH_TACTICAL_TAB } from '@/lib/library/rounds';
import { RelatedList } from '@/components/records/related-list';
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
import { cn } from '@/lib/utils';

const ASSIGNMENT_NOTE: Record<LibraryMatchTeam['assignment'], string> = {
  auto: 'Asignado por el nombre de clan de la demo.',
  manual: 'Corregido a mano; un nuevo análisis lo respeta.',
  none: 'La demo no trae nombre de clan; asígnalo a mano.',
};

/** Resumen: both teams with their scoreboards, and the demo the match was read from. */
export function MatchSummary({
  match,
  demo,
  pathname,
  onMatchChange,
  onDemoChange,
  onOpenTactical,
}: {
  match: LibraryMatch;
  demo: LibraryDemo;
  pathname: string;
  onMatchChange: (match: LibraryMatch) => void;
  onDemoChange: (demo: LibraryDemo) => void;
  onOpenTactical: () => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-5">
      <div className="grid gap-5 @[64rem]/content:grid-cols-2">
        {match.teams.map((team) => (
          <TeamPanel
            key={team.key}
            match={match}
            team={team}
            pathname={pathname}
            onMatchChange={onMatchChange}
            onOpenTactical={onOpenTactical}
          />
        ))}
      </div>
      <DemoPanel demo={demo} onDemoChange={onDemoChange} />
    </div>
  );
}

function TeamPanel({
  match,
  team,
  pathname,
  onMatchChange,
  onOpenTactical,
}: {
  match: LibraryMatch;
  team: LibraryMatchTeam;
  pathname: string;
  onMatchChange: (match: LibraryMatch) => void;
  onOpenTactical: () => void;
}): ReactNode {
  const [editing, setEditing] = useState(false);
  const rival = match.teams.find((other) => other.key !== team.key);
  const won = rival !== undefined && team.score > rival.score;
  const lost = rival !== undefined && team.score < rival.score;
  const players = (match.players ?? []).filter((player) => player.team_key === team.key);
  const name = teamLabel(team);

  return (
    <RelatedList
      title={name}
      actions={
        <Button type="button" variant="ghost" size="xs" onClick={() => setEditing(true)}>
          <Pencil aria-hidden />
          Corregir equipo
        </Button>
      }
    >
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b border-border-subtle px-4 py-3">
        <span
          className={cn(
            'font-mono text-section font-semibold tabular-nums',
            won && 'text-outcome-win',
            lost && 'text-outcome-loss',
            !won && !lost && 'text-fg-1',
          )}
          aria-label={`${team.score} rondas ganadas`}
        >
          {team.score}
        </span>
        <span className={cn('font-mono text-meta font-semibold', team.start_side === 'CT' ? 'text-side-ct' : 'text-side-t')}>
          Empezó {team.start_side}
        </span>
        {team.team_id ? (
          <Link href={teamHref(team.team_id)} className="text-body-sm font-medium text-primary hover:underline">
            Ficha del equipo
          </Link>
        ) : null}
        <span className="basis-full text-meta text-fg-3">
          {ASSIGNMENT_NOTE[team.assignment]}
          {team.clan_name ? ` Clan en la demo: ${team.clan_name}.` : ''}
        </span>
      </div>
      <table aria-label={`Jugadores de ${name}`} className="w-full border-collapse text-left text-body-sm">
        <thead className="bg-surface-2 text-fg-3">
          <tr className="border-b border-border">
            <th scope="col" className="h-9 px-3 font-medium">Jugador</th>
            {['K', 'D', 'A', 'ADR', 'Rondas'].map((header) => (
              <th key={header} scope="col" className="h-9 px-3 text-right font-medium">
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {players.map((player) => {
            // The player's rounds in the replay: the same rounds these sums were read from.
            const rounds = `${pathname}?${MATCH_TAB_QUERY}=${MATCH_TACTICAL_TAB}&${tacticalFilterToQuery({ slots: [player.slot] })}`;
            return (
              <tr key={player.steamid64} className="border-b border-border-subtle last:border-b-0">
                <td className="h-10 px-3">
                  <Link href={playerHref(player.steamid64)} className="font-medium text-fg-1 hover:text-primary hover:underline">
                    {player.name || player.steamid64}
                  </Link>
                </td>
                <td className="px-3 text-right font-mono">{player.kills}</td>
                <td className="px-3 text-right font-mono">{player.deaths}</td>
                <td className="px-3 text-right font-mono">{player.assists}</td>
                <td className="px-3 text-right font-mono">{adrLabel(player.damage, player.rounds)}</td>
                <td className="px-3 text-right font-mono">
                  <Link href={rounds} onClick={onOpenTactical} className="hover:text-primary hover:underline" aria-label={`Ver las ${player.rounds} rondas de ${player.name} en el replay`}>
                    {player.rounds}
                  </Link>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {players.length === 0 ? <p className="px-4 py-3 text-body-sm text-fg-3">Sin jugadores identificados.</p> : null}
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
    </RelatedList>
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

function DemoPanel({ demo, onDemoChange }: { demo: LibraryDemo; onDemoChange: (demo: LibraryDemo) => void }): ReactNode {
  const router = useRouter();
  const [tags, setTags] = useState(demo.tags.join(', '));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => {
    setTags(demo.tags.join(', '));
  }, [demo.tags]);

  async function act(run: () => Promise<void>, failure: string): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      await run();
    } catch {
      setError(failure);
    } finally {
      setBusy(false);
    }
  }

  const tagList = tags.split(',').map((tag) => tag.trim()).filter((tag) => tag !== '');
  const tagsChanged = tagList.join('\n') !== demo.tags.join('\n');

  return (
    <RelatedList title="Demo">
      <dl className="grid gap-x-8 gap-y-3 px-4 py-3 text-body-sm sm:grid-cols-2 @[64rem]/content:grid-cols-4">
        <Fact label="Archivo"><span className="font-mono text-meta break-all">{demo.file_name}</span></Fact>
        <Fact label="Estado">
          {PARSE_STATE_LABEL[demo.state]}
          {demo.state === 'failed' ? <span className="block text-meta text-destructive">{demoErrorLabel(demo.error_code)}</span> : null}
        </Fact>
        <Fact label="Importada">{formatShortDate(demo.imported_at)}</Fact>
        <Fact label="Huella SHA-256"><span className="font-mono text-meta" title={demo.sha256}>{demo.sha256.slice(0, 12)}</span></Fact>
        <Fact label="Build de CS2">{demo.cs2_build ?? 'Sin dato'}</Fact>
        <Fact label="Fuente">
          <Select
            value={demo.source}
            disabled={busy}
            onValueChange={(next) => {
              const source = LIBRARY_SOURCES.find((entry) => entry === next);
              if (source === undefined) return;
              void act(async () => onDemoChange(await patchLibraryDemo(demo.id, { source })), 'No se pudo cambiar la fuente.');
            }}
          >
            <SelectTrigger aria-label="Fuente de la demo" className="h-8 w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LIBRARY_SOURCES.map((source: LibrarySource) => (
                <SelectItem key={source} value={source}>
                  {SOURCE_LABEL[source]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {demo.source_manual ? <span className="block text-meta text-fg-3">Elegida a mano</span> : null}
        </Fact>
        <div className="flex flex-col gap-1 sm:col-span-2">
          <dt className="text-meta text-fg-3">Etiquetas, separadas por comas</dt>
          <dd className="flex gap-2">
            <Input value={tags} maxLength={400} disabled={busy} onChange={(event) => setTags(event.target.value)} className="h-8" />
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy || !tagsChanged}
              onClick={() => void act(async () => onDemoChange(await patchLibraryDemo(demo.id, { tags: tagList })), 'No se pudieron guardar las etiquetas.')}
            >
              Guardar
            </Button>
          </dd>
        </div>
      </dl>
      <div className="flex flex-wrap items-center gap-2 border-t border-border-subtle px-4 py-3">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={busy || demo.state === 'queued' || demo.state === 'parsing'}
          onClick={() => void act(async () => onDemoChange(await analyzeLibraryDemo(demo.id)), 'No se pudo poner la demo en cola.')}
        >
          <RefreshCw aria-hidden />
          Volver a analizar
        </Button>
        <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => setConfirmDelete(true)}>
          <Trash2 aria-hidden />
          Quitar de la biblioteca
        </Button>
        {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      </div>
      <Dialog open={confirmDelete} onOpenChange={(next) => !busy && setConfirmDelete(next)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Quitar la partida de la biblioteca</DialogTitle>
            <DialogDescription>
              Se borran la copia de la demo en la biblioteca, su análisis y sus correcciones. Los vídeos ya creados y las demos de Crear no se tocan.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => setConfirmDelete(false)}>
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              disabled={busy}
              onClick={() =>
                void act(async () => {
                  await deleteLibraryDemo(demo.id);
                  router.push(MATCHES_HREF);
                }, 'No se pudo quitar la partida.')
              }
            >
              Quitar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </RelatedList>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }): ReactNode {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <dt className="text-meta text-fg-3">{label}</dt>
      <dd className="font-medium text-fg-1">{children}</dd>
    </div>
  );
}
