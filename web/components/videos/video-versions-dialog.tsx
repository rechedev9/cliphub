'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Download, Pencil, Play, Trash2 } from 'lucide-react';
import { deleteVideoVersion, listVideoVersions, versionVideoUrl, type VideoVersion } from '@/lib/api/video-edit';
import { editVideoHref, type ProduceFormat } from '@/lib/clips/routes';
import { versionDateLabel, versionOptionsLabel } from '@/lib/produce/edit-capture';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { downloadMP4 } from '@/components/clips-hub/output-item';

export type VideoVersionsDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  jobId: string;
  variant: string;
  format: ProduceFormat;
};

type ListState = { kind: 'loading' } | { kind: 'error' } | { kind: 'ready'; versions: VideoVersion[] };

/** Versiones in Mis vídeos: every render of a video, newest first. Studio deletes none of them. */
export function VideoVersionsDialog({ open, onOpenChange, title, jobId, variant, format }: VideoVersionsDialogProps): ReactNode {
  const [state, setState] = useState<ListState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    setState({ kind: 'loading' });
    void listVideoVersions({ jobId, variant, signal: controller.signal })
      .then((versions) => { if (!controller.signal.aborted) setState({ kind: 'ready', versions }); })
      .catch(() => { if (!controller.signal.aborted) setState({ kind: 'error' }); });
    return () => controller.abort();
  }, [open, jobId, variant, attempt]);

  const onDeleted = (revision: string): void => {
    setState((current) => current.kind === 'ready' ? { kind: 'ready', versions: current.versions.filter((version) => version.revision !== revision) } : current);
  };

  let body: ReactNode;
  if (state.kind === 'ready' && state.versions.length > 0) {
    body = (
      <ul className="flex flex-col gap-2" aria-label={`Versiones de ${title}`}>
        {state.versions.map((version) => (
          <VersionRow key={version.revision} version={version} title={title} jobId={jobId} variant={variant} format={format} onDeleted={() => onDeleted(version.revision)} />
        ))}
      </ul>
    );
  } else if (state.kind === 'ready') {
    body = <p className="text-body-sm text-fg-2">Este vídeo aún no tiene versiones guardadas.</p>;
  } else if (state.kind === 'error') {
    body = (
      <div className="flex flex-col items-start gap-3">
        <p role="alert" className="text-body-sm text-destructive">No se pudieron leer las versiones. Comprueba que Studio sigue abierto.</p>
        <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((value) => value + 1)}>Reintentar</Button>
      </div>
    );
  } else {
    body = <p role="status" className="text-body-sm text-fg-2">Leyendo versiones…</p>;
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Versiones de {title}</DialogTitle>
          <DialogDescription>
            Cada edición crea una versión nueva y la tarjeta muestra la última. Studio no borra ninguna: bórralas tú cuando no las necesites.
          </DialogDescription>
        </DialogHeader>
        {body}
      </DialogContent>
    </Dialog>
  );
}

type VersionRowProps = { version: VideoVersion; title: string; jobId: string; variant: string; format: ProduceFormat; onDeleted: () => void };

function VersionRow({ version, title, jobId, variant, format, onDeleted }: VersionRowProps): ReactNode {
  const [playing, setPlaying] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const name = version.videos[0];
  const url = name === undefined ? null : versionVideoUrl({ jobId, variant, revision: version.revision, name });
  const date = versionDateLabel(version.createdAt);

  const confirm = async (): Promise<void> => {
    setError(null);
    try {
      await deleteVideoVersion({ jobId, variant, revision: version.revision });
      onDeleted();
    } catch {
      setError('No se pudo borrar la versión. Espera a que termine lo que Studio esté grabando o montando y prueba de nuevo.');
    }
  };

  return (
    <li className="flex flex-col gap-3 border border-border bg-surface-1 px-3.5 py-2.5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <span className="flex items-center gap-2 text-body-sm font-semibold text-fg-1">
            {date}
            {version.current ? <StatusTag tone="success">En Mis vídeos</StatusTag> : null}
          </span>
          <span className="text-meta text-fg-2">{versionOptionsLabel(version)}</span>
        </div>
        <div className="flex flex-wrap gap-1.5">
          <Button type="button" size="sm" variant="outline" disabled={url === null} aria-pressed={playing} onClick={() => setPlaying((value) => !value)}>
            <Play aria-hidden />
            {playing ? 'Ocultar' : 'Ver'}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={url === null} onClick={() => url && downloadMP4(url, `${title} (${date})`)}>
            <Download aria-hidden />
            MP4
          </Button>
          <Button asChild size="sm" variant="outline">
            <Link href={editVideoHref({ matchId: jobId, format, variant, revision: version.revision })}>
              <Pencil aria-hidden />
              Editar
            </Link>
          </Button>
          {version.current ? null : (
            <ConfirmDialog
              title={`¿Borrar la versión del ${date}?`}
              description="Se borra el MP4 de esta versión. No se puede deshacer."
              confirmLabel="Borrar versión"
              pendingLabel="Borrando…"
              onConfirm={confirm}
              trigger={
                <Button type="button" size="sm" variant="ghost" className="hover:bg-destructive/10 hover:text-destructive" aria-label={`Borrar la versión del ${date}`}>
                  <Trash2 aria-hidden />
                  Borrar
                </Button>
              }
            />
          )}
        </div>
      </div>
      {playing && url ? <video controls preload="metadata" src={url} className="max-h-80 w-full bg-surface-0" aria-label={`${title}, versión del ${date}`} /> : null}
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
    </li>
  );
}
