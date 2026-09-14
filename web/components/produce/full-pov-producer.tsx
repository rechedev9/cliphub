'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { Match, Play } from '@/lib/api/types';
import { hubHref, seriesHref } from '@/lib/clips/routes';
import type { FullDemoLoadFailure } from '@/lib/full-demo';
import {
  approveFullDemo, currentFullDemoOptions, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoOverlayLabel, fullDemoOverlaySource, fullDemoPlanEdit, isFullDemoOptions, loadFullDemoPlan, saveFullDemoPlan,
  FULL_DEMO_CAPTURE_VARIANT, type FullDemoDocument, type FullDemoOptions,
} from '@/lib/full-demo-plan';
import { Button } from '@/components/ui/button';
import { ProduceFooter } from './produce-footer';
import { FullDemoGroup } from './full-demo-fields';
import { FullDemoAudio, FullDemoSponsor } from './full-demo-audio';
import { FullDemoOverlays } from './full-demo-overlays';
import { FullDemoTransitions } from './full-demo-transitions';
import { FullDemoHud } from './full-demo-hud';
import { customHudLabel } from '@/lib/custom-hud';

export type FullPovProducerProps = {
  active: boolean; matchId: string; match: Match; rounds: Play[]; recapFailure: Exclude<FullDemoLoadFailure, null> | null; recBusy: boolean; seriesId: string | null;
};

/** Full POV stays in the existing generate flow; the server's editorial plan owns every decision. */
export function FullPovProducer({ active, matchId, match, recBusy, seriesId }: FullPovProducerProps): ReactNode {
  const router = useRouter();
  const returnHref = seriesId ? seriesHref(seriesId) : hubHref({ open: matchId });
  const [document, setDocument] = useState<FullDemoDocument | null>(null);
  const [options, setOptions] = useState<FullDemoOptions | null>(null);
  const [busy, setBusy] = useState<'load' | 'create' | 'asset' | null>('load');
  const [error, setError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const createRequest = useRef<AbortController | null>(null);
  const draftKey = `cliphub.full-demo.draft.v1:${matchId}`;

  useEffect(() => {
    const controller = new AbortController();
    setBusy('load'); setError(null); setDocument(null); setOptions(null);
    void loadFullDemoPlan(matchId, controller.signal).then((loaded) => {
      if (controller.signal.aborted) return;
      let initial = loaded.document?.options ?? loaded.defaults;
      try {
        const raw = localStorage.getItem(draftKey);
        const draft: unknown = raw ? JSON.parse(raw) : null;
        if (isFullDemoOptions(draft)) initial = draft;
      } catch { /* The durable server plan remains available when local drafts are unavailable. */ }
      initial = currentFullDemoOptions(initial, true);
      setDocument(loaded.document); setOptions(initial); setBusy(null);
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
  }, [matchId]);
  useEffect(() => {
    if (!active) {
      const request = createRequest.current;
      if (!request) return;
      createRequest.current = null;
      request.abort();
      // The producer remains mounted while the format is hidden. Release only
      // this request's create state so returning to Full Demo is usable, while
      // a new match load or request keeps its own busy state intact.
      setBusy((current) => current === 'create' ? null : current);
    }
  }, [active]);

  function change(next: FullDemoOptions): void {
    next = currentFullDemoOptions(next);
    setOptions(next);
    try { localStorage.setItem(draftKey, JSON.stringify(next)); } catch { /* Saving the server plan is still explicit and durable. */ }
  }
  // Creation prepares any changed draft first, then binds only that approved
  // document to the capture request. A stale plan can never be enqueued.
  const ready = options !== null && busy === null && isFullDemoOptions(options);
  const dirty = options !== null && (document === null || fullDemoOptionsKey(document.options) !== fullDemoOptionsKey(options));
  const rounds = document?.rounds ?? [];
  const savedPlanStatus = recBusy ? 'CS2 ocupado: entrará en cola' : 'Plan guardado';

  async function saveCurrentPlan(signal?: AbortSignal): Promise<FullDemoDocument> {
    if (!options) throw new Error('No se pudo preparar el plan.');
    const planned = await saveFullDemoPlan(matchId, options, signal);
    if (signal?.aborted) throw new DOMException('La preparación se canceló.', 'AbortError');
    setDocument(planned); setOptions(planned.options);
    try { localStorage.setItem(draftKey, JSON.stringify(planned.options)); } catch { /* The plan was saved durably by the server. */ }
    return planned;
  }
  async function create(): Promise<void> {
    if (!options || busy) return;
    const controller = new AbortController();
    createRequest.current = controller;
    setBusy('create'); setError(null);
    try {
      const planned = document && fullDemoApprovalKey(document, options) ? document : await saveCurrentPlan(controller.signal);
      if (controller.signal.aborted) return;
      await api.createVideo({ matchId, playIds: planned.rounds.map((round) => round.round_id), mode: 'clean', variant: FULL_DEMO_CAPTURE_VARIANT, editConfig: fullDemoPlanEdit(approveFullDemo(planned)), signal: controller.signal });
      if (controller.signal.aborted) return;
      toast('Full Demo en cola', { description: recBusy ? 'Empezará cuando quede libre CS2.' : 'Sigue el progreso en Demos y vídeos.' });
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
    { label: 'Voces', value: options.audio.voice.enabled ? 'Incluidas' : 'Sin voces' },
    { label: 'Transiciones', value: options.transitions?.enabled ? 'Dinámico' : 'Corte limpio' },
    { label: 'Sponsor', value: options.sponsor.enabled ? 'Incluido' : 'Desactivado' },
    { label: 'Overlays', value: `${fullDemoOverlayLabel(fullDemoOverlaySource(options))} · neón violeta` },
  ] : [];

  return <>
    <div className="min-w-0 space-y-2">
      <p className="wrap-anywhere font-mono text-meta uppercase tracking-ultra text-fg-3">Vídeo largo · {match.map}{match.player ? ` · ${match.player}` : ''}</p>
      <h1 className="font-display text-display-sm font-bold uppercase text-fg-1">Full POV Chill</h1>
      <p className="max-w-3xl text-body-sm text-fg-2">Todas las rondas con la mira del jugador y el audio de la partida.</p>
    </div>
    {busy === 'load' ? <p role="status" className="text-body-sm text-fg-2">Cargando la preparación guardada…</p> : null}
    {options === null && busy === null && error ? <Button variant="secondary" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>Reintentar conexión</Button> : null}
    {options ? <fieldset disabled={busy !== null} inert={busy !== null} className="grid min-w-0 items-start gap-5 @[56rem]/content:grid-cols-[minmax(0,1fr)_minmax(300px,0.8fr)]">
      <div className="min-w-0 @[56rem]/content:col-span-2"><FullDemoHud options={options} map={match.map} onChange={change} /></div>
      <div className="min-w-0 space-y-5">
        <FullDemoAudio options={options} document={document} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
        <FullDemoTransitions options={options} onChange={change} />
      </div>
      <div className="min-w-0 space-y-5">
        <FullDemoGroup title="Overlays" note="Jugadores y marcador en neón violeta.">
          <FullDemoOverlays options={options} map={match.map} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} />
        </FullDemoGroup>
        <FullDemoGroup title="Sponsor" note="Opcional; se mantiene desactivado hasta que añadas una pieza."><FullDemoSponsor options={options} document={document} onChange={change} onAssetBusy={(value) => setBusy(value ? 'asset' : null)} /></FullDemoGroup>
      </div>
    </fieldset> : null}
    <div className="space-y-3">
      {busy === 'asset' ? <p role="status" className="text-body-sm text-fg-2">Subiendo y verificando el archivo…</p> : null}
      {dirty ? <p role="status" className="text-body-sm text-fg-2">Los cambios se revisarán al crear el vídeo.</p> : null}
      {(document?.blockers ?? []).map((item, index) => <p key={`${item.code}-${index}`} role="alert" className="border border-destructive/40 bg-destructive/10 p-3 text-body-sm text-destructive">{item.message}{item.round_id ? ` (${item.round_id})` : ''}</p>)}
      {!dirty ? (document?.warnings ?? []).map((item, index) => <p key={`${item.code}-${index}`} className="text-body-sm text-fg-2">{item.message}</p>) : null}
      {document ? <a href={`/api/demos/${matchId}/full-demo/plans/${document.plan_id}`} target="_blank" rel="noreferrer" className="text-meta text-fg-3 underline">Documento del plan · {document.plan_hash.slice(0, 12)}</a> : null}
    </div>
    <ProduceFooter tone="full" eyebrow="Full POV Chill · 16:9" summary={document ? `${rounds.length} rondas · ${dirty ? 'Se preparará al crear' : savedPlanStatus}` : null}
      hint="Crear comprueba las rondas y bloqueos antes de encolar el vídeo." briefItems={briefItems}
      readyHint={dirty ? 'Se preparará al crear.' : undefined}
      ready={ready} backHref={returnHref} busy={busy !== null} error={error}
      cta={<Button variant="stream" size="lg" disabled={!ready} loading={busy === 'create'} loadingText="Preparando Full Demo…" onClick={() => void create()}>{recBusy ? 'Poner Full Demo en cola' : 'Crear Full Demo'}</Button>} />
  </>;
}
