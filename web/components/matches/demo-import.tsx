'use client';

import { useRef, useState, type DragEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { FileUp } from 'lucide-react';
import { importLibraryDemo, isLibraryOffline, type LibraryImportResult } from '@/lib/api/library';
import { matchHref } from '@/lib/clips/routes';
import { importErrorLabel } from '@/lib/library/partidas';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
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
import { cn } from '@/lib/utils';

const DEMO_FILE = /\.dem(\.zst)?$/i;

type FileResult = { readonly name: string; readonly result: LibraryImportResult | 'pending' | 'error' };

const RESULT_TAG: Record<LibraryImportResult['status'], { label: string; tone: StatusTagTone }> = {
  imported: { label: 'Importada', tone: 'success' },
  duplicate: { label: 'Ya estaba', tone: 'neutral' },
  rejected: { label: 'Rechazada', tone: 'danger' },
};

/** Import of several demos into the library, one request per file so each one reports on its own. */
export function DemoImport({ onImported }: { onImported: () => void }): ReactNode {
  const [open, setOpen] = useState(false);
  const [files, setFiles] = useState<File[]>([]);
  const [tag, setTag] = useState('');
  const [results, setResults] = useState<FileResult[]>([]);
  const [busy, setBusy] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [offline, setOffline] = useState(false);
  const input = useRef<HTMLInputElement>(null);

  function choose(list: FileList | null): void {
    const picked = Array.from(list ?? []).filter((file) => DEMO_FILE.test(file.name));
    setFiles(picked);
    setResults([]);
    setOffline(false);
  }

  function drop(event: DragEvent<HTMLDivElement>): void {
    event.preventDefault();
    setDragging(false);
    choose(event.dataTransfer.files);
  }

  async function run(): Promise<void> {
    setBusy(true);
    setOffline(false);
    const tags = tag.trim() === '' ? [] : [tag.trim()];
    const done: FileResult[] = files.map((file) => ({ name: file.name, result: 'pending' }));
    setResults([...done]);
    for (const [index, file] of files.entries()) {
      try {
        done[index] = { name: file.name, result: await importLibraryDemo(file, tags) };
      } catch (error) {
        done[index] = { name: file.name, result: 'error' };
        if (isLibraryOffline(error)) setOffline(true);
      }
      setResults([...done]);
      onImported();
    }
    setFiles([]);
    setBusy(false);
  }

  function close(next: boolean): void {
    if (busy) return;
    setOpen(next);
    if (!next) {
      setFiles([]);
      setResults([]);
      setTag('');
    }
  }

  const imported = results.filter((entry) => entry.result !== 'pending' && entry.result !== 'error' && entry.result.status === 'imported').length;

  return (
    <>
      <Button type="button" size="sm" onClick={() => setOpen(true)}>
        <FileUp aria-hidden />
        Importar demos
      </Button>
      <Dialog open={open} onOpenChange={close}>
        <DialogContent className="sm:max-w-xl" showCloseButton={!busy}>
          <DialogHeader>
            <DialogTitle>Importar demos</DialogTitle>
            <DialogDescription>
              Demos de CS2 en .dem o .dem.zst. Una demo que ya está en la biblioteca no se duplica. Cada una se analiza en este PC al llegar.
            </DialogDescription>
          </DialogHeader>

          <div
            onDragOver={(event) => {
              event.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={drop}
            className={cn(
              'flex flex-col items-center gap-2 rounded-md border border-dashed border-border-strong px-4 py-6 text-center',
              dragging && 'border-primary bg-primary/5',
            )}
          >
            <p className="text-body-sm text-fg-2">Arrastra aquí las demos o elígelas en el PC.</p>
            <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => input.current?.click()}>
              Elegir archivos
            </Button>
            <input
              ref={input}
              type="file"
              multiple
              accept=".dem,.zst"
              className="sr-only"
              tabIndex={-1}
              aria-label="Demos para importar"
              onChange={(event) => {
                choose(event.target.files);
                event.target.value = '';
              }}
            />
            {files.length > 0 ? (
              <p className="text-body-sm font-medium text-fg-1">
                {files.length === 1 ? '1 demo elegida' : `${files.length} demos elegidas`}
              </p>
            ) : null}
          </div>

          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Etiqueta (opcional)
            <Input value={tag} maxLength={60} disabled={busy} onChange={(event) => setTag(event.target.value)} placeholder="Rival: Vitality" />
          </label>

          {results.length > 0 ? (
            <ul aria-label="Resultado de la importación" className="flex max-h-64 flex-col divide-y divide-border-subtle overflow-y-auto rounded-md border border-border">
              {results.map((entry, index) => (
                <ImportResultRow key={`${entry.name}-${index}`} entry={entry} />
              ))}
            </ul>
          ) : null}
          {offline ? (
            <p role="alert" className="text-body-sm text-destructive">
              El servicio local no responde. Comprueba que ClipHub Studio sigue abierto.
            </p>
          ) : null}

          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => close(false)}>
              {results.length > 0 && !busy ? 'Cerrar' : 'Cancelar'}
            </Button>
            <Button type="button" disabled={busy || files.length === 0} onClick={() => void run()}>
              {busy ? `Importando ${results.filter((entry) => entry.result !== 'pending').length + 1} de ${results.length}` : 'Importar'}
            </Button>
          </DialogFooter>
          {imported > 0 && !busy ? (
            <p role="status" className="text-body-sm text-fg-2">
              {imported === 1 ? '1 demo nueva en cola de análisis.' : `${imported} demos nuevas en cola de análisis.`}
            </p>
          ) : null}
        </DialogContent>
      </Dialog>
    </>
  );
}

function ImportResultRow({ entry }: { entry: FileResult }): ReactNode {
  let status: ReactNode;
  let detail: ReactNode = null;
  if (entry.result === 'pending') {
    status = <StatusTag tone="neutral">En espera</StatusTag>;
  } else if (entry.result === 'error') {
    status = <StatusTag tone="danger">Error</StatusTag>;
    detail = 'No se pudo enviar el archivo.';
  } else {
    const tag = RESULT_TAG[entry.result.status];
    status = <StatusTag tone={tag.tone}>{tag.label}</StatusTag>;
    const matchId = entry.result.demo?.match_id;
    if (entry.result.status === 'rejected') detail = importErrorLabel(entry.result.error_code);
    else if (entry.result.status === 'duplicate' && matchId) {
      detail = (
        <Link href={matchHref(matchId)} className="text-primary hover:underline">
          Abrir la partida
        </Link>
      );
    }
  }
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2 text-body-sm">
      <span className="min-w-0 flex-1 truncate font-mono text-meta text-fg-1">{entry.name}</span>
      {detail ? <span className="text-meta text-fg-2">{detail}</span> : null}
      {status}
    </li>
  );
}
