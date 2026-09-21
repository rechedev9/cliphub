'use client';

import type { ReactNode } from 'react';
import type { FullDemoBumperOptions, FullDemoDocument, FullDemoOptions } from '@/lib/full-demo-plan';
import { FullDemoAssetInput } from './full-demo-asset-input';
import { FullDemoMediaPreview } from './full-demo-media-preview';
import { FullDemoToggle } from './full-demo-fields';

type Props = { options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void };
type Slot = keyof FullDemoBumperOptions;

const EMPTY_BUMPERS: FullDemoBumperOptions = { intro: { enabled: false, video: null }, outro: { enabled: false, video: null } };
const COPY: Record<Slot, { toggle: string; missing: string; preview: string; add: string; replace: string; where: string }> = {
  intro: { toggle: 'Incluir intro', missing: 'Añade el vídeo de la intro o desactívala.', preview: 'Previsualizar intro', add: 'Añadir vídeo de intro y permisos', replace: 'Cambiar vídeo de intro', where: 'Se reproduce antes del primer fotograma de la partida.' },
  outro: { toggle: 'Incluir outro', missing: 'Añade el vídeo de la outro o desactívala.', preview: 'Previsualizar outro', add: 'Añadir vídeo de outro y permisos', replace: 'Cambiar vídeo de outro', where: 'Se reproduce después de la última ronda.' },
};

/**
 * Pre-roll and outro clips around the program. They reuse the sponsor's
 * verified asset upload, but carry no placement policy: the planner puts the
 * intro at frame 0 and the outro after the last item, with the clip's own audio.
 */
export function FullDemoBumpers({ options, document, onChange, onAssetBusy }: Props): ReactNode {
  const bumpers = options.bumpers ?? EMPTY_BUMPERS;
  const assetName = (id: string | undefined): string => document?.assets?.find((asset) => asset.ref.id === id)?.title ?? 'Archivo pendiente de revisar en el plan';
  const change = (slot: Slot, patch: Partial<FullDemoBumperOptions[Slot]>): void => onChange({ ...options, bumpers: { ...bumpers, [slot]: { ...bumpers[slot], ...patch } } });
  return <div className="space-y-4">
    {(['intro', 'outro'] as const).map((slot, index) => {
      const value = bumpers[slot];
      const copy = COPY[slot];
      return <div key={slot} className={index > 0 ? 'space-y-3 border-t border-border-subtle pt-3' : 'space-y-3'}>
        <FullDemoToggle label={copy.toggle} value={value.enabled} onChange={(enabled) => change(slot, { enabled })} />
        {value.enabled ? <>
          <p className="text-meta text-fg-3">{copy.where} Suena el audio del propio clip.</p>
          {value.video ? <p className="text-body-sm text-fg-1">Vídeo: {assetName(value.video.id)}</p> : <p className="text-body-sm text-destructive">{copy.missing}</p>}
          {value.video ? <FullDemoMediaPreview asset={value.video} video label={copy.preview} gain={1} /> : null}
          <FullDemoAssetInput label={value.video ? copy.replace : copy.add} accept="video/*" onBusyChange={onAssetBusy} onUploaded={(video) => change(slot, { video })} />
        </> : null}
      </div>;
    })}
  </div>;
}
