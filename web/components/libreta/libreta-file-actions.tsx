'use client';

import { useRef, useState, type ChangeEvent, type ReactNode } from 'react';
import { FileDown, FileUp } from 'lucide-react';
import { exportLibreta } from '@/lib/api/library';
import { importLibretaText } from '@/lib/libreta/libreta-file';
import { stratbookFileName } from '@/lib/stratbook/plays';
import { Button } from '@/components/ui/button';
import { downloadJson } from './play-list';

/** The whole Libreta as one file, out and back in: plays, lineups, role sheets, calls and callout names. */
export function LibretaFileActions({ onImported }: { onImported: () => void }): ReactNode {
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  async function exportAll(): Promise<void> {
    setBusy(true);
    setMessage(null);
    try {
      const name = stratbookFileName(new Date());
      downloadJson(await exportLibreta(), name);
      setMessage(`Libreta exportada en ${name}.`);
    } catch {
      setMessage('No se pudo exportar la Libreta. Comprueba que ClipHub Studio sigue abierto.');
    } finally {
      setBusy(false);
    }
  }

  async function importFile(event: ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (file === undefined) return;
    setBusy(true);
    setMessage(null);
    setMessage(await importLibretaText(await file.text()));
    setBusy(false);
    onImported();
  }

  return (
    <div className="flex flex-col items-start gap-2 lg:items-end">
      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => fileRef.current?.click()}>
          <FileUp aria-hidden />
          Importar libreta
        </Button>
        <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void exportAll()}>
          <FileDown aria-hidden />
          Exportar libreta
        </Button>
        <input
          ref={fileRef}
          type="file"
          accept=".json,application/json"
          aria-label="Fichero de la Libreta entera"
          className="sr-only"
          tabIndex={-1}
          onChange={(event) => void importFile(event)}
        />
      </div>
      {message ? (
        <p role="status" className="max-w-80 text-body-sm text-fg-2 lg:text-right">
          {message}
        </p>
      ) : null}
    </div>
  );
}
