'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { Match, Play } from '@/lib/api/types';
import { hubHref, seriesHref } from '@/lib/clips/routes';
import type { FullDemoLoadFailure } from '@/lib/full-demo';
import {
  approveFullDemo, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoPlanEdit, isFullDemoOptions, loadFullDemoPlan, saveFullDemoPlan,
  FULL_DEMO_CAPTURE_VARIANT, type FullDemoDocument, type FullDemoOptions, type FullDemoRound,
} from '@/lib/full-demo-plan';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { MapCover } from '@/components/brand/map-cover';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { ProduceFooter } from './produce-footer';
import { FullDemoChoice, FullDemoGroup, FullDemoNumber, FullDemoToggle } from './full-demo-fields';
import { FullDemoAudio, FullDemoSponsor } from './full-demo-audio';

export type FullPovProducerProps = {
  matchId: string; match: Match; rounds: Play[]; recapFailure: Exclude<FullDemoLoadFailure, null> | null; recBusy: boolean; seriesId: string | null;
};

/** Full POV stays in the existing generate flow; the server's editorial plan owns every decision. */
export function FullPovProducer({ matchId, match, recBusy, seriesId }: FullPovProducerProps): ReactNode {
  const router = useRouter();
  const returnHref = seriesId ? seriesHref(seriesId) : hubHref({ open: matchId });
  const [document, setDocument] = useState<FullDemoDocument | null>(null);
  const [options, setOptions] = useState<FullDemoOptions | null>(null);
  const [approvedHash, setApprovedHash] = useState<string | null>(null);
  const [busy, setBusy] = useState<'load' | 'plan' | 'create' | 'asset' | null>('load');
  const [error, setError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const draftKey = `cliphub.full-demo.draft.v1:${matchId}`;

  useEffect(() => {
    const controller = new AbortController();
    setBusy('load'); setError(null); setDocument(null); setOptions(null); setApprovedHash(null);
    void loadFullDemoPlan(matchId, controller.signal).then((loaded) => {
      if (controller.signal.aborted) return;
      let initial = loaded.document?.options ?? loaded.defaults;
      try {
        const raw = localStorage.getItem(draftKey);
        const draft: unknown = raw ? JSON.parse(raw) : null;
        if (isFullDemoOptions(draft)) initial = draft;
      } catch { /* The durable server plan remains available when local drafts are unavailable. */ }
      setDocument(loaded.document); setOptions(initial); setBusy(null);
    }).catch((failure: unknown) => {
      if (controller.signal.aborted) return;
      setError(failure instanceof Error ? failure.message : 'No se pudo cargar el plan.'); setBusy(null);
    });
    return () => controller.abort();
  }, [matchId, draftKey, loadAttempt]);

  function change(next: FullDemoOptions): void {
    setOptions(next); setApprovedHash(null);
    try { localStorage.setItem(draftKey, JSON.stringify(next)); } catch { /* Saving the server plan is still explicit and durable. */ }
  }
  const approvalKey = document && options ? fullDemoApprovalKey(document, options) : null;
  const approved = approvalKey !== null && approvedHash === approvalKey;
  const dirty = options !== null && (document === null || fullDemoOptionsKey(document.options) !== fullDemoOptionsKey(options));
  const rounds = document?.rounds ?? [];

  async function plan(): Promise<void> {
    if (!options || busy) return;
    setBusy('plan'); setError(null); setApprovedHash(null);
    try {
      const planned = await saveFullDemoPlan(matchId, options);
      setDocument(planned); setOptions(planned.options);
      try { localStorage.setItem(draftKey, JSON.stringify(planned.options)); } catch { /* The plan was saved durably by the server. */ }
    } catch (failure) { setError(failure instanceof Error ? failure.message : 'No se pudo guardar el plan.'); }
    finally { setBusy(null); }
  }
  async function create(): Promise<void> {
    if (!document || !approved || busy) return;
    setBusy('create'); setError(null);
    try {
      await api.createVideo({ matchId, playIds: rounds.map((round) => round.round_id), mode: 'clean', variant: FULL_DEMO_CAPTURE_VARIANT, editConfig: fullDemoPlanEdit(approveFullDemo(document)) });
      toast('Full Demo en cola', { description: recBusy ? 'Empezará cuando quede libre CS2.' : 'Sigue el progreso en Demos y vídeos.' });
      router.push(returnHref);
    } catch (failure) { setError(failure instanceof Error ? failure.message : 'No se pudo encolar el vídeo.'); setBusy(null); }
  }

  const briefItems = options ? [
    { label: 'Jugador / duración', value: `${match.player ?? document?.input.target_steamid64 ?? 'Pendiente'} · ${((document?.timeline?.at(-1)?.end_frame ?? 0) / 60).toFixed(1)} s estimados · captura local con CS2` },
    { label: 'Formato', value: '1920×1080 · 60 fps · 1× · primera persona' },
    { label: 'HUD / xray', value: `${options.capture.hud_profile === 'native-clean-spectator' ? 'Espectador limpio' : 'Nativo'} · xray desactivado` },
    { label: 'Crosshair', value: `${options.capture.crosshair.mode === 'observed' ? 'Del jugador' : options.capture.crosshair.code}${options.capture.crosshair.allow_capture_default ? ' · fallback aceptado' : ''}` },
    { label: 'Rondas', value: `${rounds.length} · incluidas las de 0 kills · freeze ${options.editorial.freeze_seconds} s${options.editorial.keep_freeze_voice ? `, voz hasta ${options.editorial.max_freeze_seconds} s` : ''}` },
    { label: 'Colas', value: `Muerte ${options.editorial.death_tail_seconds} s · supervivencia ${options.editorial.round_tail_seconds} s · recorte seguro ${options.editorial.allow_safe_tail_trim ? 'sí' : 'no'}` },
    { label: 'Juego / voces', value: `${options.audio.game.gain}× / ${options.audio.voice.enabled ? `${options.audio.voice.gain}×` : 'sin voces'} · fallback ${options.audio.voice.approved_fallback === 'block' ? 'bloquear' : 'sin voces'}` },
    { label: 'Música', value: options.audio.music.enabled ? `${options.audio.music.assets.length} pistas · ${options.audio.music.bed_gain_db} dB · ${options.audio.music.loop_policy === 'ordered-loop' ? 'repetir' : 'una vez'}` : 'Desactivada' },
    { label: 'Reducción de música', value: options.audio.music.ducking.enabled && options.audio.music.enabled ? `Ataque ${options.audio.music.ducking.attack_ms} ms · recuperación ${options.audio.music.ducking.release_ms} ms · juego ${options.audio.music.ducking.game_contribution}` : 'Desactivada' },
    { label: 'Sponsor', value: options.sponsor.enabled ? `${options.sponsor.audio_policy === 'embedded' ? 'Audio del vídeo' : 'Narración de reemplazo'} · ${((document?.sponsor_placement.start_frame ?? 0) / 60).toFixed(2)}–${(((document?.sponsor_placement.start_frame ?? 0) + (document?.sponsor_placement.duration_frames ?? 0)) / 60).toFixed(2)} s · música pausada` : 'Desactivado' },
    { label: 'Archivos aprobados', value: document?.assets?.length ? document.assets.map((asset) => `${asset.title} (${asset.ref.sha256.slice(0, 12)})`).join(' · ') : 'Sin archivos adicionales' },
    { label: 'Overlays', value: `Roster ${options.overlays.roster ? 'sí' : 'no'} · marcador ${options.overlays.scoreboard ? 'sí' : 'no'} · ${options.overlays.source}` },
    { label: 'Intro / outro / efectos / contador', value: 'Desactivados · corte directo' },
    { label: 'Portada / loudness', value: `${options.outputs.cover_policy === 'no-cover' ? 'Sin portada' : 'Fotograma del gameplay'} · −14 LUFS · ≤ −1,5 dBTP` },
  ] : [];

  return <>
    <div className="space-y-2">
      <p className="font-mono text-meta uppercase tracking-ultra text-fg-3">Vídeo largo · {match.map}{match.player ? ` · ${match.player}` : ''}</p>
      <h1 className="font-display text-display-sm font-bold uppercase text-fg-1">Full POV Chill</h1>
      <p className="max-w-3xl text-body-sm text-fg-2">Todas las rondas del jugador en orden, con su HUD y audio continuo. Ajusta el montaje, revisa sus intervalos y aprueba el plan antes de grabar.</p>
    </div>
    {busy === 'load' ? <p role="status" className="text-body-sm text-fg-2">Cargando el plan guardado…</p> : null}
    {options === null && busy === null && error ? <Button variant="secondary" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>Reintentar conexión y cargar plan</Button> : null}
    {options ? <fieldset disabled={busy !== null} inert={busy !== null} className="grid min-w-0 items-start gap-5 @[56rem]/content:grid-cols-[minmax(0,1fr)_minmax(300px,0.8fr)]">
      <div className="min-w-0 space-y-5">
        <FullDemoGroup title="Captura" note="1080p60, velocidad normal y primera persona. El HUD se decide antes de grabar.">
          <div className="grid gap-4 sm:grid-cols-2">
            <FullDemoChoice label="Origen de la demo" value={options.source_kind} options={[{ value: 'demo', label: 'Demo local' }, { value: 'premier', label: 'Premier' }, { value: 'professional', label: 'Profesional' }, { value: 'faceit', label: 'FACEIT' }]} onChange={(source_kind) => change({ ...options, source_kind })} />
            <FullDemoChoice label="HUD" value={options.capture.hud_profile} options={[{ value: 'native-clean-spectator', label: 'Espectador limpio' }, { value: 'native', label: 'Nativo' }]} onChange={(hud_profile) => change({ ...options, capture: { ...options.capture, hud_profile } })} />
            <FullDemoChoice label="Crosshair" value={options.capture.crosshair.mode} options={[{ value: 'observed', label: 'Del jugador observado' }, { value: 'provided-code', label: 'Código personalizado' }]} onChange={(mode) => change({ ...options, capture: { ...options.capture, crosshair: { ...options.capture.crosshair, mode, code: '' } } })} />
            {options.capture.crosshair.mode === 'provided-code' ? <label className="space-y-1.5 text-body-sm text-fg-2">Código de crosshair<Input value={options.capture.crosshair.code} maxLength={34} placeholder="CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx" onChange={(event) => change({ ...options, capture: { ...options.capture, crosshair: { ...options.capture.crosshair, code: event.target.value } } })} /></label> : null}
          </div>
          <FullDemoToggle label="Si no hay crosshair en la demo, acepto el de captura" value={options.capture.crosshair.allow_capture_default} onChange={(allow_capture_default) => change({ ...options, capture: { ...options.capture, crosshair: { ...options.capture.crosshair, allow_capture_default } } })} />
          <p className="text-meta text-fg-3">Xray desactivado. Sin cámara de muerte, cambios de jugador ni adornos de Shorts.</p>
        </FullDemoGroup>
        <FullDemoGroup title="Rondas" note="El plan conserva también las rondas sin bajas. Los límites se obtienen de eventos de la demo.">
          <div className="grid gap-4 sm:grid-cols-2">
            <FullDemoNumber label="Freeze antes de jugar (s)" value={options.editorial.freeze_seconds} max={20} step={0.5} onChange={(freeze_seconds) => change({ ...options, editorial: { ...options.editorial, freeze_seconds } })} />
            <FullDemoNumber label="Freeze máximo con voces (s)" value={options.editorial.max_freeze_seconds} max={60} step={0.5} onChange={(max_freeze_seconds) => change({ ...options, editorial: { ...options.editorial, max_freeze_seconds } })} />
            <FullDemoNumber label="Contexto de las voces (s)" value={options.editorial.voice_context_seconds} max={3} step={0.1} onChange={(voice_context_seconds) => change({ ...options, editorial: { ...options.editorial, voice_context_seconds } })} />
            <FullDemoNumber label="Cola tras morir (s)" value={options.editorial.death_tail_seconds} max={3} step={0.5} onChange={(death_tail_seconds) => change({ ...options, editorial: { ...options.editorial, death_tail_seconds } })} />
            <FullDemoNumber label="Cola si sobrevives (s)" value={options.editorial.round_tail_seconds} max={2} step={0.5} onChange={(round_tail_seconds) => change({ ...options, editorial: { ...options.editorial, round_tail_seconds } })} />
          </div>
          <FullDemoToggle label="Conservar voces durante el freeze" value={options.editorial.keep_freeze_voice} onChange={(keep_freeze_voice) => change({ ...options, editorial: { ...options.editorial, keep_freeze_voice } })} />
          <FullDemoToggle label="Permitir acortar solo las colas si se pierde la primera persona" value={options.editorial.allow_safe_tail_trim} onChange={(allow_safe_tail_trim) => change({ ...options, editorial: { ...options.editorial, allow_safe_tail_trim } })} />
          <div className="flex items-center gap-2"><StatusTag tone="primary">{rounds.length} rondas</StatusTag><span className="text-meta text-fg-3">{dirty ? 'Cambios pendientes de calcular' : 'Plan guardado'}</span></div>
          {rounds.map((round) => <RoundRow key={round.round_id} round={round} options={options} tickRate={document?.clock.tick_rate ?? 64} voice={document?.voice} onChange={change} />)}
        </FullDemoGroup>
      </div>
      <div className="min-w-0 space-y-5">
        <MediaFrame aspect="16:9" fallback={<MapCover map={match.map} />} footer={<span className="text-meta text-fg-2">1920×1080 · 60 fps · primera persona</span>} />
        <FullDemoAudio options={options} document={document} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
        <FullDemoSponsor options={options} document={document} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
        <details className="studio-panel p-4">
          <summary className="cursor-pointer font-display text-body-lg font-semibold uppercase text-fg-1">Avanzado · overlays y portada</summary>
          <div className="mt-4 space-y-3">
            <FullDemoToggle label="Overlay de roster" value={options.overlays.roster} onChange={(roster) => change({ ...options, overlays: { ...options.overlays, roster } })} />
            <FullDemoToggle label="Overlay de marcador" value={options.overlays.scoreboard} onChange={(scoreboard) => change({ ...options, overlays: { ...options.overlays, scoreboard } })} />
            <FullDemoChoice label="Datos del overlay" value={options.overlays.source} options={[{ value: 'demo', label: 'Demo' }, { value: 'faceit', label: 'FACEIT (requiere conexión)' }]} onChange={(source) => change({ ...options, overlays: { ...options.overlays, source } })} />
            <FullDemoChoice label="Tema del overlay" value={options.overlays.theme} options={[{ value: 'faceit-orange', label: 'Naranja' }, { value: 'neon-violet', label: 'Violeta' }]} onChange={(theme) => change({ ...options, overlays: { ...options.overlays, theme } })} />
            <FullDemoChoice label="Portada" value={options.outputs.cover_policy} options={[{ value: 'no-cover', label: 'Sin portada' }, { value: 'generated-gameplay', label: 'Fotograma del gameplay' }]} onChange={(cover_policy) => change({ ...options, outputs: { ...options.outputs, cover_policy } })} />
          </div>
        </details>
      </div>
    </fieldset> : null}
    <div className="space-y-3">
      {busy === 'asset' ? <p role="status" className="text-body-sm text-fg-2">Subiendo y verificando el archivo…</p> : null}
      <Button onClick={() => void plan()} disabled={!options || busy !== null || !isFullDemoOptions(options)} loading={busy === 'plan'} loadingText="Analizando voces y rondas…">{document ? 'Actualizar y guardar plan' : 'Analizar rondas y guardar plan'}</Button>
      {dirty ? <p role="status" className="text-body-sm text-fg-2">Guarda el plan para revisar los nuevos intervalos, archivos y bloqueos. Esta operación no abre CS2.</p> : null}
      {(document?.blockers ?? []).map((item, index) => <p key={`${item.code}-${index}`} role="alert" className="border border-destructive/40 bg-destructive/10 p-3 text-body-sm text-destructive">{item.message}{item.round_id ? ` (${item.round_id})` : ''}</p>)}
      {(document?.warnings ?? []).map((item, index) => <p key={`${item.code}-${index}`} className="text-body-sm text-fg-2">{item.message}</p>)}
      {document ? <a href={`/api/demos/${matchId}/full-demo/plans/${document.plan_id}`} target="_blank" rel="noreferrer" className="text-body-sm text-primary underline">Ver documento del plan · {document.plan_hash.slice(0, 12)}</a> : null}
    </div>
    <ProduceFooter tone="full" eyebrow="Full POV Chill · 16:9" summary={document ? `${rounds.length} rondas · ${recBusy ? 'CS2 ocupado: entrará en cola' : 'listo para revisar'}` : null}
      hint="Completa los ajustes y guarda un plan sin bloqueos para continuar." briefItems={briefItems}
      briefApproved={approved} briefReady={approvalKey !== null && busy === null} onBriefApprovedChange={(value) => setApprovedHash(value ? approvalKey : null)}
      backHref={returnHref} busy={busy === 'create'} error={error}
      cta={<Button variant="stream" size="lg" disabled={!approved || busy !== null} loading={busy === 'create'} loadingText="Encolando…" onClick={() => void create()}>{recBusy ? 'Poner Full Demo en cola' : 'Crear Full Demo'}</Button>} />
  </>;
}

function RoundRow({ round, options, tickRate, voice, onChange }: { round: FullDemoRound; options: FullDemoOptions; tickRate: number; voice: FullDemoDocument['voice'] | undefined; onChange: (options: FullDemoOptions) => void }): ReactNode {
  const custom = options.editorial.manual_ranges.find((range) => range.round_id === round.round_id);
  const start = custom?.start_tick ?? round.requested_start_tick;
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
      <FullDemoNumber label={`R${round.source_round_number}: tick inicial`} value={start} onChange={(value) => range(value, end)} />
      <FullDemoNumber label={`R${round.source_round_number}: tick final`} value={end} onChange={(value) => range(start, value)} />
    </div>
    {custom ? <Button variant="ghost" size="sm" onClick={() => onChange({ ...options, editorial: { ...options.editorial, manual_ranges: options.editorial.manual_ranges.filter((item) => item.round_id !== round.round_id) } })}>Restablecer intervalo automático</Button> : null}
  </details>;
}
