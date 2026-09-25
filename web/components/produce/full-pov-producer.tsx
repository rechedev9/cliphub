'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { Match, Play } from '@/lib/api/types';
import { hubHref, seriesHref } from '@/lib/clips/routes';
import type { FullDemoLoadFailure } from '@/lib/full-demo';
import {
  approveFullDemo, bumperSummary, currentFullDemoOptions, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoOverlayLabel, fullDemoOverlaySource, fullDemoPlanEdit, isFullDemoOptions, loadFullDemoPlan, saveFullDemoPlan,
  FULL_DEMO_CAPTURE_VARIANT, type FullDemoDocument, type FullDemoOptions,
} from '@/lib/full-demo-plan';
import { FULL_DEMO_MISSING_FILES, hasMissingFullDemoFiles } from '@/lib/produce/full-demo-requirements';
import { PRODUCE_DRAFT_RESET, PRODUCE_FULL_CTA, PRODUCE_FULL_DRAFT_RESTORED, PRODUCE_FULL_QUEUE_CTA, PRODUCE_FULL_TITLE } from '@/lib/produce/copy';
import { Button } from '@/components/ui/button';
import { ProduceFooter } from './produce-footer';
import { FullDemoGroup } from './full-demo-fields';
import { FullDemoAudio, FullDemoSponsor } from './full-demo-audio';
import { FullDemoBumpers } from './full-demo-bumpers';
import { FullDemoOverlays } from './full-demo-overlays';
import { FullDemoTransitions } from './full-demo-transitions';
import { FullDemoHud } from './full-demo-hud';
import { customHudLabel } from '@/lib/custom-hud';

/** Roster and scoreboard keep ClipHub's own palette whatever the HUD; the demo origin picks their layout. */
const OVERLAYS_NOTE = 'Jugadores y marcador con el diseño de ClipHub, independiente del HUD. El origen de la demo decide su formato.';

export type FullPovProducerProps = {
  active: boolean; matchId: string; match: Match; rounds: Play[]; recapFailure: Exclude<FullDemoLoadFailure, null> | null; recBusy: boolean; seriesId: string | null;
};

export function FullPovProducer({ active, matchId, match, recBusy, seriesId }: FullPovProducerProps): ReactNode {
  const router = useRouter();
  const returnHref = seriesId ? seriesHref(seriesId) : hubHref({ open: matchId });
  const [document, setDocument] = useState<FullDemoDocument | null>(null);
  const [options, setOptions] = useState<FullDemoOptions | null>(null);
  /** What "Empezar de cero" goes back to: the saved plan's options, else the defaults. */
  const [baseline, setBaseline] = useState<FullDemoOptions | null>(null);
  /** A local draft differing from the baseline was restored on load. */
  const [restored, setRestored] = useState(false);
  /** A create attempt hit a missing sponsor/intro/outro file: its hint now reads as an error. */
  const [showMissing, setShowMissing] = useState(false);
  const [busy, setBusy] = useState<'load' | 'plan' | 'create' | 'asset' | null>('load');
  const [error, setError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const createRequest = useRef<AbortController | null>(null);
  const boundaryRequest = useRef<AbortController | null>(null);
  const draftKey = `cliphub.full-demo.draft.v1:${matchId}`;

  useEffect(() => {
    const controller = new AbortController();
    setBusy('load'); setError(null); setDocument(null); setOptions(null); setBaseline(null); setRestored(false); setShowMissing(false);
    void loadFullDemoPlan(matchId, controller.signal).then((loaded) => {
      if (controller.signal.aborted) return;
      const base = currentFullDemoOptions(loaded.document?.options ?? loaded.defaults, true);
      let draft: unknown = null;
      try {
        const raw = localStorage.getItem(draftKey);
        draft = raw ? JSON.parse(raw) : null;
      } catch { }
      const initial = isFullDemoOptions(draft) ? currentFullDemoOptions(draft, true) : base;
      // Saving a plan also stores its options as the draft; only a real divergence is "recovered".
      setRestored(fullDemoOptionsKey(initial) !== fullDemoOptionsKey(base));
      setDocument(loaded.document); setBaseline(base); setOptions(initial); setBusy(null);
    }).catch((failure: unknown) => {
      if (controller.signal.aborted) return;
      setError(failure instanceof Error ? failure.message : 'No se pudo cargar el plan.'); setBusy(null);
    });
    return () => controller.abort();
  }, [matchId, draftKey, loadAttempt]);
  useEffect(() => () => {
    const request = createRequest.current;
    if (request) {
      createRequest.current = null;
      request.abort();
    }
    const boundary = boundaryRequest.current;
    if (boundary) {
      boundaryRequest.current = null;
      boundary.abort();
    }
  }, [matchId]);
  useEffect(() => {
    if (!active) {
      const request = createRequest.current;
      if (request) {
        createRequest.current = null;
        request.abort();
        setBusy((current) => current === 'create' ? null : current);
      }
      const boundary = boundaryRequest.current;
      if (boundary) {
        boundaryRequest.current = null;
        boundary.abort();
        setBusy((current) => current === 'plan' ? null : current);
      }
    }
  }, [active]);

  function change(next: FullDemoOptions): void {
    next = currentFullDemoOptions(next);
    setOptions(next);
    try { localStorage.setItem(draftKey, JSON.stringify(next)); } catch { }
  }
  function startOver(): void {
    if (!baseline || busy) return;
    try { localStorage.removeItem(draftKey); } catch { }
    setOptions(baseline); setRestored(false); setShowMissing(false); setError(null);
  }
  const ready = options !== null && busy === null && isFullDemoOptions(options);
  const missingFiles = options !== null && hasMissingFullDemoFiles(options);
  const dirty = options !== null && (document === null || fullDemoOptionsKey(document.options) !== fullDemoOptionsKey(options));
  const rounds = document?.rounds ?? [];
  const savedPlanStatus = recBusy ? 'CS2 ocupado: entrará en cola' : 'Plan guardado';

  async function saveCurrentPlan(signal?: AbortSignal): Promise<FullDemoDocument> {
    if (!options) throw new Error('No se pudo preparar el plan.');
    const planned = await saveFullDemoPlan(matchId, options, signal);
    if (signal?.aborted) throw new DOMException('La preparación se canceló.', 'AbortError');
    setDocument(planned); setOptions(planned.options);
    try { localStorage.setItem(draftKey, JSON.stringify(planned.options)); } catch { }
    return planned;
  }
  async function prepareSponsorRoundBoundaries(): Promise<FullDemoDocument | null> {
    if (!options || busy) return null;
    const controller = new AbortController();
    boundaryRequest.current = controller;
    setBusy('plan'); setError(null);
    try {
      return await saveCurrentPlan(controller.signal);
    } catch (failure) {
      if (controller.signal.aborted) throw failure instanceof DOMException && failure.name === 'AbortError' ? failure : new DOMException('La preparación se canceló.', 'AbortError');
      setError(failure instanceof Error ? failure.message : 'No se pudieron preparar las rondas.');
      return null;
    } finally {
      if (boundaryRequest.current === controller) {
        boundaryRequest.current = null;
        setBusy(null);
      }
    }
  }
  async function create(): Promise<void> {
    if (!options || busy) return;
    if (hasMissingFullDemoFiles(options)) {
      setShowMissing(true);
      return;
    }
    const controller = new AbortController();
    createRequest.current = controller;
    setBusy('create'); setError(null);
    try {
      const planned = document && fullDemoApprovalKey(document, options) ? document : await saveCurrentPlan(controller.signal);
      if (controller.signal.aborted) return;
      await api.createVideo({ matchId, playIds: planned.rounds.map((round) => round.round_id), mode: 'clean', variant: FULL_DEMO_CAPTURE_VARIANT, editConfig: fullDemoPlanEdit(approveFullDemo(planned)), signal: controller.signal });
      if (controller.signal.aborted) return;
      toast('Vídeo largo en cola', { description: recBusy ? 'Empezará cuando quede libre CS2.' : 'Sigue el progreso en Demos y vídeos.' });
      router.push(returnHref);
    } catch (failure) {
      if (!controller.signal.aborted) { setError(failure instanceof Error ? failure.message : 'No se pudo encolar el vídeo.'); setBusy(null); }
    } finally {
      if (createRequest.current === controller) createRequest.current = null;
    }
  }

  const briefItems = options ? [
    { label: 'Jugador', value: match.player ?? document?.input.target_steamid64 ?? 'Pendiente' },
    { label: 'HUD', value: customHudLabel(options.overlays.hud_theme) },
    { label: 'POV', value: options.capture.trueview ? 'Original 1:1' : 'Estándar' },
    { label: 'Voces', value: options.audio.voice.enabled ? 'Incluidas' : 'Sin voces' },
    { label: 'Transiciones', value: options.transitions?.enabled ? 'Dinámico' : 'Corte limpio' },
    { label: 'Sponsor', value: options.sponsor.enabled ? 'Incluido' : 'Desactivado' },
    { label: 'Intro y outro', value: bumperSummary(options) },
    { label: 'Overlays', value: fullDemoOverlayLabel(fullDemoOverlaySource(options)) },
  ] : [];

  return <>
    <div className="mb-2 flex min-w-0 flex-wrap items-baseline gap-x-4 gap-y-1">
      <p className="wrap-anywhere font-mono text-meta uppercase tracking-ultra text-fg-3">Nuevo vídeo largo · {match.map}{match.player ? ` · ${match.player}` : ''}</p>
      <h1 className="order-first font-display text-display-sm font-bold text-fg-1">{PRODUCE_FULL_TITLE}</h1>
      <p className="w-full text-body-sm text-fg-2">Todas las rondas con la mira del jugador y el audio de la partida.</p>
      {restored ? (
        <p role="status" className="w-full text-body-sm text-fg-3">
          {PRODUCE_FULL_DRAFT_RESTORED}{' '}
          <button type="button" disabled={busy !== null} onClick={startOver} className="text-primary underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50">
            {PRODUCE_DRAFT_RESET}
          </button>
        </p>
      ) : null}
    </div>
    {busy === 'load' ? <p role="status" className="text-body-sm text-fg-2">Cargando la preparación guardada…</p> : null}
    {options === null && busy === null && error ? <Button variant="secondary" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>Reintentar conexión</Button> : null}
    {/*
      The HUD leads full-width. The fixed cards (sound, transitions, overlays) stack in one
      column; the optional ones, which grow with their upload forms, fill the other(s).
      2 columns: HUD on top, fixed cards | sponsor over intro/outro.
      3 columns: HUD (2) with the fixed cards beside it, sponsor | intro/outro right under the HUD.
    */}
    {options ? <fieldset disabled={busy !== null} inert={busy !== null}
      className="grid min-w-0 items-start gap-4 @[40rem]/content:grid-cols-2 @[40rem]/content:grid-rows-[auto_auto_1fr] @[64rem]/content:grid-cols-3 @[64rem]/content:grid-rows-[auto_1fr]">
      <div className="min-w-0 @[40rem]/content:col-span-2"><FullDemoHud options={options} map={match.map} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} /></div>
      <div className="min-w-0 space-y-4 @[40rem]/content:row-span-2 @[64rem]/content:col-start-3 @[64rem]/content:row-start-1">
        <FullDemoAudio options={options} document={document} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
        <FullDemoTransitions options={options} onChange={change} />
        <FullDemoGroup title="Overlays" note={OVERLAYS_NOTE}>
          <FullDemoOverlays options={options} map={match.map} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
        </FullDemoGroup>
      </div>
      <FullDemoGroup title="Sponsor" note="Opcional. Añade un vídeo para incluirlo."><FullDemoSponsor options={options} document={document} showMissing={showMissing} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} onPrepareRoundBoundaries={prepareSponsorRoundBoundaries} /></FullDemoGroup>
      <FullDemoBumpers options={options} document={document} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
    </fieldset> : null}
    <div className="space-y-3">
      {busy === 'asset' ? <p role="status" className="text-body-sm text-fg-2">Subiendo y verificando el archivo…</p> : null}
      {(document?.blockers ?? []).map((item, index) => <p key={`${item.code}-${index}`} role="alert" className="border border-destructive/40 bg-destructive/10 p-3 text-body-sm text-destructive">{item.message}{item.round_id ? ` (${item.round_id})` : ''}</p>)}
      {!dirty ? (document?.warnings ?? []).map((item, index) => <p key={`${item.code}-${index}`} className="text-body-sm text-fg-2">{item.message}</p>) : null}
    </div>
    <ProduceFooter tone="full" eyebrow="Vídeo largo · 16:9" summary={document ? `${rounds.length} rondas · ${dirty ? 'Se preparará al crear' : savedPlanStatus}` : null}
      hint="Crear comprueba las rondas y bloqueos antes de encolar el vídeo." briefItems={briefItems}
      readyHint={dirty ? 'Se preparará al crear.' : undefined}
      ready={ready} backHref={returnHref} busy={busy !== null} error={error ?? (showMissing && missingFiles ? FULL_DEMO_MISSING_FILES : null)}
      cta={<Button variant="hero" size="sm" disabled={!ready} loading={busy === 'create'} loadingText="Preparando vídeo largo…" onClick={() => void create()}
        className="neon-notch shrink-0 focus-visible:-outline-offset-4">{recBusy ? PRODUCE_FULL_QUEUE_CTA : PRODUCE_FULL_CTA}</Button>} />
  </>;
}
