'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Activity } from 'lucide-react';
import { getDesktopSettingsBridge, type StudioTelemetryStatus } from '@/lib/desktop-settings';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { IconTile } from '@/components/studio/icon-tile';
import { StudioDataRow } from '@/components/studio/data-row';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

export function TelemetrySettings(): ReactNode {
  const [status, setStatus] = useState<StudioTelemetryStatus | null>(null);
  const [unavailable, setUnavailable] = useState(false);
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState(false);

  useEffect(() => {
    const bridge = getDesktopSettingsBridge();
    if (bridge === null) {
      setUnavailable(true);
      return;
    }
    void bridge.getTelemetry().then(setStatus).catch(() => setUnavailable(true));
  }, []);

  const update = (enabled: boolean): void => {
    const bridge = getDesktopSettingsBridge();
    if (bridge === null) return;
    setPending(true);
    setFailure(false);
    void bridge.updateTelemetry(enabled)
      .then(setStatus)
      .catch(() => setFailure(true))
      .finally(() => setPending(false));
  };

  let body: ReactNode;
  if (status?.available) {
    body = (
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <StudioDataRow label="Estado" value={status.enabled ? 'Activado' : 'Desactivado'} />
          <StudioDataRow label="Código de soporte" value={status.supportCode} />
          <StudioDataRow label="Retención" value={`${status.retentionDays} días`} />
          <StudioDataRow label="Muestra de rendimiento" value={`${status.performanceSamplePercent} %`} />
        </div>
        <p className="text-body-sm text-fg-2">
          Se envían códigos de error estructurados y tiempos de ejecución. Nunca se incluyen demos, vídeos, rutas,
          SteamID, nombres de jugadores, credenciales, prompts ni multimedia.
        </p>
        {failure ? (
          <p role="alert" className="text-body-sm text-destructive">
            No se pudo guardar la preferencia. Inténtalo de nuevo.
          </p>
        ) : null}
        <div>
          <Button
            type="button"
            variant={status.enabled ? 'outline' : 'default'}
            loading={pending}
            loadingText="GUARDANDO"
            onClick={() => update(!status.enabled)}
          >
            {status.enabled ? 'Desactivar diagnósticos' : 'Activar diagnósticos'}
          </Button>
        </div>
      </div>
    );
  } else if (unavailable || status?.available === false) {
    body = (
      <p className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
        Los diagnósticos remotos solo están disponibles en una versión distribuida de la app de escritorio.
      </p>
    );
  } else {
    body = (
      <div role="status" aria-label="Leyendo configuración de diagnósticos" className="flex flex-col gap-2">
        <Skeleton className="h-11 w-full rounded-none" />
        <Skeleton className="h-11 w-full rounded-none" />
      </div>
    );
  }

  return (
    <section className="studio-panel flex flex-col gap-5 p-5 sm:p-6" aria-labelledby="telemetry-settings-title">
      <div className="flex items-center gap-4">
        <IconTile icon={Activity} size="md" depth="inset" />
        <div className="flex min-w-0 flex-col gap-1">
          <SectionEyebrow label="PRIVACIDAD" />
          <h2 id="telemetry-settings-title" className="font-display text-title font-bold uppercase text-fg-1">
            Diagnósticos
          </h2>
        </div>
      </div>
      {body}
    </section>
  );
}
