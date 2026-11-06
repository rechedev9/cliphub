'use client';

import { useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { PackageOpen } from 'lucide-react';
import { isLibraryOffline, LibraryApiError } from '@/lib/api/library';
import { importSharePackage } from '@/lib/api/share-package';
import { collectionHref, reportHref } from '@/lib/clips/routes';
import { SHARE_FILE, SHARE_PACKAGE_EXTENSION, shareErrorLabel, shareImportSummary, type SharePackageImport } from '@/lib/library/share';
import { Button } from '@/components/ui/button';

/** The page a finished import opens: the collection or report it brought. */
export function importedHref(result: SharePackageImport): string | null {
  if (result.collection_id) return collectionHref(result.collection_id);
  if (result.report_id) return reportHref(result.report_id);
  return null;
}

/** The sentence a failed package import shows. */
export function packageImportError(failure: unknown): string {
  if (isLibraryOffline(failure)) return 'El servicio local no responde. Comprueba que ClipHub Studio sigue abierto.';
  const code = failure instanceof LibraryApiError ? failure.code : undefined;
  return shareErrorLabel(code, 'No se pudo importar el paquete.');
}

/** "Importar paquete": opens a .cliphub file a teammate shared. */
export function PackageImportButton({ onImported }: { onImported?: () => void }): ReactNode {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<SharePackageImport | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function run(file: File | undefined): Promise<void> {
    if (file === undefined) return;
    setResult(null);
    if (!SHARE_FILE.test(file.name)) {
      setError(`Elige un archivo ${SHARE_PACKAGE_EXTENSION}.`);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      setResult(await importSharePackage(file));
      onImported?.();
    } catch (failure) {
      setError(packageImportError(failure));
    } finally {
      setBusy(false);
    }
  }

  const href = result === null ? null : importedHref(result);
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
      <Button type="button" size="sm" variant="outline" loading={busy} loadingText="Importando" onClick={() => input.current?.click()}>
        <PackageOpen aria-hidden />
        Importar paquete
      </Button>
      <input
        ref={input}
        type="file"
        accept={SHARE_PACKAGE_EXTENSION}
        className="sr-only"
        tabIndex={-1}
        aria-label="Paquete para importar"
        onChange={(event) => {
          void run(event.target.files?.[0]);
          event.target.value = '';
        }}
      />
      {result !== null ? (
        <p role="status" className="text-body-sm text-fg-2">
          {shareImportSummary(result)}{' '}
          {href ? (
            <Link href={href} className="text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1">
              Abrir
            </Link>
          ) : null}
        </p>
      ) : null}
      {error ? (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  );
}
