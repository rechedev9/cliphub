'use client';

import { useState, type FormEvent, type ReactElement } from 'react';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { importShareCode, type SteamImportResult } from '@/lib/api/steam-import';

export type SteamDownloadDialogProps = {
  open: boolean;
  code: string;
  onOpenChange: (open: boolean) => void;
  onQueued: (jobId: string) => void;
};

export function SteamDownloadDialog({
  open,
  code,
  onOpenChange,
  onQueued,
}: SteamDownloadDialogProps): ReactElement {
  const [error, setError] = useState<string | undefined>();
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const username = String(form.get('username') ?? '').trim();
    const password = String(form.get('password') ?? '');
    const guard = String(form.get('guard') ?? '').trim();
    if (username === '' || password === '' || guard === '') {
      setError('Haz falta usuario, contraseña y Steam Guard.');
      return;
    }
    setPending(true);
    setError(undefined);
    const result = await importShareCode(code, { username, password, guard });
    setPending(false);
    handleResult(result);
  }

  function handleResult(result: SteamImportResult): void {
    if (result.kind === 'queued') {
      onOpenChange(false);
      onQueued(result.id);
      return;
    }
    if (result.kind === 'offline') {
      setError('El servicio local de ClipHub no está en marcha.');
      return;
    }
    if (result.kind === 'needCredentials') {
      setError('Steam no aceptó esa sesión. Revisa usuario, contraseña y Steam Guard.');
      return;
    }
    setError(result.message);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="uppercase">Descargar esta demo</DialogTitle>
          <DialogDescription>
            Valve solo entrega la demo si ClipHub abre una sesión corta con la cuenta con la que
            juegas. Si estás en una partida de CS2, Steam te echará de ella.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={(event) => { void onSubmit(event); }} className="flex flex-col gap-4">
          <Field label="Usuario de Steam" error={error}>
            {(control) => (
              <Input {...control} name="username" autoComplete="off" spellCheck={false} />
            )}
          </Field>
          <Field label="Contraseña">
            {(control) => (
              <Input {...control} name="password" type="password" autoComplete="off" />
            )}
          </Field>
          <Field label="Steam Guard" hint="El código de la app o el correo, solo para esta descarga.">
            {(control) => (
              <Input {...control} name="guard" autoComplete="one-time-code" spellCheck={false} />
            )}
          </Field>
          <DialogFooter>
            <Button type="submit" loading={pending} loadingText="CONECTANDO">
              DESCARGAR
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
