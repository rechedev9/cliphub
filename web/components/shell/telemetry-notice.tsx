'use client';

import { useEffect, useState, type ReactElement } from 'react';
import { Activity, ShieldCheck } from 'lucide-react';
import { getDesktopSettingsBridge, type StudioTelemetryStatus } from '@/lib/desktop-settings';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

/** One-time informed choice shown before any remote diagnostic can be sent. */
export function TelemetryNotice(): ReactElement | null {
  const [status, setStatus] = useState<StudioTelemetryStatus | null>(null);
  const [pending, setPending] = useState<boolean | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const bridge = getDesktopSettingsBridge();
    if (bridge === null) return;
    void bridge.getTelemetry().then(setStatus).catch(() => setFailed(true));
  }, []);

  if (status === null || !status.available || status.noticeAcknowledged) return null;

  const choose = (enabled: boolean): void => {
    const bridge = getDesktopSettingsBridge();
    if (bridge === null) return;
    setPending(enabled);
    setFailed(false);
    void bridge.updateTelemetry(enabled)
      .then(setStatus)
      .catch(() => setFailed(true))
      .finally(() => setPending(null));
  };

  return (
    <Dialog open>
      <DialogContent showCloseButton={false} aria-describedby="telemetry-notice-description">
        <DialogHeader>
          <div className="mb-2 flex items-center gap-3 text-primary">
            <span className="grid size-10 place-items-center rounded-md border border-primary/50 bg-surface-3">
              <Activity className="size-5" aria-hidden />
            </span>
            <span className="font-mono text-meta uppercase tracking-wider">Diagnóstico local-first</span>
          </div>
          <DialogTitle>Ayuda a detectar fallos de ClipHub</DialogTitle>
          <DialogDescription id="telemetry-notice-description">
            ClipHub puede enviar automáticamente errores estructurados y una muestra del 10 % de los tiempos de
            ejecución. Los usamos para encontrar fallos por versión y se eliminan a los 30 días.
          </DialogDescription>
        </DialogHeader>

        <div className="rounded-md border border-border bg-surface-3 p-4 text-body-sm text-fg-2">
          <p className="flex items-start gap-2">
            <ShieldCheck className="mt-0.5 size-4 shrink-0 text-success" aria-hidden />
            No enviamos demos, vídeos, rutas, SteamID, nombres de jugadores, credenciales, prompts ni contenido
            multimedia. Puedes desactivarlo después en Configuración.
          </p>
          <p className="mt-3 font-mono text-meta tracking-wider text-fg-3">
            CÓDIGO DE SOPORTE · {status.supportCode}
          </p>
        </div>

        {failed ? (
          <p role="alert" className="text-body-sm text-destructive">
            No se pudo guardar tu elección. No se enviará nada hasta poder guardarla.
          </p>
        ) : null}

        <DialogFooter>
          <Button
            type="button"
            variant="ghost"
            loading={pending === false}
            loadingText="DESACTIVANDO"
            onClick={() => choose(false)}
          >
            Desactivar
          </Button>
          <Button
            type="button"
            loading={pending === true}
            loadingText="GUARDANDO"
            onClick={() => choose(true)}
          >
            Mantener activado
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
