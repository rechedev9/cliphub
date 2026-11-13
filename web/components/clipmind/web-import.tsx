'use client';

import { useRef, useState, type ChangeEvent, type DragEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { scanDemoInBrowser } from '@/lib/browser-import/scan-demo';

type Entry = {
  key: string;
  name: string;
  phase: 'reading' | 'sending' | 'done' | 'failed';
  percent: number;
  matchId?: string;
  already?: boolean;
  error?: string;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function uploadAnswer(value: unknown): { matchId: string; created: boolean } | undefined {
  if (!isRecord(value) || !isRecord(value.match)) return undefined;
  const { id } = value.match;
  return typeof id === 'string' ? { matchId: id, created: value.created === true } : undefined;
}

function doneLabel(entry: Entry): string {
  return entry.already === true ? 'Ya estaba' : 'Lista';
}

async function describeFailure(response: Response): Promise<string> {
  if (response.status === 401) return 'Tu sesión ha terminado. Entra de nuevo.';
  if (response.status === 413) return 'El análisis es demasiado grande.';
  const body: unknown = await response.json().catch(() => undefined);
  const detail = isRecord(body) && typeof body.error === 'string' ? body.error : '';
  return detail === '' ? 'No se pudo guardar la partida.' : `No se pudo guardar la partida: ${detail}`;
}

/**
 * The web edition's import. The browser reads the .dem with the scanner in
 * WebAssembly and sends only the analysis; the demo never leaves this computer.
 */
export function WebImport(): ReactNode {
  const router = useRouter();
  const input = useRef<HTMLInputElement>(null);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [over, setOver] = useState(false);
  const busy = entries.some((entry) => entry.phase === 'reading' || entry.phase === 'sending');

  function patch(key: string, change: Partial<Entry>): void {
    setEntries((current) => current.map((entry) => (entry.key === key ? { ...entry, ...change } : entry)));
  }

  async function importOne(file: File, key: string): Promise<void> {
    const scanned = await scanDemoInBrowser(file, {
      onProgress: (read, total) => patch(key, { percent: total > 0 ? Math.round((read / total) * 100) : 0 }),
    }).catch((error: unknown) => ({ ok: false as const, error: error instanceof Error ? error.message : 'No se pudo leer la demo.' }));
    if (!scanned.ok) {
      patch(key, { phase: 'failed', error: `No se pudo leer esta demo: puede estar dañada o no ser de CS2. (${scanned.error})` });
      return;
    }
    patch(key, { phase: 'sending', percent: 100 });
    const form = new FormData();
    form.set('document', new Blob([scanned.document], { type: 'application/json' }), 'document.json');
    form.set('positions', new Blob([new Uint8Array(scanned.positions)], { type: 'application/octet-stream' }), 'positions.zvpos');
    form.set('file_name', file.name);
    form.set('size_bytes', String(file.size));
    form.set('cs2_build', String(scanned.cs2Build));
    try {
      // The upload skips the proxy that renews the sign-in; a scan can outlast the access token.
      await fetch('/api/cm/auth/me', { cache: 'no-store' });
      const response = await fetch('/api/library/uploads', { method: 'POST', body: form });
      if (!response.ok) {
        patch(key, { phase: 'failed', error: await describeFailure(response) });
        return;
      }
      const answer = uploadAnswer(await response.json());
      patch(key, answer === undefined ? { phase: 'failed', error: 'Respuesta inesperada del servidor.' } : { phase: 'done', matchId: answer.matchId, already: !answer.created });
    } catch {
      patch(key, { phase: 'failed', error: 'No se pudo contactar con ClipMind.' });
    }
  }

  async function start(files: File[]): Promise<void> {
    const demos = files.filter((file) => file.name.toLowerCase().endsWith('.dem'));
    if (demos.length === 0) return;
    const added = demos.map((file, index) => ({ file, key: `${Date.now()}-${index}-${file.name}` }));
    setEntries((current) => [
      ...added.map(({ file, key }): Entry => ({ key, name: file.name, phase: 'reading', percent: 0 })),
      ...current,
    ]);
    // One at a time: a scan is memory heavy and the next one starts when the last is stored.
    for (const { file, key } of added) await importOne(file, key);
    router.refresh();
  }

  function onPick(event: ChangeEvent<HTMLInputElement>): void {
    const files = Array.from(event.target.files ?? []);
    event.target.value = '';
    void start(files);
  }

  function onDrop(event: DragEvent): void {
    event.preventDefault();
    setOver(false);
    if (!busy) void start(Array.from(event.dataTransfer.files));
  }

  return (
    <div className="flex flex-col gap-6">
      <div
        onDragOver={(event) => {
          event.preventDefault();
          setOver(true);
        }}
        onDragLeave={() => setOver(false)}
        onDrop={onDrop}
        className={`flex flex-col items-center gap-3 rounded-md border border-dashed px-6 py-10 text-center ${over ? 'border-primary bg-primary/8' : 'border-border-strong'}`}
      >
        <Upload aria-hidden className="size-6 stroke-[1.5] text-fg-3" />
        <p className="text-body-sm text-fg-1">Suelta aquí tus demos (.dem) o elígelas</p>
        <p className="measure-read text-body-sm text-fg-3">
          Tu navegador lee la demo y solo envía el análisis del equipo. El archivo no sale de tu ordenador.
        </p>
        <input ref={input} type="file" accept=".dem" multiple className="sr-only" aria-label="Elegir demos" onChange={onPick} />
        <Button type="button" variant="outline" disabled={busy} onClick={() => input.current?.click()}>
          Elegir demos
        </Button>
      </div>

      {entries.length > 0 ? (
        <ul className="flex flex-col divide-y divide-border rounded-md border border-border" aria-live="polite">
          {entries.map((entry) => (
            <li key={entry.key} className="flex flex-col gap-2 px-4 py-3">
              <div className="flex items-center justify-between gap-4">
                <span className="min-w-0 truncate text-body-sm text-fg-1">{entry.name}</span>
                <span className="shrink-0 text-body-sm text-fg-3">
                  {entry.phase === 'reading' ? `Leyendo ${entry.percent}%` : null}
                  {entry.phase === 'sending' ? 'Guardando' : null}
                  {entry.phase === 'done' ? doneLabel(entry) : null}
                  {entry.phase === 'failed' ? 'Falló' : null}
                </span>
              </div>
              {entry.phase === 'reading' ? <Progress value={entry.percent} aria-label={`Leyendo ${entry.name}`} /> : null}
              {entry.phase === 'done' && entry.matchId !== undefined ? (
                <Link href={`/partidas/${entry.matchId}`} className="text-body-sm text-primary underline-offset-4 hover:underline">
                  Abrir la partida
                </Link>
              ) : null}
              {entry.phase === 'failed' ? (
                <p role="alert" className="text-body-sm text-destructive">
                  {entry.error}
                </p>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
