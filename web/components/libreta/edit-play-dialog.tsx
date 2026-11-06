'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import { patchLibraryPlay } from '@/lib/api/library';
import type { LibretaStatus } from '@/lib/api/libreta';
import type { LibraryPlay } from '@/lib/api/stratbook';
import type { TacticalSite } from '@/lib/api/tactical';
import { parseTags, tagsText } from '@/lib/libreta/status';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Field, KindField, NO_TEAM, SiteField, TagsField, TeamField, TitleField, useLibraryTeams } from './play-fields';
import { StatusSelect } from './status-badge';

/** Edits what a play is called and how the team uses it; map and side stay those of its rounds. */
export function EditPlayDialog({ play, onClose, onSaved }: { play: LibraryPlay; onClose: () => void; onSaved: () => void }): ReactNode {
  const teams = useLibraryTeams();
  const [title, setTitle] = useState(play.title);
  const [kind, setKind] = useState(play.kind);
  const [site, setSite] = useState<TacticalSite>(play.site);
  const [teamId, setTeamId] = useState(play.team_id ?? NO_TEAM);
  const [status, setStatus] = useState<LibretaStatus>(play.status);
  const [tags, setTags] = useState(tagsText(play.tags));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await patchLibraryPlay(play.id, {
        title: title.trim(),
        kind,
        site,
        team_id: teamId === NO_TEAM ? null : teamId,
        status,
        tags: parseTags(tags),
      });
      onSaved();
      onClose();
    } catch {
      setError('No se pudo guardar la jugada.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Editar la jugada</DialogTitle>
            <DialogDescription>El mapa y el lado no cambian: una jugada en otro mapa es otra jugada.</DialogDescription>
          </DialogHeader>
          <TitleField value={title} onChange={setTitle} />
          <div className="grid gap-3 sm:grid-cols-2">
            <KindField side={play.side} value={kind} onChange={setKind} />
            <SiteField value={site} onChange={setSite} />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <TeamField value={teamId} onChange={setTeamId} teams={teams} currentName={play.team_name} />
            <Field label="Estado">
              <StatusSelect value={status} onChange={setStatus} />
            </Field>
          </div>
          <TagsField value={tags} onChange={setTags} />
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || title.trim() === ''}>
              Guardar
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
