'use client';

import { useState, type ReactNode } from 'react';
import { Share2 } from 'lucide-react';
import { isLibraryOffline, LibraryApiError } from '@/lib/api/library';
import { exportSharePackage, savePackageFile, type SharePackageFile, type ShareTarget } from '@/lib/api/share-package';
import { SHARE_NOTES_MAX, packageSizeLabel, shareErrorLabel } from '@/lib/library/share';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Textarea } from '@/components/ui/textarea';

type SharePackageButtonProps = {
  target: ShareTarget;
  /** What the file will carry, from `reportShareSummary`. */
  summary: string;
  disabled?: boolean;
};

/** "Compartir": builds a .cliphub file another Studio opens without the demos, and saves it. */
export function SharePackageButton({ target, summary, disabled = false }: SharePackageButtonProps): ReactNode {
  const [open, setOpen] = useState(false);
  const [notes, setNotes] = useState('');
  const [busy, setBusy] = useState(false);
  const [file, setFile] = useState<SharePackageFile | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function build(): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      const built = await exportSharePackage(target, notes.trim());
      setFile(built);
      savePackageFile(built);
    } catch (failure) {
      const code = failure instanceof LibraryApiError ? failure.code : undefined;
      setError(isLibraryOffline(failure) ? 'El servicio local no responde. Vuelve a intentarlo.' : shareErrorLabel(code, 'No se pudo crear el paquete.'));
    } finally {
      setBusy(false);
    }
  }

  function close(next: boolean): void {
    if (busy) return;
    setOpen(next);
    if (!next) {
      setFile(null);
      setError(null);
      setNotes('');
    }
  }

  return (
    <>
      <Button type="button" size="sm" variant="outline" disabled={disabled} onClick={() => setOpen(true)}>
        <Share2 aria-hidden />
        Compartir
      </Button>
      <Dialog open={open} onOpenChange={close}>
        <DialogContent className="sm:max-w-lg" showCloseButton={!busy}>
          <DialogHeader>
            <DialogTitle>Compartir informe</DialogTitle>
            <DialogDescription>
              {summary} Quien lo abra con Importar demos, en Partidas de su Studio, ve las repeticiones sin conexión.
            </DialogDescription>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Mensaje para el equipo (opcional)
            <Textarea
              value={notes}
              maxLength={SHARE_NOTES_MAX}
              disabled={busy || file !== null}
              onChange={(event) => setNotes(event.target.value)}
            />
          </label>
          {file !== null ? (
            <p role="status" className="text-body-sm text-fg-1">
              <span className="font-mono text-meta">{file.fileName}</span>, {packageSizeLabel(file.blob.size)}. Envíalo por el canal que uséis.
            </p>
          ) : null}
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={() => close(false)}>
              {file === null ? 'Cancelar' : 'Cerrar'}
            </Button>
            {file === null ? (
              <Button type="button" loading={busy} loadingText="Creando paquete" onClick={() => void build()}>
                Crear y guardar
              </Button>
            ) : (
              <Button type="button" onClick={() => savePackageFile(file)}>
                Guardar otra vez
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
