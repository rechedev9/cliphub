'use client';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Upload, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { uploadFullDemoBumper, type FullDemoBumperOptions, type FullDemoDocument, type FullDemoOptions, type FullDemoAssetRef } from '@/lib/full-demo-plan';
import { FullDemoMediaPreview } from './full-demo-media-preview';

type Props = { options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void };
type Slot = keyof FullDemoBumperOptions;
const EMPTY_BUMPERS: FullDemoBumperOptions = { intro: { enabled: false, video: null }, outro: { enabled: false, video: null } };

export function FullDemoBumpers({ options, document, onChange, onAssetBusy }: Props): ReactNode {
  const bumpers = options.bumpers ?? EMPTY_BUMPERS;
  return <div className="space-y-4">
    {(['intro', 'outro'] as const).map((slot) => <BumperUpload key={slot} slot={slot}
      asset={bumpers[slot].enabled ? bumpers[slot].video : null}
      savedName={document?.assets?.find((asset) => asset.ref.id === bumpers[slot].video?.id)?.title}
      onBusy={onAssetBusy} onChange={(video) => onChange({ ...options, bumpers: { ...bumpers, [slot]: { enabled: video !== null, video } } })} />)}
  </div>;
}

function BumperUpload({ slot, asset, savedName, onBusy, onChange }: {
  slot: Slot; asset: FullDemoAssetRef | null; savedName?: string;
  onBusy: (busy: boolean) => void; onChange: (asset: FullDemoAssetRef | null) => void;
}): ReactNode {
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const request = useRef<AbortController | null>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fileName, setFileName] = useState<{ id: string; name: string } | null>(null);
  const title = slot === 'intro' ? 'Intro' : 'Outro';
  const name = fileName?.id === asset?.id ? fileName?.name : savedName;
  useEffect(() => () => request.current?.abort(), []);
  useEffect(() => {
    if (!asset || name) return;
    const controller = new AbortController();
    void fetch(`/api/editor/assets/${asset.id}`, { signal: controller.signal }).then(async (response) => {
      if (!response.ok) return;
      const value: unknown = await response.json();
      if (!controller.signal.aborted && typeof value === 'object' && value !== null && 'file_name' in value && typeof value.file_name === 'string') {
        setFileName({ id: asset.id, name: value.file_name });
      }
    }).catch(() => {});
    return () => controller.abort();
  }, [asset, name]);

  async function upload(file: File): Promise<void> {
    if (request.current) return;
    const controller = new AbortController(); request.current = controller;
    setError(null); setUploading(true); onBusy(true);
    try {
      const ref = await uploadFullDemoBumper(file, controller.signal);
      if (!controller.signal.aborted) { setFileName({ id: ref.id, name: file.name }); onChange(ref); }
    } catch (failure) {
      if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'No se pudo subir el MP4. Inténtalo de nuevo.');
    } finally {
      if (!controller.signal.aborted) { request.current = null; setUploading(false); onBusy(false); }
    }
  }

  return <div data-bumper={slot} className="min-w-0 space-y-2 border-t border-border-subtle pt-3 first:border-0 first:pt-0">
    <div className="flex min-w-0 items-start justify-between gap-2">
      <div className="min-w-0">
        <h3 id={id} className="text-body-sm font-semibold text-fg-1">{title}</h3>
        <p className="text-body-sm text-fg-2">{slot === 'intro' ? 'Antes de la demo' : 'Después de la demo'} · Transición automática</p>
      </div>
      {asset ? <Button type="button" variant="ghost" size="icon-sm" aria-label={`Quitar ${slot}`} onClick={() => { setError(null); onChange(null); }}><X aria-hidden /></Button> : null}
    </div>
    {asset ? <>
      <p className="truncate text-body-sm text-fg-1" title={name}>{name ?? `${title}.mp4`}</p>
      <FullDemoMediaPreview asset={asset} video label={`Previsualizar ${slot}`} showDescription={false} preload="metadata" />
    </> : null}
    <input ref={input} className="sr-only" type="file" accept=".mp4,video/mp4" tabIndex={-1} aria-label={`Archivo MP4 de ${slot}`} onChange={(event) => {
      const file = event.target.files?.[0]; event.target.value = ''; if (file) void upload(file);
    }} />
    <Button type="button" variant="outline-primary" className="group relative w-full overflow-hidden motion-reduce:transition-none"
      loading={uploading} loadingText={`Subiendo ${slot}…`} onClick={() => input.current?.click()}
      onPointerMove={(event) => {
        const box = event.currentTarget.getBoundingClientRect();
        event.currentTarget.style.setProperty('--pointer-x', `${event.clientX - box.left}px`);
        event.currentTarget.style.setProperty('--pointer-y', `${event.clientY - box.top}px`);
      }}>
      <span aria-hidden className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-(--dur-fast) group-hover:opacity-100 motion-reduce:transition-none"
        style={{ background: 'radial-gradient(110px circle at var(--pointer-x,50%) var(--pointer-y,50%),color-mix(in oklch,var(--primary) 24%,transparent),transparent)' }} />
      {!uploading ? <Upload aria-hidden className="relative transition-transform duration-(--dur-fast) motion-safe:group-hover:-translate-y-0.5" /> : null}
      <span className="relative">{asset ? `Cambiar MP4 de ${slot}` : `Subir MP4 de ${slot}`}</span>
    </Button>
    {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
  </div>;
}
