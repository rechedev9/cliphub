'use client';

import { useEffect, useId, useState, type ReactNode } from 'react';
import {
  BRAND_CORNERS,
  BRAND_CORNER_LABELS,
  brandImageUrl,
  brandKitProblem,
  getBrandKit,
  isBrandCorner,
  saveBrandKit,
  uploadBrandImage,
  type BrandImage,
  type BrandKit,
} from '@/lib/brand-kit';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';

/** The saved brand kit, read once; a save hands the new kit back through setKit. */
export function useBrandKit(): { kit: BrandKit | null; error: string | null; setKit: (kit: BrandKit) => void } {
  const [kit, setKit] = useState<BrandKit | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    getBrandKit(controller.signal).then(setKit).catch((failure: unknown) => {
      if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'No se pudo leer el kit de marca.');
    });
    return () => controller.abort();
  }, []);
  return { kit, error, setKit };
}

/** The kit editor once the kit is read, or why it is not there yet. */
export function BrandKitPanel({ kit, error, onSaved, disabled = false }: {
  kit: BrandKit | null; error: string | null; onSaved: (kit: BrandKit) => void; disabled?: boolean;
}): ReactNode {
  if (kit) return <BrandKitEditor kit={kit} onSaved={onSaved} disabled={disabled} />;
  if (error) return <p role="alert" className="text-body-sm text-destructive">{error}</p>;
  return <p role="status" className="text-body-sm text-fg-3">Cargando el kit de marca…</p>;
}

/** Edits the saved kit: logo, its corner and opacity, and the sponsor banner with its window. */
function BrandKitEditor({ kit, onSaved, disabled = false }: { kit: BrandKit; onSaved: (kit: BrandKit) => void; disabled?: boolean }): ReactNode {
  const [draft, setDraft] = useState<BrandKit>(kit);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ tone: 'error' | 'info'; text: string } | null>(null);
  const id = useId();
  const dirty = JSON.stringify(draft) !== JSON.stringify(kit);
  const problem = brandKitProblem(draft);
  const opacityPercent = Math.round(draft.watermark_opacity * 100);

  async function upload(file: File, field: 'logo' | 'banner'): Promise<void> {
    setBusy(true); setMessage(null);
    try {
      const image = await uploadBrandImage(file);
      setDraft((current) => ({ ...current, [field]: image }));
    } catch (failure) {
      setMessage({ tone: 'error', text: failure instanceof Error ? failure.message : 'No se pudo subir la imagen.' });
    } finally { setBusy(false); }
  }

  function remove(field: 'logo' | 'banner'): void {
    setDraft((current) => {
      const { [field]: _removed, ...rest } = current;
      return rest;
    });
  }

  async function save(): Promise<void> {
    setBusy(true); setMessage(null);
    try {
      const saved = await saveBrandKit(draft);
      onSaved(saved);
      setDraft(saved);
      setMessage({ tone: 'info', text: 'Kit de marca guardado.' });
    } catch (failure) {
      setMessage({ tone: 'error', text: failure instanceof Error ? failure.message : 'No se pudo guardar el kit de marca.' });
    } finally { setBusy(false); }
  }

  const off = disabled || busy;
  return (
    <details className="min-w-0 rounded-xl border border-border-subtle p-3">
      <summary className="cursor-pointer text-body-sm font-semibold text-fg-1">Editar kit de marca</summary>
      <div className="mt-3 grid min-w-0 gap-4 sm:grid-cols-2">
        <BrandImageField id={`${id}-logo`} label="Logo del canal" image={draft.logo} disabled={off}
          onFile={(file) => void upload(file, 'logo')} onRemove={() => remove('logo')} />
        <BrandImageField id={`${id}-banner`} label="Banner de sponsor" image={draft.banner} disabled={off}
          onFile={(file) => void upload(file, 'banner')} onRemove={() => remove('banner')} />
        <div className="flex min-w-0 flex-col gap-2">
          <span className="text-meta text-fg-3">Esquina de la marca de agua</span>
          <ToggleGroup type="single" value={draft.watermark_corner} variant="outline" className="flex-wrap" disabled={off}
            onValueChange={(corner) => { if (isBrandCorner(corner)) setDraft({ ...draft, watermark_corner: corner }); }}>
            {BRAND_CORNERS.map((corner) => (
              <ToggleGroupItem key={corner} value={corner} aria-label={BRAND_CORNER_LABELS[corner]}>{BRAND_CORNER_LABELS[corner]}</ToggleGroupItem>
            ))}
          </ToggleGroup>
          <label htmlFor={`${id}-opacity`} className="text-meta text-fg-3">
            Opacidad <span className="text-fg-1">· {opacityPercent} %</span>
          </label>
          <input id={`${id}-opacity`} type="range" min={10} max={100} step={5} value={opacityPercent} disabled={off}
            onChange={(event) => setDraft({ ...draft, watermark_opacity: Number(event.target.value) / 100 })} className="w-full accent-primary" />
        </div>
        <div className="flex min-w-0 flex-col gap-2">
          <span className="text-meta text-fg-3">Cuándo se ve el banner</span>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="flex flex-col gap-1.5 text-meta text-fg-3">
              Entra (s)
              <Input type="number" min={0} step={0.5} value={draft.banner_start_seconds} disabled={off}
                onChange={(event) => setDraft({ ...draft, banner_start_seconds: Number(event.target.value) })} />
            </label>
            <label className="flex flex-col gap-1.5 text-meta text-fg-3">
              Sale (s)
              <Input type="number" min={0} step={0.5} value={draft.banner_end_seconds ?? ''} placeholder="Al final" disabled={off}
                onChange={(event) => {
                  const { banner_end_seconds: _end, ...rest } = draft;
                  const end = Number(event.target.value);
                  setDraft(event.target.value === '' || end === 0 ? rest : { ...rest, banner_end_seconds: end });
                }} />
            </label>
          </div>
          <p className="text-body-sm text-fg-3">Segundos del vídeo. Sin salida, el banner sigue hasta el final.</p>
        </div>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-3">
        <Button type="button" onClick={() => void save()} disabled={off || !dirty || problem !== null}>Guardar kit</Button>
        {problem && dirty ? <p role="alert" className="text-body-sm text-destructive">{problem}</p> : null}
        {message ? <p role={message.tone === 'error' ? 'alert' : 'status'} className={message.tone === 'error' ? 'text-body-sm text-destructive' : 'text-body-sm text-fg-2'}>{message.text}</p> : null}
      </div>
    </details>
  );
}

function BrandImageField({ id, label, image, disabled, onFile, onRemove }: {
  id: string; label: string; image?: BrandImage; disabled: boolean; onFile: (file: File) => void; onRemove: () => void;
}): ReactNode {
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <label htmlFor={id} className="text-meta text-fg-3">{label}</label>
      {image ? <img src={brandImageUrl(image)} alt={label} className="h-16 w-full rounded border border-border-subtle bg-surface-3 object-contain" /> : null}
      <Input id={id} type="file" accept="image/png,image/jpeg" disabled={disabled}
        onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ''; if (file) onFile(file); }} />
      <p className="text-body-sm text-fg-3">PNG o JPG, hasta 10 MB. Un PNG sin fondo queda mejor.</p>
      {image ? <Button type="button" variant="secondary" disabled={disabled} onClick={onRemove}>Quitar</Button> : null}
    </div>
  );
}
