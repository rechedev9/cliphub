'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { FolderOpen, Loader2, RefreshCw, Trash2 } from 'lucide-react';
import { api } from '@/lib/api';
import type { StorageFile, StorageFiles } from '@/lib/api/storage';
import type { Match } from '@/lib/api/types';
import { deleteErrorMessage, deleteFeedbackTone } from '@/lib/delete-error';
import { formatBytes, storageLoadErrorMessage } from '@/lib/storage';
import {
  STORAGE_FILE_KIND_LABEL,
  formatFileDate,
  groupByFolder,
  groupByKind,
  libraryVideoFor,
  matchLabel,
  splitPath,
  totalBytes,
  type FileGroup,
} from '@/lib/storage-files';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

const SCAN_POLL_MS = 2000;

type MatchIndex = ReadonlyMap<string, Pick<Match, 'map' | 'player'>>;

type LoadState =
  | { kind: 'loading' }
  | { kind: 'error'; message: string }
  | { kind: 'ready'; files: StorageFiles; matches: MatchIndex };

/** Almacenamiento: where demos and Studio videos live on this PC, with delete per file. */
export function StorageFilesScreen({ children }: { children?: ReactNode }): ReactNode {
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [notice, setNotice] = useState<string | null>(null);
  const [rescanError, setRescanError] = useState<string | null>(null);
  const [childrenKey, setChildrenKey] = useState(0);

  const load = useCallback(async (): Promise<void> => {
    try {
      const [files, matches] = await Promise.all([api.listStorageFiles(), api.listMatches().catch(() => [])]);
      setState({ kind: 'ready', files, matches: new Map(matches.map((match) => [match.id, match])) });
    } catch (err) {
      setState({ kind: 'error', message: storageLoadErrorMessage(err) });
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const scanning = state.kind === 'ready' && state.files.scanning;
  useEffect(() => {
    if (!scanning) return;
    const timer = setTimeout(() => void load(), SCAN_POLL_MS);
    return () => clearTimeout(timer);
  }, [scanning, state, load]);

  const rescan = async (): Promise<void> => {
    setNotice(null);
    setRescanError(null);
    try {
      await api.rescanStorageFiles();
    } catch (err) {
      setRescanError(storageLoadErrorMessage(err));
    }
    await load();
  };

  const removeFile = (path: string, freedBytes: number, label: string): void => {
    setNotice(`${label}: se han liberado ${formatBytes(freedBytes)}.`);
    setState((current) => {
      if (current.kind !== 'ready') return current;
      const keep = (file: StorageFile): boolean => file.path !== path;
      const files = current.files;
      return {
        ...current,
        files: { ...files, demos: files.demos.filter(keep), studioDemos: files.studioDemos.filter(keep), videos: files.videos.filter(keep) },
      };
    });
  };

  let body: ReactNode;
  if (state.kind === 'ready') {
    body = <ReadyBody files={state.files} matches={state.matches} onRemoved={removeFile} onMatchDeleted={() => { setChildrenKey((key) => key + 1); void load(); }} />;
  } else if (state.kind === 'error') {
    body = (
      <div className="flex flex-col items-start gap-3">
        <p role="alert" className="text-body-sm text-destructive">{state.message}</p>
        <Button type="button" variant="outline" size="sm" onClick={() => { setState({ kind: 'loading' }); void load(); }}>
          Reintentar
        </Button>
      </div>
    );
  } else {
    body = (
      <div role="status" aria-label="Leyendo archivos" className="flex flex-col gap-2">
        <Skeleton className="h-16 w-full rounded-none" />
        <Skeleton className="h-40 w-full rounded-none" />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="Almacenamiento"
        description="Dónde están las demos de tu PC y los vídeos que ha creado Studio. Borra lo que ya no necesites."
        actions={
          <Button type="button" variant="outline" size="sm" disabled={state.kind !== 'ready' || scanning} onClick={() => void rescan()}>
            {scanning ? <Loader2 aria-hidden className="motion-safe:animate-spin" /> : <RefreshCw aria-hidden />}
            {scanning ? 'Buscando demos…' : 'Buscar de nuevo'}
          </Button>
        }
      />
      {notice ? <p role="status" className="text-body-sm text-success">{notice}</p> : null}
      {rescanError ? <p role="alert" className="text-body-sm text-destructive">{rescanError}</p> : null}
      {body}
      <div key={childrenKey}>{children}</div>
    </div>
  );
}

function ReadyBody({
  files,
  matches,
  onRemoved,
  onMatchDeleted,
}: {
  files: StorageFiles;
  matches: MatchIndex;
  onRemoved: (path: string, freedBytes: number, label: string) => void;
  onMatchDeleted: () => void;
}): ReactNode {
  const demoGroups = useMemo(() => groupByFolder(files.demos), [files.demos]);
  const videoGroups = useMemo(() => groupByKind(files.videos), [files.videos]);

  return (
    <div className="flex flex-col gap-8">
      <dl className="grid gap-px border border-border bg-border @[40rem]/content:grid-cols-3">
        <Summary label="Demos en tu PC" files={files.demos} pending={files.scanning && files.scannedAt === null} />
        <Summary label="Copias de demos en Studio" files={files.studioDemos} />
        <Summary label="Vídeos creados por Studio" files={files.videos} />
      </dl>

      <Section
        title="Demos en tu PC"
        description={files.scanning
          ? 'Studio está buscando archivos .dem en todos tus discos. La lista se completa sola.'
          : 'Todos los archivos .dem de tus discos, agrupados por carpeta.'}
      >
        {demoGroups.length === 0 ? (
          <EmptyLine>{files.scanning ? 'Buscando demos…' : 'No hay demos en tu PC fuera de Studio.'}</EmptyLine>
        ) : (
          demoGroups.map((group) => (
            <Group key={group.key} group={group}>
              {group.files.map((file) => (
                <FileRow key={file.path} file={file} title={splitPath(file.path).name} onRemoved={onRemoved} />
              ))}
            </Group>
          ))
        )}
      </Section>

      <Section
        title="Vídeos creados por Studio"
        description={<>Se guardan en la carpeta de Studio: <span className="break-all font-mono text-meta">{files.dataDir}</span></>}
      >
        {videoGroups.length === 0 ? (
          <EmptyLine>Todavía no has creado ningún vídeo.</EmptyLine>
        ) : (
          videoGroups.map((group) => (
            <Group key={group.key} group={group}>
              {group.files.map((file) => (
                <FileRow
                  key={file.path}
                  file={file}
                  title={matchLabel(file.jobId, matches) ?? splitPath(file.path).name}
                  subtitle={matchLabel(file.jobId, matches) ? splitPath(file.path).name : undefined}
                  onRemoved={onRemoved}
                />
              ))}
            </Group>
          ))
        )}
      </Section>

      <Section
        title="Copias de demos en Studio"
        description="Al cargar una demo, Studio guarda una copia para analizarla y grabarla. Borrar la partida quita la copia, sus vídeos y sus capturas. Tu .dem original no se toca."
      >
        {files.studioDemos.length === 0 ? (
          <EmptyLine>Studio no guarda ninguna copia.</EmptyLine>
        ) : (
          <ul className="flex flex-col gap-2">
            {files.studioDemos.map((file) => (
              <StudioDemoRow key={file.path} file={file} label={matchLabel(file.jobId, matches)} onDeleted={onMatchDeleted} />
            ))}
          </ul>
        )}
      </Section>
    </div>
  );
}

function Summary({ label, files, pending = false }: { label: string; files: readonly StorageFile[]; pending?: boolean }): ReactNode {
  return (
    <div className="flex flex-col gap-1 bg-surface-1 px-4 py-3">
      <dt className="text-body-sm text-fg-2">{label}</dt>
      <dd className="font-display text-title font-bold tabular-nums text-fg-1">
        {pending ? '…' : formatBytes(totalBytes(files))}
        <span className="ml-2 text-body-sm font-normal text-fg-2">
          {pending ? '' : `${files.length} ${files.length === 1 ? 'archivo' : 'archivos'}`}
        </span>
      </dd>
    </div>
  );
}

function Section({ title, description, children }: { title: string; description: ReactNode; children: ReactNode }): ReactNode {
  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-col gap-1">
        <h2 className="font-display text-title font-bold text-fg-1">{title}</h2>
        <p className="measure-read text-body-sm text-fg-2">{description}</p>
      </div>
      {children}
    </section>
  );
}

function Group({ group, children }: { group: FileGroup; children: ReactNode }): ReactNode {
  return (
    <div className="flex flex-col border border-border bg-surface-1">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-b border-border px-3.5 py-2.5">
        <span className="min-w-0 break-all font-mono text-meta text-fg-1">{group.label}</span>
        <span className="text-meta tabular-nums text-fg-2">
          {formatBytes(group.totalBytes)} · {group.files.length} {group.files.length === 1 ? 'archivo' : 'archivos'}
        </span>
      </div>
      <ul className="flex flex-col divide-y divide-border">{children}</ul>
    </div>
  );
}

function EmptyLine({ children }: { children: ReactNode }): ReactNode {
  return <p className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">{children}</p>;
}

/** One deletable file; "Borrar" asks inline before it deletes for good. */
function FileRow({
  file,
  title,
  subtitle,
  onRemoved,
}: {
  file: StorageFile;
  title: string;
  subtitle?: string;
  onRemoved: (path: string, freedBytes: number, label: string) => void;
}): ReactNode {
  const [armed, setArmed] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const meta = [subtitle, formatBytes(file.sizeBytes), formatFileDate(file.modifiedAt)].filter(Boolean).join(' · ');

  const confirm = async (): Promise<void> => {
    if (deleting) return;
    setDeleting(true);
    setError(null);
    try {
      const freed = await api.deleteStorageFile(file.path);
      const cardId = libraryVideoFor(await api.listVideos().catch(() => []), file);
      if (cardId !== null) await api.deleteVideo(cardId).catch(() => undefined);
      onRemoved(file.path, freed, title);
    } catch (err) {
      setError(deleteErrorMessage(err));
      setArmed(false);
      setDeleting(false);
    }
  };

  return (
    <li className="flex flex-col gap-2 px-3.5 py-2.5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="truncate text-body-sm font-semibold text-fg-1" title={file.path}>{title}</span>
          <span className="text-meta tabular-nums text-fg-2">{meta}</span>
        </div>
        <div className="flex items-center gap-1">
          <RevealButton path={file.path} name={title} />
          {armed ? null : (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => { setError(null); setArmed(true); }}
              className="hover:bg-destructive/10 hover:text-destructive"
              aria-label={`Borrar ${title}`}
            >
              <Trash2 aria-hidden />
              Borrar
            </Button>
          )}
        </div>
      </div>
      {armed ? (
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-body-sm text-fg-2">
            {file.kind === 'demo' ? 'Se borra este .dem de tu PC.' : `Se borra este vídeo (${STORAGE_FILE_KIND_LABEL[file.kind]}).`} No se puede deshacer.
          </p>
          <Button type="button" variant="destructive" size="sm" autoFocus loading={deleting} loadingText="Borrando…" onClick={() => void confirm()}>
            <Trash2 aria-hidden />
            Borrar
          </Button>
          <Button type="button" variant="outline" size="sm" disabled={deleting} onClick={() => setArmed(false)}>Cancelar</Button>
        </div>
      ) : null}
      {error ? (
        <p role="status" className={`text-body-sm ${deleteFeedbackTone(error) === 'notice' ? 'text-fg-2' : 'text-destructive'}`}>{error}</p>
      ) : null}
    </li>
  );
}

/** A Studio demo copy goes with its match; a library copy has no match to delete here. */
function StudioDemoRow({ file, label, onDeleted }: { file: StorageFile; label: string | null; onDeleted: () => void }): ReactNode {
  const [armed, setArmed] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const title = label ?? (file.jobId ? 'Partida sin nombre' : 'Demo de la biblioteca');
  const jobId = file.jobId;

  const confirm = async (): Promise<void> => {
    if (deleting || jobId === undefined) return;
    setDeleting(true);
    setError(null);
    try {
      await api.deleteMatch(jobId);
      onDeleted();
    } catch (err) {
      setError(deleteErrorMessage(err));
      setArmed(false);
      setDeleting(false);
    }
  };

  return (
    <li className="flex flex-col gap-2 border border-border bg-surface-1 px-3.5 py-2.5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="truncate text-body-sm font-semibold text-fg-1" title={file.path}>{title}</span>
          <span className="text-meta tabular-nums text-fg-2">{formatBytes(file.sizeBytes)} · {formatFileDate(file.modifiedAt)}</span>
        </div>
        <div className="flex items-center gap-1">
          <RevealButton path={file.path} name={title} />
          {jobId === undefined || armed ? null : (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => { setError(null); setArmed(true); }}
              className="hover:bg-destructive/10 hover:text-destructive"
            >
              <Trash2 aria-hidden />
              Borrar partida
            </Button>
          )}
        </div>
      </div>
      {armed ? (
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-body-sm text-fg-2">Se borra la partida de Studio con sus vídeos y capturas. No se puede deshacer.</p>
          <Button type="button" variant="destructive" size="sm" autoFocus loading={deleting} loadingText="Borrando…" onClick={() => void confirm()}>
            <Trash2 aria-hidden />
            Borrar partida
          </Button>
          <Button type="button" variant="outline" size="sm" disabled={deleting} onClick={() => setArmed(false)}>Cancelar</Button>
        </div>
      ) : null}
      {error ? (
        <p role="status" className={`text-body-sm ${deleteFeedbackTone(error) === 'notice' ? 'text-fg-2' : 'text-destructive'}`}>{error}</p>
      ) : null}
    </li>
  );
}

function RevealButton({ path, name }: { path: string; name: string }): ReactNode {
  const [failed, setFailed] = useState(false);
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      onClick={() => {
        setFailed(false);
        api.revealStorageFile(path).catch(() => setFailed(true));
      }}
      aria-label={`Mostrar ${name} en la carpeta`}
      title={failed ? 'No se pudo abrir la carpeta. Pulsa Buscar de nuevo.' : path}
    >
      <FolderOpen aria-hidden />
      <span className="hidden @[48rem]/content:inline">{failed ? 'No se pudo abrir' : 'Mostrar en carpeta'}</span>
    </Button>
  );
}
