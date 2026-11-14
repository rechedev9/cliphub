'use client';

import { useEffect, useRef, useState, useSyncExternalStore, type ChangeEvent, type DragEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Upload } from 'lucide-react';
import { WebImportRow } from '@/components/clipmind/web-import-row';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { emptyQueue, summarize, type PauseReason, type QueueState, type QueueSummary } from '@/lib/browser-import/import-queue';
import { ImportRunner, type AddReport } from '@/lib/browser-import/import-runner';
import { scanDemoInBrowser } from '@/lib/browser-import/scan-demo';
import { uploadAnalysis } from '@/lib/browser-import/upload-analysis';
import { MATCHES_HREF } from '@/lib/clips/routes';
import { countOf } from '@/lib/records/selection';

const PATTERNS_HREF = '/patrones';

const PAUSE_TEXT: Record<PauseReason, string> = {
  auth: 'Tu sesión ha terminado y la importación está en pausa. Entra de nuevo en otra pestaña y pulsa Reanudar. No se pierde nada.',
  offline: 'Se ha cortado la conexión y la importación está en pausa. Seguirá sola cuando vuelva la conexión, o pulsa Reanudar.',
  unavailable: 'ClipMind no responde ahora mismo y la importación está en pausa. Pulsa Reanudar en unos minutos; las demos ya analizadas no se pierden.',
};

let shared: ImportRunner | undefined;
let guarding = false;

function warnBeforeLeaving(event: BeforeUnloadEvent): void {
  event.preventDefault();
  event.returnValue = '';
}

// The queue lives for the tab, not the page: the coach can open Partidas while it runs.
function sharedRunner(): ImportRunner {
  if (shared !== undefined) return shared;
  const runner = new ImportRunner({ scan: scanDemoInBrowser, upload: uploadAnalysis, newId: () => crypto.randomUUID() });
  runner.subscribe(() => {
    const pending = summarize(runner.getSnapshot()).pending > 0;
    if (pending === guarding) return;
    guarding = pending;
    if (pending) window.addEventListener('beforeunload', warnBeforeLeaving);
    else window.removeEventListener('beforeunload', warnBeforeLeaving);
  });
  window.addEventListener('online', () => {
    if (runner.getSnapshot().paused === 'offline') runner.resume();
  });
  shared = runner;
  return runner;
}

function subscribe(listener: () => void): () => void {
  return sharedRunner().subscribe(listener);
}

function snapshot(): QueueState {
  return sharedRunner().getSnapshot();
}

function serverSnapshot(): QueueState {
  return emptyQueue;
}

/** Queues the demos and returns what the coach should hear about the rest, if anything. */
function enqueue(files: readonly File[]): AddReport | undefined {
  if (files.length === 0) return undefined;
  const report = sharedRunner().add(files);
  return report.unsupported.length > 0 || report.repeated.length > 0 ? report : undefined;
}

function retryOne(id: string): void {
  sharedRunner().retry(id);
}

function removeOne(id: string): void {
  sharedRunner().remove(id);
}

function hasFiles(event: globalThis.DragEvent): boolean {
  return event.dataTransfer?.types.includes('Files') === true;
}

function listSentence(parts: readonly string[]): string {
  if (parts.length < 2) return parts.join('');
  return `${parts.slice(0, -1).join(', ')} y ${parts.at(-1)}`;
}

function summaryLine(summary: QueueSummary): string {
  const parts = [
    summary.imported > 0 ? countOf(summary.imported, { one: 'partida nueva', many: 'partidas nuevas', feminine: true }) : '',
    summary.duplicate > 0 ? countOf(summary.duplicate, { one: 'ya estaba en la biblioteca', many: 'ya estaban en la biblioteca', feminine: true }) : '',
    summary.failed > 0 ? `${summary.failed} con error` : '',
  ].filter((part) => part !== '');
  return `${listSentence(parts)}.`;
}

function namesSentence(names: readonly string[]): string {
  const shown = names.slice(0, 3);
  const rest = names.length - shown.length;
  return rest > 0 ? `${shown.join(', ')} y ${rest} más` : listSentence(shown);
}

function IgnoredNotice({ report, onClose }: { report: AddReport; onClose: () => void }): ReactNode {
  return (
    <div role="status" className="flex items-start justify-between gap-4 rounded-md border border-border-strong bg-surface-3 px-4 py-3">
      <div className="flex flex-col gap-1 text-body-sm text-fg-2">
        {report.unsupported.length > 0 ? (
          <p>
            {report.unsupported.length === 1 ? 'No se añadió 1 archivo' : `No se añadieron ${report.unsupported.length} archivos`} porque no son demos:{' '}
            <span className="text-fg-1">{namesSentence(report.unsupported)}</span>. Se admiten .dem, .dem.zst y .dem.gz; si la demo viene en .rar o .zip,
            descomprímela antes.
          </p>
        ) : null}
        {report.repeated.length > 0 ? (
          <p>
            {report.repeated.length === 1 ? 'Ya estaba en la lista' : 'Ya estaban en la lista'}: <span className="text-fg-1">{namesSentence(report.repeated)}</span>.
          </p>
        ) : null}
      </div>
      <Button type="button" size="xs" variant="ghost" onClick={onClose}>
        Cerrar
      </Button>
    </div>
  );
}

function FinishedPanel({ summary }: { summary: QueueSummary }): ReactNode {
  return (
    <section aria-labelledby="web-import-finished" className="studio-panel flex flex-col gap-3 px-4 py-4">
      <h2 id="web-import-finished" className="text-body-lg font-semibold text-fg-1">
        {summary.failed > 0 ? 'Importación terminada, con errores' : 'Importación terminada'}
      </h2>
      <p className="text-body-sm text-fg-2">{summaryLine(summary)}</p>
      <div className="flex flex-wrap gap-2">
        <Button asChild>
          <Link href={MATCHES_HREF}>Ver partidas</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href={PATTERNS_HREF}>Abrir el Buscador de patrones</Link>
        </Button>
        {summary.failed > 0 ? (
          <Button type="button" variant="outline" onClick={() => sharedRunner().retryAllFailed()}>
            {summary.failed === 1 ? 'Reintentar la que falló' : `Reintentar las ${summary.failed} que fallaron`}
          </Button>
        ) : null}
      </div>
    </section>
  );
}

/**
 * The web edition's bulk import. The browser reads each demo (.dem, .dem.zst or
 * .dem.gz) with the scanner in WebAssembly and sends only the analysis.
 */
export function WebImport(): ReactNode {
  const router = useRouter();
  const input = useRef<HTMLInputElement>(null);
  const queue = useSyncExternalStore(subscribe, snapshot, serverSnapshot);
  const [ignored, setIgnored] = useState<AddReport | undefined>();
  const [over, setOver] = useState(false);
  const summary = summarize(queue);
  const running = summary.pending > 0;
  const wasRunning = useRef(running);

  useEffect(() => {
    if (wasRunning.current && !running) router.refresh();
    wasRunning.current = running;
  }, [running, router]);

  // A demo dropped next to the zone is still a demo to import, not a file for the browser to open.
  useEffect(() => {
    function onWindowDragOver(event: globalThis.DragEvent): void {
      if (hasFiles(event)) event.preventDefault();
    }
    function onWindowDrop(event: globalThis.DragEvent): void {
      if (event.defaultPrevented || !hasFiles(event)) return;
      event.preventDefault();
      setIgnored(enqueue(Array.from(event.dataTransfer?.files ?? [])));
    }
    window.addEventListener('dragover', onWindowDragOver);
    window.addEventListener('drop', onWindowDrop);
    return () => {
      window.removeEventListener('dragover', onWindowDragOver);
      window.removeEventListener('drop', onWindowDrop);
    };
  }, []);

  function onPick(event: ChangeEvent<HTMLInputElement>): void {
    const files = Array.from(event.target.files ?? []);
    event.target.value = '';
    setIgnored(enqueue(files));
  }

  function onDrop(event: DragEvent<HTMLDivElement>): void {
    event.preventDefault();
    setOver(false);
    setIgnored(enqueue(Array.from(event.dataTransfer.files)));
  }

  function onDragLeave(event: DragEvent<HTMLDivElement>): void {
    const next = event.relatedTarget;
    if (next instanceof Node && event.currentTarget.contains(next)) return;
    setOver(false);
  }

  const progressText = `${summary.finished} de ${countOf(summary.total, { one: 'demo terminada', many: 'demos terminadas', feminine: true })}`;

  return (
    <div className="measure-work flex flex-col gap-6">
      <div
        onDragOver={(event) => {
          event.preventDefault();
          setOver(true);
        }}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
        className={`flex flex-col items-center gap-3 rounded-md border border-dashed px-6 py-10 text-center transition-colors duration-(--dur-fast) ${over ? 'border-primary bg-primary/8' : 'border-border-strong'}`}
      >
        <Upload aria-hidden className="size-6 stroke-[1.5] text-fg-3" />
        <p className="text-body-sm text-fg-1">Suelta aquí tus demos o elígelas</p>
        <p className="measure-read text-body-sm text-fg-3">
          Admite .dem, .dem.zst de FACEIT y .dem.gz, todas las que quieras a la vez. Tu navegador analiza cada demo y solo envía el análisis: el archivo no
          sale de tu ordenador.
        </p>
        <input
          ref={input}
          type="file"
          accept=".dem,.zst,.gz"
          multiple
          tabIndex={-1}
          className="sr-only"
          aria-label="Demos para importar"
          onChange={onPick}
        />
        <Button type="button" variant="outline" onClick={() => input.current?.click()}>
          {summary.total > 0 ? 'Añadir más demos' : 'Elegir demos'}
        </Button>
      </div>

      {ignored === undefined ? null : <IgnoredNotice report={ignored} onClose={() => setIgnored(undefined)} />}

      {queue.paused === undefined ? null : (
        <div role="alert" className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-warning/45 bg-warning/10 px-4 py-3">
          <p className="measure-read text-body-sm text-fg-1">{PAUSE_TEXT[queue.paused]}</p>
          <Button type="button" size="sm" variant="outline" onClick={() => sharedRunner().resume()}>
            Reanudar
          </Button>
        </div>
      )}

      <p role="status" className="sr-only">
        {running ? `${progressText}.` : null}
        {summary.total > 0 && !running ? `Importación terminada: ${summaryLine(summary)}` : null}
      </p>

      {summary.total > 0 && !running ? <FinishedPanel summary={summary} /> : null}

      {summary.total > 0 ? (
        <section aria-labelledby="web-import-queue" className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-baseline gap-3">
              <h2 id="web-import-queue" className="text-body-lg font-semibold text-fg-1">
                Demos
              </h2>
              <span className="text-body-sm tabular-nums text-fg-2">{progressText}</span>
            </div>
            <div className="flex flex-wrap gap-2">
              {running && summary.failed > 0 ? (
                <Button type="button" size="sm" variant="outline" onClick={() => sharedRunner().retryAllFailed()}>
                  Reintentar las que fallaron
                </Button>
              ) : null}
              {summary.imported + summary.duplicate > 0 ? (
                <Button type="button" size="sm" variant="ghost" onClick={() => sharedRunner().clearFinished()}>
                  Quitar las terminadas
                </Button>
              ) : null}
            </div>
          </div>
          {running ? (
            <div className="flex flex-col gap-1.5">
              <Progress size="sm" value={(summary.finished / summary.total) * 100} aria-label="Progreso de la importación" />
              <p className="text-meta text-fg-3">Puedes seguir usando ClipMind mientras tanto. No cierres ni recargues esta pestaña hasta que termine.</p>
            </div>
          ) : null}
          <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
            {queue.items.map((item) => (
              <WebImportRow key={item.id} item={item} onRetry={retryOne} onRemove={removeOne} />
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  );
}
