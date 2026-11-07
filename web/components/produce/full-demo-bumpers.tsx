'use client';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { AlertCircle, Check, Film, Plus, RefreshCw, Upload, X } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { LongOperation } from '@/components/studio/long-operation';
import { formatUploadBytes, uploadProgress, uploadSent, type UploadProgress } from '@/lib/api/upload';
import { uploadFullDemoBumper, type FullDemoBumperOptions, type FullDemoDocument, type FullDemoOptions, type FullDemoAssetRef } from '@/lib/full-demo-plan';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { roundListLabel } from '@/lib/library/analysis-video';
import { fullDemoOrder, type FullDemoBumperSlot } from '@/lib/produce/full-demo-order';
import { FullDemoMediaPreview } from './full-demo-media-preview';
import styles from './full-demo-bumpers.module.css';

type Props = {
  options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void;
  /** Source rounds of the video in play order, one block each in the strip. */
  rounds: readonly number[];
};
type Slot = FullDemoBumperSlot;
const EMPTY_BUMPERS: FullDemoBumperOptions = { intro: { enabled: false, video: null }, outro: { enabled: false, video: null } };
const SLOTS: readonly Slot[] = ['intro', 'sponsor', 'outro'];
const SLOT_COPY: Record<Slot, { title: string; named: string; placement: string; hint: string }> = {
  intro: { title: 'Intro', named: 'la intro', placement: 'Antes de la demo', hint: 'Selecciona tu clip de apertura' },
  // Fixed by the planner: after the second round, or after the only one.
  sponsor: { title: 'Sponsor', named: 'el sponsor', placement: 'Tras la ronda 2', hint: 'Selecciona el anuncio del sponsor' },
  outro: { title: 'Outro', named: 'el outro', placement: 'Después de la demo', hint: 'Selecciona tu clip de cierre' },
};

export function FullDemoBumpers({ options, document, onChange, onAssetBusy, rounds }: Props): ReactNode {
  const bumpers = options.bumpers ?? EMPTY_BUMPERS;
  const inputs = useRef<Partial<Record<Slot, HTMLInputElement | null>>>({});
  const headingId = useId();
  // `bumpers.sponsor` stays absent until a sponsor video is added, like Go's omitempty.
  const loaded = (slot: Slot): FullDemoAssetRef | null => bumpers[slot]?.enabled ? bumpers[slot].video : null;
  // FullDemoGroup's markup, plus the "Opcional" badge its string title cannot carry.
  return <section aria-labelledby={headingId} className="studio-panel flex min-w-0 flex-col gap-3.5 rounded-2xl p-4">
    <div>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id={headingId} className="font-display text-body-lg font-bold text-fg-1">Intro, sponsor y outro</h2>
        <Badge variant="outline" className="text-fg-2">Opcional</Badge>
      </div>
      <p className="mt-1.5 text-body-sm text-fg-2">Tu marca al principio y al final, y tu sponsor tras la ronda 2. Se quedan guardados para tus próximas demos.</p>
    </div>
    <div className="flex min-h-9 min-w-0 items-stretch gap-1" role="group" aria-label="Orden del vídeo: intro, rondas, sponsor y outro">
      {fullDemoOrder(rounds).map((part, index) => part.kind === 'bumper'
        ? <SlotButton key={part.slot} slot={part.slot} loaded={loaded(part.slot) !== null} onOpen={() => inputs.current[part.slot]?.click()} />
        : <RoundBlocks key={`rounds-${index}`} rounds={part.rounds} placeholder={index === 1 && rounds.length === 0} />)}
    </div>
    <p className="text-meta text-fg-3">Cada bloque es una ronda. Intro, sponsor y outro son MP4 tuyos y opcionales: pulsa uno para subirlo.</p>
    <div className={styles.body}>
      <div className={styles.slots}>
        {SLOTS.map((slot) => <BumperUpload key={slot} slot={slot} asset={loaded(slot)} inputRef={(element) => { inputs.current[slot] = element; }}
          savedName={document?.assets?.find((asset) => asset.ref.id === bumpers[slot]?.video?.id)?.title}
          onBusy={onAssetBusy} onChange={(video) => onChange({ ...options, bumpers: { ...bumpers, [slot]: { enabled: video !== null, video } } })} />)}
      </div>
      <p className={styles.transitionNote}><span aria-hidden />Transiciones automáticas con la demo</p>
    </div>
  </section>;
}

function SlotButton({ slot, loaded, onOpen }: { slot: Slot; loaded: boolean; onOpen: () => void }): ReactNode {
  const copy = SLOT_COPY[slot];
  return <button type="button" onClick={onOpen} aria-label={`${copy.title}, ${copy.placement.toLowerCase()}: ${loaded ? 'cambiar el MP4' : 'subir un MP4'}`}
    data-loaded={loaded}
    className="flex w-16 shrink-0 flex-col items-center justify-center gap-0.5 rounded-lg border-[1.5px] border-dashed border-format-full/60 py-1 text-meta font-bold text-fg-2 transition-colors hover:bg-format-full/12 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-50 data-[loaded=true]:border-solid data-[loaded=true]:bg-format-full/12 data-[loaded=true]:text-fg-1">
    {loaded ? <Check aria-hidden className="size-3.5 text-format-full" /> : <Plus aria-hidden className="size-3.5" />}{copy.title}
  </button>;
}

/** The round winners are not in the plan, so every block wears the Demo completa colour. */
function RoundBlocks({ rounds, placeholder }: { rounds: readonly number[]; placeholder: boolean }): ReactNode {
  if (placeholder) return <span className="flex min-w-0 flex-1 items-center justify-center rounded-sm bg-format-full/12 text-meta text-fg-2">Rondas</span>;
  if (rounds.length === 0) return null;
  return <span role="img" aria-label={`${rounds.length === 1 ? 'Ronda' : 'Rondas'} ${roundListLabel(rounds)}`} className="contents">
    {rounds.map((round) => <span key={round} title={`Ronda ${round}`} className={`min-w-0 flex-1 rounded-sm ${FORMAT_TONE.full.bg}`} />)}
  </span>;
}

function BumperUpload({ slot, asset, savedName, inputRef, onBusy, onChange }: {
  slot: Slot; asset: FullDemoAssetRef | null; savedName?: string; inputRef: (element: HTMLInputElement | null) => void;
  onBusy: (busy: boolean) => void; onChange: (asset: FullDemoAssetRef | null) => void;
}): ReactNode {
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const request = useRef<AbortController | null>(null);
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState<UploadProgress | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [fileName, setFileName] = useState<{ id: string; name: string } | null>(null);
  const copy = SLOT_COPY[slot];
  const title = copy.title;
  const actionLabel = asset ? `Cambiar MP4 de ${copy.named}` : `Subir MP4 de ${copy.named}`;
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
    setError(null); setUploading(true); setProgress(uploadProgress(0, file.size)); onBusy(true);
    try {
      const ref = await uploadFullDemoBumper(file, { signal: controller.signal, onProgress: setProgress });
      if (!controller.signal.aborted) { setFileName({ id: ref.id, name: file.name }); onChange(ref); }
    } catch (failure) {
      if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'No se pudo subir el MP4. Inténtalo de nuevo.');
    } finally {
      if (!controller.signal.aborted) { request.current = null; setUploading(false); setProgress(null); onBusy(false); }
    }
  }

  return <div data-bumper={slot} data-loaded={!!asset} className={styles.card} role="group" aria-labelledby={id}>
    <div className={styles.cardHeader}>
      <div className={styles.cardTitle}>
        <div><h3 id={id}>{title}</h3><p>{copy.placement}</p></div>
      </div>
      {asset ? <span className={styles.ready}><Check aria-hidden />Listo</span> : null}
    </div>
    {asset ? <div className={styles.preview}>
      <FullDemoMediaPreview asset={asset} video label={`Previsualizar ${copy.named}`} showDescription={false} preload="metadata" />
    </div> : null}
    <div className={asset ? styles.loadedControls : undefined}>
      {asset ? <div className={styles.file}>
        <Film aria-hidden /><span title={name}>{name ?? `${title}.mp4`}</span>
      </div> : null}
    <input ref={(element) => { input.current = element; inputRef(element); }} className="sr-only" type="file" accept=".mp4,video/mp4" tabIndex={-1} aria-label={`Archivo MP4 de ${copy.named}`} onChange={(event) => {
      const file = event.target.files?.[0]; event.target.value = ''; if (file) void upload(file);
    }} />
    <Button type="button" variant="ghost" className={`${styles.upload} ${asset ? styles.replace : styles.empty}`}
      aria-label={uploading ? `Subiendo ${copy.named}…` : actionLabel}
      loading={uploading} loadingText={`Subiendo ${copy.named}…`} onClick={() => input.current?.click()}>
      {asset ? <RefreshCw aria-hidden className={styles.replaceIcon} /> : <span className={styles.uploadIcon}><Upload aria-hidden /></span>}
      <span className={styles.uploadCopy}>{asset ? 'Cambiar' : 'Subir un MP4'}{!asset ? <span>{copy.hint}</span> : null}</span>
    </Button>
    {asset ? <Button type="button" variant="ghost" size="icon-xs" className={styles.remove} aria-label={`Quitar ${copy.named}`} title={`Quitar ${copy.named}`} onClick={() => { setError(null); onChange(null); }}><X aria-hidden /></Button> : null}
    </div>
    {uploading && progress ? <div className="px-3 pb-3">
      {uploadSent(progress) ? <LongOperation stage="Verificando el archivo…" />
        : <LongOperation stage="Subiendo" percent={progress.percent} detail={formatUploadBytes(progress)} />}
    </div> : null}
    {error ? <div role="alert" className={styles.error}><AlertCircle aria-hidden /><p>{error}{asset ? <span>Tu vídeo anterior sigue seleccionado.</span> : null}</p></div> : null}
  </div>;
}
