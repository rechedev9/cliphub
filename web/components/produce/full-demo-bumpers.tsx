'use client';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { AlertCircle, Check, ChevronRight, Film, RefreshCw, Upload, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { uploadFullDemoBumper, type FullDemoBumperOptions, type FullDemoDocument, type FullDemoOptions, type FullDemoAssetRef } from '@/lib/full-demo-plan';
import { FullDemoMediaPreview } from './full-demo-media-preview';
import styles from './full-demo-bumpers.module.css';

type Props = { options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void };
type Slot = keyof FullDemoBumperOptions;
const EMPTY_BUMPERS: FullDemoBumperOptions = { intro: { enabled: false, video: null }, outro: { enabled: false, video: null } };
const SLOTS: readonly Slot[] = ['intro', 'sponsor', 'outro'];
const SLOT_COPY: Record<Slot, { title: string; number: string; placement: string; hint: string }> = {
  intro: { title: 'Intro', number: '01', placement: 'Antes de la demo', hint: 'Selecciona tu clip de apertura' },
  // Fixed by the planner: after the second round, or after the only one.
  sponsor: { title: 'Sponsor', number: '02', placement: 'Tras la ronda 2', hint: 'Selecciona el anuncio del sponsor' },
  outro: { title: 'Outro', number: '03', placement: 'Después de la demo', hint: 'Selecciona tu clip de cierre' },
};

export function FullDemoBumpers({ options, document, onChange, onAssetBusy }: Props): ReactNode {
  const bumpers = options.bumpers ?? EMPTY_BUMPERS;
  const headingId = useId();
  // `bumpers.sponsor` stays absent until a sponsor video is added, like Go's omitempty.
  const loaded = (slot: Slot): FullDemoAssetRef | null => bumpers[slot]?.enabled ? bumpers[slot].video : null;
  return <section className={`studio-panel ${styles.panel}`} aria-labelledby={headingId}>
    <header className={styles.heading}>
      <div className={styles.headingTitle}><Film aria-hidden /><h2 id={headingId}>Intro, sponsor y outro</h2></div>
      <span className={styles.optional}>Opcional</span>
    </header>
    <p className={styles.description}>Tu marca al principio y al final, y tu sponsor tras la ronda 2.</p>
    <div className={styles.sequence} role="img" aria-label="Orden del vídeo: intro, rondas 1 y 2, sponsor, resto de la demo y outro">
      <span data-active={!!loaded('intro')}>Intro</span>
      <ChevronRight aria-hidden />
      <span className={styles.demo}><Film aria-hidden />R1–2</span>
      <ChevronRight aria-hidden />
      <span data-active={!!loaded('sponsor')}>Sponsor</span>
      <ChevronRight aria-hidden />
      <span className={styles.demo}><Film aria-hidden />Resto</span>
      <ChevronRight aria-hidden />
      <span data-active={!!loaded('outro')}>Outro</span>
    </div>
    <div className={styles.slots}>
      {SLOTS.map((slot) => <BumperUpload key={slot} slot={slot} asset={loaded(slot)}
        savedName={document?.assets?.find((asset) => asset.ref.id === bumpers[slot]?.video?.id)?.title}
        onBusy={onAssetBusy} onChange={(video) => onChange({ ...options, bumpers: { ...bumpers, [slot]: { enabled: video !== null, video } } })} />)}
    </div>
    <p className={styles.transitionNote}><span aria-hidden />Transiciones automáticas con la demo</p>
  </section>;
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
  const copy = SLOT_COPY[slot];
  const title = copy.title;
  const actionLabel = asset ? `Cambiar MP4 de ${slot}` : `Subir MP4 de ${slot}`;
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

  return <div data-bumper={slot} data-loaded={!!asset} className={styles.card} role="group" aria-labelledby={id}>
    <div className={styles.cardHeader}>
      <div className={styles.cardTitle}>
        <span className={styles.number} aria-hidden>{copy.number}</span>
        <div><h3 id={id}>{title}</h3><p>{copy.placement}</p></div>
      </div>
      {asset ? <span className={styles.ready}><Check aria-hidden />Listo</span> : null}
    </div>
    {asset ? <div className={styles.preview}>
      <FullDemoMediaPreview asset={asset} video label={`Previsualizar ${slot}`} showDescription={false} preload="metadata" />
    </div> : null}
    <div className={asset ? styles.loadedControls : undefined}>
      {asset ? <div className={styles.file}>
        <Film aria-hidden /><span title={name}>{name ?? `${title}.mp4`}</span>
      </div> : null}
    <input ref={input} className="sr-only" type="file" accept=".mp4,video/mp4" tabIndex={-1} aria-label={`Archivo MP4 de ${slot}`} onChange={(event) => {
      const file = event.target.files?.[0]; event.target.value = ''; if (file) void upload(file);
    }} />
    <Button type="button" variant="ghost" className={`${styles.upload} ${asset ? styles.replace : styles.empty}`}
      aria-label={uploading ? `Subiendo ${slot}…` : actionLabel}
      loading={uploading} loadingText={`Subiendo ${slot}…`} onClick={() => input.current?.click()}
      onPointerMove={(event) => {
        const box = event.currentTarget.getBoundingClientRect();
        event.currentTarget.style.setProperty('--pointer-x', `${event.clientX - box.left}px`);
        event.currentTarget.style.setProperty('--pointer-y', `${event.clientY - box.top}px`);
      }}>
      <span aria-hidden data-bumper-glow className={styles.glow} />
      {asset ? <RefreshCw aria-hidden className={styles.replaceIcon} /> : <span className={styles.uploadIcon}><Upload aria-hidden /></span>}
      <span className={styles.uploadCopy}>{asset ? 'Cambiar' : 'Subir un MP4'}{!asset ? <span>{copy.hint}</span> : null}</span>
    </Button>
    {asset ? <Button type="button" variant="ghost" size="icon-xs" className={styles.remove} aria-label={`Quitar ${slot}`} title={`Quitar ${slot}`} onClick={() => { setError(null); onChange(null); }}><X aria-hidden /></Button> : null}
    </div>
    {error ? <div role="alert" className={styles.error}><AlertCircle aria-hidden /><p>{error}{asset ? <span>Tu vídeo anterior sigue seleccionado.</span> : null}</p></div> : null}
  </div>;
}
