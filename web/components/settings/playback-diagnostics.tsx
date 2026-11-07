'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { StudioDataRow } from '@/components/studio/data-row';
import { Skeleton } from '@/components/ui/skeleton';
import { SettingsSection } from './settings-section';
import { getDesktopSettingsBridge, type StudioPlaybackInfo } from '@/lib/desktop-settings';

const VIDEO_DECODE_LABEL: Record<Extract<StudioPlaybackInfo, { available: true }>['videoDecode'], string> = {
  'hardware-accelerated': 'Acelerada por hardware',
  'software-only': 'Solo software',
  unavailable: 'No disponible',
  unknown: 'Sin determinar',
};

export function PlaybackDiagnostics(): ReactNode {
  const [info, setInfo] = useState<StudioPlaybackInfo | null>(null);
  const [unavailable, setUnavailable] = useState(false);

  useEffect(() => {
    const bridge = getDesktopSettingsBridge();
    if (!bridge?.getPlaybackInfo) {
      setUnavailable(true);
      return;
    }
    void bridge.getPlaybackInfo().then(setInfo).catch(() => setUnavailable(true));
  }, []);

  let body: ReactNode;
  if (info?.available) {
    body = (
      <div className="flex flex-col divide-y divide-border border border-border bg-surface-2">
        <StudioDataRow label="Aceleración gráfica" value={info.hardwareAcceleration === 'enabled' ? 'Activada' : 'Desactivada'} className="border-0 bg-transparent" />
        <StudioDataRow label="Decodificación de vídeo" value={VIDEO_DECODE_LABEL[info.videoDecode]} className="border-0 bg-transparent" />
      </div>
    );
  } else if (info?.state === 'initializing') {
    body = (
      <p role="status" className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
        El motor de vídeo todavía se está iniciando. Vuelve a abrir Ajustes en unos segundos.
      </p>
    );
  } else if (unavailable) {
    body = (
      <p className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
        Este diagnóstico solo está disponible en una versión reciente de la app de escritorio.
      </p>
    );
  } else {
    body = (
      <div role="status" aria-label="Leyendo diagnóstico de reproducción" className="flex flex-col gap-px">
        <Skeleton className="h-11 w-full rounded-none" />
        <Skeleton className="h-11 w-full rounded-none" />
      </div>
    );
  }

  return (
    <SettingsSection
      id="playback"
      title="Motor de vídeo"
      description="Estado general del motor incluido. Un vídeo concreto puede usar otra ruta de decodificación."
    >
      {body}
    </SettingsSection>
  );
}
