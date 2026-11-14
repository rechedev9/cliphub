'use client';

import { useCallback, useEffect, useId, useRef, useState, type DragEvent, type FormEvent, type ReactNode } from 'react';
import { FileText, Trash2, Upload } from 'lucide-react';
import { toast } from 'sonner';
import { formatShortDate } from '@/lib/format';
import {
  defaultTeamDocumentTitle,
  deleteTeamDocument,
  formatDocumentSize,
  listTeamDocuments,
  teamDocumentFileProblem,
  uploadTeamDocument,
  TEAM_DOCUMENT_ACCEPT,
  TEAM_DOCUMENT_TITLE_MAX,
  type TeamDocument,
} from '@/lib/api/team-documents';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { DocumentViewer } from './document-viewer';

type ListState = { kind: 'loading' } | { kind: 'ready'; documents: TeamDocument[] } | { kind: 'error'; message: string };

/**
 * The Documentos tab of the Buscador de patrones: the HTML reports a coach or an assistant attached to the team.
 * `openId` is the document in the viewer, kept in the URL so a link (or an assistant's studio_path) opens it.
 */
export function TeamDocuments({ teamId, teamName, openId, onOpen }: { teamId: string; teamName: string; openId: string | null; onOpen: (id: string | null) => void }): ReactNode {
  const [state, setState] = useState<ListState>({ kind: 'loading' });
  const [pending, setPending] = useState<File | null>(null);
  const [confirming, setConfirming] = useState<TeamDocument | null>(null);
  const [dragging, setDragging] = useState(false);
  const input = useRef<HTMLInputElement>(null);

  const load = useCallback(
    (signal?: AbortSignal) => {
      listTeamDocuments(teamId, signal)
        .then((documents) => setState({ kind: 'ready', documents }))
        .catch((error: unknown) => {
          if (signal?.aborted) return;
          setState({ kind: 'error', message: error instanceof Error ? error.message : 'No se pudieron leer los documentos.' });
        });
    },
    [teamId],
  );
  useEffect(() => {
    const controller = new AbortController();
    load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const pick = (file: File | undefined): void => {
    if (file === undefined) return;
    const problem = teamDocumentFileProblem(file);
    if (problem !== null) {
      toast.error(problem);
      return;
    }
    setPending(file);
  };
  const drop = (event: DragEvent<HTMLDivElement>): void => {
    event.preventDefault();
    setDragging(false);
    pick(event.dataTransfer.files[0]);
  };

  const remove = async (doc: TeamDocument): Promise<void> => {
    setConfirming(null);
    try {
      await deleteTeamDocument(doc.id);
      if (openId === doc.id) onOpen(null);
      toast.success(`Documento «${doc.title}» borrado.`);
      load();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'No se pudo borrar el documento.');
    }
  };

  const documents = state.kind === 'ready' ? state.documents : [];
  const open = documents.find((doc) => doc.id === openId) ?? null;

  return (
    <div
      className={cn('flex flex-col gap-3 rounded-md', dragging && 'outline-2 outline-offset-4 outline-dashed outline-primary')}
      onDragOver={(event) => {
        event.preventDefault();
        setDragging(true);
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={drop}
    >
      <div className="flex items-center justify-between gap-2">
        <p className="text-body-sm text-fg-2">Informes HTML del equipo</p>
        <Button type="button" variant="outline" size="sm" onClick={() => input.current?.click()}>
          <Upload aria-hidden />
          Subir
        </Button>
        <input
          ref={input}
          type="file"
          accept={TEAM_DOCUMENT_ACCEPT}
          className="sr-only"
          tabIndex={-1}
          aria-label="Elegir un documento HTML"
          onChange={(event) => {
            pick(event.target.files?.[0]);
            event.target.value = '';
          }}
        />
      </div>

      {state.kind === 'loading' ? (
        <div role="status" aria-label="Cargando los documentos" className="flex flex-col gap-2">
          <Skeleton className="h-11 w-full" />
          <Skeleton className="h-11 w-full" />
        </div>
      ) : null}
      {state.kind === 'error' ? (
        <div className="flex flex-col items-start gap-2 text-body-sm text-fg-2">
          <p role="alert">{state.message}</p>
          <Button type="button" variant="outline" size="sm" onClick={() => load()}>
            Reintentar
          </Button>
        </div>
      ) : null}
      {state.kind === 'ready' && documents.length === 0 ? (
        <div className="flex flex-col gap-2 rounded-md border border-dashed border-border-strong p-3 text-body-sm text-fg-2">
          <p className="font-medium text-fg-1">Aún no hay documentos de {teamName}.</p>
          <p>Sube un informe HTML, como las antistrats que prepare el analista o un asistente con el MCP, y ábrelo aquí sin salir del mapa. También puedes arrastrarlo a este panel.</p>
        </div>
      ) : null}
      {documents.length > 0 ? (
        <ul className="flex flex-col gap-1.5" aria-label={`Documentos de ${teamName}`}>
          {documents.map((doc) => (
            <li key={doc.id} className="group flex items-center gap-1 rounded-md border border-border bg-surface-3 hover:border-border-strong">
              <button
                type="button"
                onClick={() => onOpen(doc.id)}
                className="flex min-w-0 flex-1 items-center gap-2.5 rounded-md px-2.5 py-2 text-left outline-none focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
              >
                <FileText aria-hidden className="size-4 shrink-0 text-fg-3" />
                <span className="flex min-w-0 flex-col">
                  <span className="truncate text-body-sm font-medium text-fg-1">{doc.title}</span>
                  <span className="truncate text-meta text-fg-3">
                    {formatShortDate(doc.updated_at)} · {formatDocumentSize(doc.size_bytes)}
                  </span>
                </span>
              </button>
              <Button type="button" variant="ghost" size="icon-xs" className="mr-1.5 shrink-0" aria-label={`Borrar el documento ${doc.title}`} onClick={() => setConfirming(doc)}>
                <Trash2 aria-hidden />
              </Button>
            </li>
          ))}
        </ul>
      ) : null}

      <UploadDialog
        file={pending}
        teamId={teamId}
        teamName={teamName}
        onClose={() => setPending(null)}
        onUploaded={(doc) => {
          setPending(null);
          toast.success(`Documento «${doc.title}» subido.`);
          load();
          onOpen(doc.id);
        }}
      />

      <Dialog open={confirming !== null} onOpenChange={(next) => !next && setConfirming(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>¿Borrar «{confirming?.title}»?</DialogTitle>
            <DialogDescription>Se borra para todo el equipo. No se puede deshacer.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="ghost" size="sm" onClick={() => setConfirming(null)}>
              Cancelar
            </Button>
            <Button type="button" variant="destructive" size="sm" onClick={() => confirming !== null && void remove(confirming)}>
              Borrar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <DocumentViewer document={open} onClose={() => onOpen(null)} />
    </div>
  );
}

/** Names the picked file before it is sent: the title defaults to the file name. */
function UploadDialog({
  file,
  teamId,
  teamName,
  onClose,
  onUploaded,
}: {
  file: File | null;
  teamId: string;
  teamName: string;
  onClose: () => void;
  onUploaded: (doc: TeamDocument) => void;
}): ReactNode {
  const [title, setTitle] = useState('');
  const [shown, setShown] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const titleId = useId();

  // Only a new file resets the dialog: clearing it on close would blank the text while it fades out.
  useEffect(() => {
    if (file === null) return;
    setShown(file);
    setTitle(defaultTeamDocumentTitle(file.name));
    setError(null);
  }, [file]);

  const submit = async (event: FormEvent): Promise<void> => {
    event.preventDefault();
    if (file === null || busy) return;
    setBusy(true);
    setError(null);
    try {
      onUploaded(await uploadTeamDocument({ teamId, title, file }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo subir el documento.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open={file !== null} onOpenChange={(next) => !next && !busy && onClose()}>
      <DialogContent className="max-w-md">
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Subir documento</DialogTitle>
            <DialogDescription>
              {shown === null ? null : `${shown.name} · ${formatDocumentSize(shown.size)}. Lo verá todo el que tenga acceso a ${teamName}.`}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={titleId}>Título</Label>
            <Input
              id={titleId}
              value={title}
              maxLength={TEAM_DOCUMENT_TITLE_MAX}
              onChange={(event) => setTitle(event.target.value)}
              aria-invalid={error === null ? undefined : true}
              aria-describedby={error === null ? undefined : `${titleId}-error`}
              autoFocus
            />
            {error === null ? null : (
              <p id={`${titleId}-error`} role="alert" className="text-body-sm text-destructive">
                {error}
              </p>
            )}
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" size="sm" loading={busy} loadingText="Subiendo" disabled={title.trim() === ''}>
              Subir
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
