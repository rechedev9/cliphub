'use client';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { fullDemoOverlayImageURL, uploadFullDemoOverlayImage, type FullDemoAssetRef, type FullDemoOptions } from '@/lib/full-demo-plan';
import { MapCover } from '@/components/brand/map-cover';
import { Button } from '@/components/ui/button';
import { FullDemoChoice, FullDemoToggle } from './full-demo-fields';

export function FullDemoOverlays({ options, map, onChange, onAssetBusy }: { options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void }): ReactNode {
  const overlay = options.overlays;
  const change = (next: Partial<FullDemoOptions['overlays']>): void => onChange({ ...options, overlays: { ...overlay, ...next } });
  const screenshots = overlay.mode === 'screenshots';
  return <div className="min-w-0 space-y-4">
    <FullDemoChoice label="Origen de la demo" value={options.source_kind} options={[{ value: 'faceit', label: 'FACEIT' }, { value: 'premier', label: 'Premier' }, { value: 'professional', label: 'Profesional' }, { value: 'demo', label: 'Demo local' }]} onChange={(source_kind) => onChange({ ...options, source_kind })} />
    <p className="text-meta text-fg-3">El origen fija el formato del roster y del marcador. Un custom HUD no lo cambia.</p>
    <FullDemoChoice label="Cómo crear los overlays" value={overlay.mode ?? 'generated'} options={[{ value: 'generated', label: 'Generar automáticamente' }, { value: 'screenshots', label: 'Subir capturas' }]} onChange={(mode) => change({ mode })} />
    <FullDemoToggle label="Overlay de roster" value={overlay.roster} onChange={(roster) => change({ roster })} />
    <FullDemoToggle label="Overlay de marcador" value={overlay.scoreboard} onChange={(scoreboard) => change({ scoreboard })} />
    {screenshots ? <>
      <p className="text-body-sm text-fg-2">Las imágenes se muestran completas, sin recortarlas ni estirarlas. PNG o JPG · hasta 10 MB por captura.</p>
      {overlay.roster ? <div className="grid min-w-0 gap-3 sm:grid-cols-2">
        <ScreenshotInput label="Equipo 1" asset={overlay.team1_image} onChange={(team1_image) => change({ team1_image })} onBusy={onAssetBusy} />
        <ScreenshotInput label="Equipo 2" asset={overlay.team2_image} onChange={(team2_image) => change({ team2_image })} onBusy={onAssetBusy} />
      </div> : null}
      {overlay.roster ? <ScreenshotPreview map={map} team1={overlay.team1_image} team2={overlay.team2_image} /> : null}
      {overlay.scoreboard ? <>
        <ScreenshotInput label="Marcador final" asset={overlay.scoreboard_image} onChange={(scoreboard_image) => change({ scoreboard_image })} onBusy={onAssetBusy} />
        <ScreenshotPreview map={map} scoreboard={overlay.scoreboard_image} outro />
      </> : null}
    </> : <>
      <FullDemoChoice label="Tema del overlay" value={overlay.theme} options={[{ value: 'neon-violet', label: 'Neón violeta' }, { value: 'faceit-orange', label: 'Naranja' }]} onChange={(theme) => change({ theme })} />
      {options.source_kind === 'demo' ? <>
        <FullDemoChoice label="Datos del overlay" value={overlay.source} options={[{ value: 'demo', label: 'Esta partida' }, { value: 'faceit', label: 'FACEIT (requiere conexión)' }]} onChange={(source) => change({ source })} />
        <p className="text-meta text-fg-3">El roster y el marcador se crean con los datos disponibles. FACEIT añade perfiles y estadísticas recientes.</p>
      </> : <p className="text-meta text-fg-3">{options.source_kind === 'faceit' ? 'Roster y marcador con niveles, ELO y estadísticas FACEIT (requiere conexión).' : 'Roster y marcador con los datos de la partida sobre las placas del origen.'}</p>}
    </>}
  </div>;
}

function ScreenshotInput({ label, asset, onChange, onBusy }: { label: string; asset?: FullDemoAssetRef | null; onChange: (ref: FullDemoAssetRef | null) => void; onBusy: (busy: boolean) => void }): ReactNode {
  const id = useId();
  const [error, setError] = useState<string | null>(null);
  const request = useRef<AbortController | null>(null);
  const fileInput = useRef<HTMLInputElement | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  async function upload(file: File): Promise<void> {
    const controller = new AbortController(); request.current = controller;
    setError(null); onBusy(true);
    try { const ref = await uploadFullDemoOverlayImage(file, controller.signal); if (!controller.signal.aborted) onChange(ref); }
    catch (failure) { if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : 'No se pudo subir la captura.'); }
    finally { if (!controller.signal.aborted) onBusy(false); }
  }
  return <div className="min-w-0 space-y-2 rounded border border-border-subtle p-3">
    <label htmlFor={id} className="block text-body-sm font-medium text-fg-1">{label}</label>
    <input ref={fileInput} id={id} aria-label={label} type="file" accept="image/png,image/jpeg,.png,.jpg,.jpeg" className="hidden" onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ''; if (file) void upload(file); }} />
    <Button type="button" size="sm" variant="secondary" className="w-full" aria-label={`${asset ? 'Cambiar' : 'Subir'} captura de ${label}`} onClick={() => fileInput.current?.click()}>{asset ? 'Cambiar captura' : 'Subir captura'}</Button>
    {asset ? <div className="flex items-center justify-between gap-2"><span className="text-meta text-fg-2">Captura guardada</span><Button type="button" size="sm" variant="ghost" aria-label={`Quitar captura de ${label}`} onClick={() => onChange(null)}>Quitar</Button></div> : null}
    {error ? <p role="alert" className="break-words text-body-sm text-destructive">{error}</p> : null}
  </div>;
}

function ScreenshotPreview({ map, team1, team2, scoreboard, outro = false }: { map: string; team1?: FullDemoAssetRef | null; team2?: FullDemoAssetRef | null; scoreboard?: FullDemoAssetRef | null; outro?: boolean }): ReactNode {
  const image = (ref: FullDemoAssetRef | null | undefined, label: string): ReactNode => ref
    // Local screenshots already have bounded dimensions; preserve their pixels.
    // eslint-disable-next-line @next/next/no-img-element
    ? <img src={fullDemoOverlayImageURL(ref)} alt={label} className="h-full w-full object-contain drop-shadow-lg" />
    : <span className="p-1 text-center text-[10px] text-white/70">{label}</span>;
  return <figure className="min-w-0 space-y-2">
    <figcaption className="text-meta text-fg-2">Vista previa · {outro ? 'marcador final' : 'inicio, equipo 1 a la izquierda y equipo 2 a la derecha'}</figcaption>
    <div aria-label={outro ? 'Vista previa del marcador final' : 'Vista previa de los equipos'} className="relative aspect-video overflow-hidden rounded border border-border-subtle bg-black">
      <div className="absolute inset-0 opacity-50"><MapCover map={map} /></div>
      {outro ? <div className="absolute left-[4%] top-[8%] flex h-[84%] w-[92%] items-center justify-center">{image(scoreboard, 'Marcador final')}</div> : <>
        <div className="absolute left-[2%] top-[5%] flex h-[90%] w-[30%] items-center justify-center">{image(team1, 'Equipo 1')}</div>
        <div className="absolute right-[2%] top-[5%] flex h-[90%] w-[30%] items-center justify-center">{image(team2, 'Equipo 2')}</div>
      </>}
    </div>
  </figure>;
}
