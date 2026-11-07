'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { ChevronDown, RefreshCw, Trash2 } from 'lucide-react';
import {
  LIBRARY_SOURCES,
  analyzeLibraryDemo,
  deleteLibraryDemo,
  patchLibraryDemo,
  type LibraryDemo,
  type LibrarySource,
} from '@/lib/api/library';
import { MATCHES_HREF } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { PARSE_STATE_LABEL, SHARED_MATCH_REASON, SOURCE_LABEL, demoErrorLabel } from '@/lib/library/partidas';
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

/** A shared match keeps its source until its .dem is imported, so it is never offered. */
const EDITABLE_SOURCES = LIBRARY_SOURCES.filter((source) => source !== 'shared');

/** The demo a partida was read from, folded at the end of Resumen: its file, source, tags and upkeep. */
export function MatchDemoDetails({ demo, onDemoChange }: { demo: LibraryDemo; onDemoChange: (demo: LibraryDemo) => void }): ReactNode {
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
  const hint = demo.cs2_build === undefined ? 'Archivo, fuente y etiquetas' : `Archivo, build ${demo.cs2_build}, fuente y etiquetas`;

  return (
    <details className="group/demo studio-panel overflow-hidden">
      <summary
        className={`flex min-h-12 cursor-pointer list-none items-center gap-3 px-4 py-3 [&::-webkit-details-marker]:hidden ${FOCUS_RING}`}
      >
        <span className="text-body-sm font-semibold text-fg-1">Detalles de la demo</span>
        {demo.state === 'ready' ? null : (
          <Badge variant={demo.state === 'failed' ? 'danger' : 'secondary'}>{PARSE_STATE_LABEL[demo.state]}</Badge>
        )}
        <span className="min-w-0 grow truncate text-meta text-fg-3">{hint}</span>
        <ChevronDown aria-hidden className="size-4 shrink-0 text-fg-3 transition-transform group-open/demo:rotate-180" />
      </summary>
      <div className="flex flex-col gap-4 border-t border-border-subtle px-4 pt-3 pb-4">
        <dl className="grid gap-x-6 gap-y-3 text-body-sm sm:grid-cols-2 @[64rem]/content:grid-cols-4">
          <Fact label="Archivo">
            <span className="font-mono text-meta break-all">{demo.file_name}</span>
          </Fact>
          <Fact label="Huella SHA-256">
            <span className="font-mono text-meta" title={demo.sha256}>{demo.sha256.slice(0, 12)}</span>
          </Fact>
          <Fact label="Build de CS2">{demo.cs2_build ?? 'Sin dato'}</Fact>
          <Fact label="Importada">{formatShortDate(demo.imported_at)}</Fact>
          <Fact label="Estado">
            {PARSE_STATE_LABEL[demo.state]}
            {demo.state === 'failed' ? <span className="block text-meta text-destructive">{demoErrorLabel(demo.error_code)}</span> : null}
          </Fact>
          <Fact label="Fuente">
            {demo.source === 'shared' ? SOURCE_LABEL.shared : (
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
                  {EDITABLE_SOURCES.map((source: LibrarySource) => (
                    <SelectItem key={source} value={source}>
                      {SOURCE_LABEL[source]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            {demo.source_manual && demo.source !== 'shared' ? <span className="block text-meta text-fg-3">Elegida a mano</span> : null}
          </Fact>
        </dl>
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex min-w-0 grow basis-72 flex-col gap-1.5 text-meta text-fg-3 sm:max-w-md">
            Etiquetas, separadas por comas
            <Input value={tags} maxLength={400} disabled={busy} onChange={(event) => setTags(event.target.value)} className="h-8" />
          </label>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy || !tagsChanged}
            onClick={() => void act(async () => onDemoChange(await patchLibraryDemo(demo.id, { tags: tagList })), 'No se pudieron guardar las etiquetas.')}
          >
            Guardar etiquetas
          </Button>
          <span className="grow" />
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy || demo.state === 'queued' || demo.state === 'parsing' || demo.source === 'shared'}
            onClick={() => void act(async () => onDemoChange(await analyzeLibraryDemo(demo.id)), 'No se pudo poner la demo en cola.')}
          >
            <RefreshCw aria-hidden />
            Volver a analizar
          </Button>
          <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => setConfirmDelete(true)}>
            <Trash2 aria-hidden />
            Quitar de la biblioteca
          </Button>
        </div>
        {demo.source === 'shared' ? <p className="text-body-sm text-fg-3">{SHARED_MATCH_REASON}</p> : null}
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
    </details>
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
