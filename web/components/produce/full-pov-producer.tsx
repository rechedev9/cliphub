'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { Match, Play } from '@/lib/api/types';
import { hubHref, seriesHref } from '@/lib/clips/routes';
import type { FullDemoLoadFailure } from '@/lib/full-demo';
import {
  approveFullDemo, currentFullDemoOptions, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoPlanEdit, isFullDemoOptions, loadFullDemoPlan, saveFullDemoPlan,
  FULL_DEMO_CAPTURE_VARIANT, type FullDemoDocument, type FullDemoOptions, type FullDemoRound,
} from '@/lib/full-demo-plan';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { MapCover } from '@/components/brand/map-cover';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { recommendedFullDemoSettings } from '@/lib/produce/full-demo-recommended';
import { ProduceFooter } from './produce-footer';
import { FullDemoChoice, FullDemoGroup, FullDemoNumber, FullDemoToggle } from './full-demo-fields';
import { FullDemoAudio, FullDemoAudioAdvanced, FullDemoSponsor } from './full-demo-audio';
import { FullDemoOverlays } from './full-demo-overlays';
import { FullDemoTransitions } from './full-demo-transitions';
import { fullDemoTransitionSummary } from '@/lib/full-demo-transitions';
import { FullDemoHud } from './full-demo-hud';
import { customHudLabel } from '@/lib/custom-hud';

export type FullPovProducerProps = {
  matchId: string; match: Match; rounds: Play[]; recapFailure: Exclude<FullDemoLoadFailure, null> | null; recBusy: boolean; seriesId: string | null;
};

/** Full POV stays in the existing generate flow; the server's editorial plan owns every decision. */
export function FullPovProducer({ matchId, match, recBusy, seriesId }: FullPovProducerProps): ReactNode {
  const router = useRouter();
  const returnHref = seriesId ? seriesHref(seriesId) : hubHref({ open: matchId });
  const [document, setDocument] = useState<FullDemoDocument | null>(null);
  const [options, setOptions] = useState<FullDemoOptions | null>(null);
  const [defaults, setDefaults] = useState<FullDemoOptions | null>(null);
  const [busy, setBusy] = useState<'load' | 'plan' | 'create' | 'asset' | null>('load');
  const [error, setError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const draftKey = `cliphub.full-demo.draft.v1:${matchId}`;

  useEffect(() => {
    const controller = new AbortController();
    setBusy('load'); setError(null); setDocument(null); setOptions(null); setDefaults(null);
    void loadFullDemoPlan(matchId, controller.signal).then((loaded) => {
      if (controller.signal.aborted) return;
      let initial = loaded.document?.options ?? loaded.defaults;
      try {
        const raw = localStorage.getItem(draftKey);
        const draft: unknown = raw ? JSON.parse(raw) : null;
        if (isFullDemoOptions(draft)) initial = draft;
      } catch { /* The durable server plan remains available when local drafts are unavailable. */ }
      initial = currentFullDemoOptions(initial);
      if (loaded.document) {
        const saved = loaded.document;
        initial.editorial.manual_ranges = initial.editorial.manual_ranges.map((range) => {
          const round = saved.rounds.find((round) => round.round_id === range.round_id);
          return round ? { ...range, start_tick: round.live_start_tick - 2 * saved.clock.tick_rate } : range;
        });
      }
      setDocument(loaded.document); setDefaults(loaded.defaults); setOptions(initial); setBusy(null);
    }).catch((failure: unknown) => {
      if (controller.signal.aborted) return;
      setError(failure instanceof Error ? failure.message : 'No se pudo cargar el plan.'); setBusy(null);
    });
    return () => controller.abort();
  }, [matchId, draftKey, loadAttempt]);

  function change(next: FullDemoOptions): void {
    next = currentFullDemoOptions(next);
    setOptions(next);
    try { localStorage.setItem(draftKey, JSON.stringify(next)); } catch { /* Saving the server plan is still explicit and durable. */ }
  }
  // The saved plan must match every current option and have no blockers.
  // Creating binds that validated document to the capture request directly.
  const ready = document !== null && options !== null && fullDemoApprovalKey(document, options) !== null && busy === null;
  const dirty = options !== null && (document === null || fullDemoOptionsKey(document.options) !== fullDemoOptionsKey(options));
  const rounds = document?.rounds ?? [];
  const savedPlanStatus = recBusy ? 'CS2 ocupado: entrará en cola' : 'Plan guardado';

  async function plan(): Promise<void> {
    if (!options || busy) return;
    setBusy('plan'); setError(null);
    try {
      const planned = await saveFullDemoPlan(matchId, options);
      setDocument(planned); setOptions(planned.options);
      try { localStorage.setItem(draftKey, JSON.stringify(planned.options)); } catch { /* The plan was saved durably by the server. */ }
    } catch (failure) { setError(failure instanceof Error ? failure.message : 'No se pudo guardar el plan.'); }
    finally { setBusy(null); }
  }
  async function create(): Promise<void> {
    if (!document || !ready) return;
    setBusy('create'); setError(null);
    try {
      await api.createVideo({ matchId, playIds: rounds.map((round) => round.round_id), mode: 'clean', variant: FULL_DEMO_CAPTURE_VARIANT, editConfig: fullDemoPlanEdit(approveFullDemo(document)) });
      toast('Full Demo en cola', { description: recBusy ? 'Empezará cuando quede libre CS2.' : 'Sigue el progreso en Demos y vídeos.' });
      router.push(returnHref);
    } catch (failure) { setError(failure instanceof Error ? failure.message : 'No se pudo encolar el vídeo.'); setBusy(null); }
  }

  const nativeHudLabel = options?.capture.hud_profile === 'native-clean-spectator' ? 'Espectador limpio' : 'Nativo';
  const briefItems = options ? [
    { label: 'Jugador', value: match.player ?? document?.input.target_steamid64 ?? 'Pendiente' },
    { label: 'HUD', value: options.overlays.hud_theme ? customHudLabel(options.overlays.hud_theme) : nativeHudLabel },
    { label: 'Crosshair', value: options.capture.crosshair.mode === 'observed' ? 'Del jugador' : options.capture.crosshair.code },
    { label: 'Voces', value: options.audio.voice.enabled ? `${options.audio.voice.gain}×` : 'Sin voces' },
    { label: 'Transiciones', value: fullDemoTransitionSummary(options.transitions) },
    { label: 'Música', value: options.audio.music.enabled ? `${options.audio.music.assets.length} pistas` : 'Desactivada' },
    { label: 'Sponsor', value: options.sponsor.enabled ? 'Incluido' : 'Desactivado' },
    { label: 'Overlays', value: `Roster ${options.overlays.roster ? 'sí' : 'no'} · marcador ${options.overlays.scoreboard ? 'sí' : 'no'}` },
  ] : [];

  return <>
    <div className="min-w-0 space-y-2">
      <p className="wrap-anywhere font-mono text-meta uppercase tracking-ultra text-fg-3">Vídeo largo · {match.map}{match.player ? ` · ${match.player}` : ''}</p>
      <h1 className="font-display text-display-sm font-bold uppercase text-fg-1">Full POV Chill</h1>
      <p className="max-w-3xl text-body-sm text-fg-2">Todas las rondas del jugador en orden, con su HUD y audio. Ajusta el aspecto y el sonido, guarda el plan y graba.</p>
    </div>
    {options && defaults ? <div className="studio-panel my-4 flex flex-wrap items-center gap-3 p-4">
      <div className="min-w-0 flex-1">
        <p className="font-semibold text-fg-1">Ajustes recomendados</p>
        <p className="text-body-sm text-fg-2">Primera persona a 1080p60, 2 segundos antes de cada ronda y mezcla equilibrada. Conserva tu música, anuncios y opciones de voces.</p>
      </div>
      <Button variant="outline" disabled={busy !== null} onClick={() => change(recommendedFullDemoSettings(options, defaults))}>Usar ajustes recomendados</Button>
    </div> : null}
    {busy === 'load' ? <p role="status" className="text-body-sm text-fg-2">Cargando el plan guardado…</p> : null}
    {options === null && busy === null && error ? <Button variant="secondary" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>Reintentar conexión y cargar plan</Button> : null}
    {options ? <fieldset disabled={busy !== null} inert={busy !== null} className="grid min-w-0 items-start gap-5 @[56rem]/content:grid-cols-[minmax(0,1fr)_minmax(300px,0.8fr)]">
      <div className="min-w-0 @[56rem]/content:col-span-2"><FullDemoHud options={options} map={match.map} onChange={change} /></div>
      <div className="min-w-0 space-y-5">
        <FullDemoGroup title="Aspecto" note="1080p60 y primera persona. Los custom HUD comparten una misma captura base.">
          <div className="grid gap-4 sm:grid-cols-2">
            {!options.overlays.hud_theme ? <FullDemoChoice label="HUD nativo" value={options.capture.hud_profile} options={[{ value: 'native-clean-spectator', label: 'Espectador limpio' }, { value: 'native', label: 'Nativo' }]} onChange={(hud_profile) => change({ ...options, capture: { ...options.capture, hud_profile } })} /> : null}
            <FullDemoChoice label="Crosshair" value={options.capture.crosshair.mode} options={[{ value: 'observed', label: 'Del jugador observado' }, { value: 'provided-code', label: 'Código personalizado' }]} onChange={(mode) => change({ ...options, capture: { ...options.capture, crosshair: { ...options.capture.crosshair, mode, code: '' } } })} />
            {options.capture.crosshair.mode === 'provided-code' ? <label className="space-y-1.5 text-body-sm text-fg-2">Código de crosshair<Input value={options.capture.crosshair.code} maxLength={34} placeholder="CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx" onChange={(event) => change({ ...options, capture: { ...options.capture, crosshair: { ...options.capture.crosshair, code: event.target.value } } })} /></label> : null}
          </div>
        </FullDemoGroup>
        <FullDemoAudio options={options} document={document} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
        <FullDemoTransitions options={options} onChange={change} />
      </div>
      <div className="min-w-0 space-y-5">
        <MediaFrame aspect="16:9" fallback={<MapCover map={match.map} />} footer={<span className="text-meta text-fg-2">1920×1080 · 60 fps · primera persona</span>} />
        <FullDemoGroup title="Extra" note="Overlays sobre el vídeo y un anuncio opcional.">
          <FullDemoOverlays options={options} map={match.map} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
          <div className="space-y-4 border-t border-border-subtle pt-3">
            <FullDemoSponsor options={options} document={document} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
          </div>
        </FullDemoGroup>
      </div>
      <details className="studio-panel p-4 @[56rem]/content:col-span-2">
        <summary className="cursor-pointer font-display text-body-lg font-semibold uppercase text-fg-1">Avanzado</summary>
        <div className="mt-4 space-y-6">
          <section className="space-y-4">
            <h3 className="font-display text-body font-semibold uppercase text-fg-1">Captura</h3>
            <div className="grid gap-4 sm:grid-cols-2">
              <FullDemoChoice label="Origen de la demo" value={options.source_kind} options={[{ value: 'demo', label: 'Demo local' }, { value: 'premier', label: 'Premier' }, { value: 'professional', label: 'Profesional' }, { value: 'faceit', label: 'FACEIT' }]} onChange={(source_kind) => change({ ...options, source_kind })} />
            </div>
            <FullDemoToggle label="Si no hay crosshair en la demo, acepto el de captura" value={options.capture.crosshair.allow_capture_default} onChange={(allow_capture_default) => change({ ...options, capture: { ...options.capture, crosshair: { ...options.capture.crosshair, allow_capture_default } } })} />
            <p className="text-meta text-fg-3">Xray desactivado. Sin cámara de muerte, cambios de jugador ni adornos de Shorts.</p>
          </section>
          <section className="space-y-4 border-t border-border-subtle pt-4">
            <div>
              <h3 className="font-display text-body font-semibold uppercase text-fg-1">Rondas</h3>
              <p className="mt-1 text-body-sm text-fg-2">Freeze fijo: los últimos 2 segundos antes de jugar en todas las rondas, sin ampliarlo por voces. El calentamiento de cámara queda fuera del vídeo.</p>
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <FullDemoNumber label="Cola tras morir (s)" value={options.editorial.death_tail_seconds} max={3} step={0.5} onChange={(death_tail_seconds) => change({ ...options, editorial: { ...options.editorial, death_tail_seconds } })} />
              <FullDemoNumber label="Cola si sobrevives (s)" value={options.editorial.round_tail_seconds} max={2} step={0.5} onChange={(round_tail_seconds) => change({ ...options, editorial: { ...options.editorial, round_tail_seconds } })} />
            </div>
            <FullDemoToggle label="Permitir acortar solo las colas si se pierde la primera persona" value={options.editorial.allow_safe_tail_trim} onChange={(allow_safe_tail_trim) => change({ ...options, editorial: { ...options.editorial, allow_safe_tail_trim } })} />
            <div className="flex items-center gap-2"><StatusTag tone="primary">{rounds.length} rondas</StatusTag><span className="text-meta text-fg-3">{dirty ? 'Cambios pendientes de calcular' : 'Plan guardado'}</span></div>
            {rounds.map((round) => <RoundRow key={round.round_id} round={round} options={options} tickRate={document?.clock.tick_rate ?? 64} voice={document?.voice} onChange={change} />)}
          </section>
          <section className="space-y-4 border-t border-border-subtle pt-4">
            <h3 className="font-display text-body font-semibold uppercase text-fg-1">Mezcla</h3>
            <FullDemoAudioAdvanced options={options} onChange={change} />
          </section>
          <section className="space-y-4 border-t border-border-subtle pt-4">
            <h3 className="font-display text-body font-semibold uppercase text-fg-1">Overlays y portada</h3>
            <FullDemoChoice label="Portada" value={options.outputs.cover_policy} options={[{ value: 'no-cover', label: 'Sin portada' }, { value: 'generated-gameplay', label: 'Fotograma del gameplay' }]} onChange={(cover_policy) => change({ ...options, outputs: { ...options.outputs, cover_policy } })} />
          </section>
          {document ? <a href={`/api/demos/${matchId}/full-demo/plans/${document.plan_id}`} target="_blank" rel="noreferrer" className="text-body-sm text-primary underline">Ver documento del plan · {document.plan_hash.slice(0, 12)}</a> : null}
        </div>
      </details>
    </fieldset> : null}
    <div className="space-y-3">
      {busy === 'asset' ? <p role="status" className="text-body-sm text-fg-2">Subiendo y verificando el archivo…</p> : null}
      <Button onClick={() => void plan()} disabled={!options || busy !== null || !isFullDemoOptions(options)} loading={busy === 'plan'} loadingText="Analizando voces y rondas…">{document ? 'Actualizar y guardar plan' : 'Analizar rondas y guardar plan'}</Button>
      {dirty ? <p role="status" className="text-body-sm text-fg-2">Guarda el plan para revisar los nuevos intervalos, archivos y bloqueos. Esta operación no abre CS2.</p> : null}
      {(document?.blockers ?? []).map((item, index) => <p key={`${item.code}-${index}`} role="alert" className="border border-destructive/40 bg-destructive/10 p-3 text-body-sm text-destructive">{item.message}{item.round_id ? ` (${item.round_id})` : ''}</p>)}
      {(document?.warnings ?? []).map((item, index) => <p key={`${item.code}-${index}`} className="text-body-sm text-fg-2">{item.message}</p>)}
    </div>
    <ProduceFooter tone="full" eyebrow="Full POV Chill · 16:9" summary={document ? `${rounds.length} rondas · ${dirty ? 'Cambios pendientes de guardar' : savedPlanStatus}` : null}
      hint="Completa los ajustes y guarda un plan sin bloqueos para continuar." briefItems={briefItems}
      ready={ready} backHref={returnHref} busy={busy !== null} error={error}
      cta={<Button variant="stream" size="lg" disabled={!ready} loading={busy === 'create'} loadingText="Encolando…" onClick={() => void create()}>{recBusy ? 'Poner Full Demo en cola' : 'Crear Full Demo'}</Button>} />
  </>;
}

function RoundRow({ round, options, tickRate, voice, onChange }: { round: FullDemoRound; options: FullDemoOptions; tickRate: number; voice: FullDemoDocument['voice'] | undefined; onChange: (options: FullDemoOptions) => void }): ReactNode {
  const custom = options.editorial.manual_ranges.find((range) => range.round_id === round.round_id);
  const start = round.live_start_tick - 2 * tickRate;
  const end = custom?.end_tick ?? round.requested_end_tick;
  const audible = voice?.activity?.some((interval) => interval.start < end && interval.end > start) ?? false;
  let voiceLabel = 'no disponibles; revisa el informe de voz';
  if (voice?.availability === 'not_requested') voiceLabel = 'análisis desactivado';
  if (voice?.availability === 'available') voiceLabel = audible ? 'actividad detectada en este intervalo' : 'sin actividad detectada en este intervalo';
  function range(start_tick: number, end_tick: number): void {
    onChange({ ...options, editorial: { ...options.editorial, manual_ranges: [...options.editorial.manual_ranges.filter((item) => item.round_id !== round.round_id), { round_id: round.round_id, start_tick, end_tick }] } });
  }
  return <details className="border-t border-border-subtle py-2">
    <summary className="flex min-h-10 cursor-pointer flex-wrap items-center gap-3 text-body-sm text-fg-1">
      <span className="font-mono">R{String(round.source_round_number).padStart(2, '0')}</span>
      <span>{round.kills?.length ?? 0} kills</span><span className="ml-auto font-mono text-fg-2">{((end - start) / tickRate).toFixed(1)} s{custom ? ' · manual' : ''}</span>
    </summary>
    <p className="mb-3 text-meta text-fg-3">Inicio: {round.start_reason} · final: {round.end_reason}. El intervalo debe quedar dentro de los límites de la ronda.</p>
    <p className="mb-3 text-meta text-fg-3">Voces de equipo: {voiceLabel}.</p>
    <div className="grid gap-3 sm:grid-cols-2">
      <p className="text-body-sm text-fg-2">Inicio fijo: tick {start} · 2 segundos de freeze</p>
      <FullDemoNumber label={`R${round.source_round_number}: tick final`} value={end} onChange={(value) => range(start, value)} />
    </div>
    {custom ? <Button variant="ghost" size="sm" onClick={() => onChange({ ...options, editorial: { ...options.editorial, manual_ranges: options.editorial.manual_ranges.filter((item) => item.round_id !== round.round_id) } })}>Restablecer intervalo automático</Button> : null}
  </details>;
}
