'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { parseFailureReason } from '@/lib/api/failure-reason';
import type { Match, Play } from '@/lib/api/types';
import { flowHref, PRODUCE_FORMAT, RESULTS_HREF, seriesHref } from '@/lib/clips/routes';
import type { FullDemoLoadFailure } from '@/lib/full-demo';
import {
  approveFullDemo, bumperSummary, currentFullDemoOptions, FULL_DEMO_BUMPERS_LABEL, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoOverlayLabel, fullDemoOverlaySource, fullDemoPlanEdit, isFullDemoOptions, loadFullDemoPlan, saveFullDemoPlan, withFullDemoRounds,
  FULL_DEMO_CAPTURE_VARIANT, type FullDemoBumperOptions, type FullDemoDocument, type FullDemoOptions, type FullDemoProPlayer,
} from '@/lib/full-demo-plan';
import { proSettingsSummary } from '@/lib/produce/full-demo-pro-settings';
import { availableFullDemoBumpers, recallFullDemoBumpers, rememberFullDemoBumpers, withRememberedBumpers } from '@/lib/produce/full-demo-bumper-memory';
import { fullDemoQueueToast } from '@/lib/produce/full-demo-queue-toast';
import { FULL_DEMO_MISSING_FILES, hasMissingFullDemoFiles } from '@/lib/produce/full-demo-requirements';
import { PRODUCE_DRAFT_RESET, PRODUCE_FULL_CTA, PRODUCE_FULL_DRAFT_RESTORED, PRODUCE_FULL_QUEUE_CTA, PRODUCE_FULL_TITLE, PRODUCE_READY_HINT } from '@/lib/produce/copy';
import { formatVideoDuration, fullDemoDurationSeconds, fullDemoOrderRounds } from '@/lib/produce/full-demo-order';
import { prettyMapName } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { FullDemoGroup, FullDemoPart } from './full-demo-fields';
import { FullDemoAudio } from './full-demo-audio';
import { FullDemoBumpers } from './full-demo-bumpers';
import { FullDemoOverlays } from './full-demo-overlays';
import { FullDemoTransitions } from './full-demo-transitions';
import { FullDemoProSettings } from './full-demo-pro-settings';
import { FullDemoHud, FullDemoHudPreview } from './full-demo-hud';
import { FullDemoActionBar, FullDemoSummary } from './full-demo-summary';
import { customHudLabel } from '@/lib/custom-hud';
import { roundListLabel } from '@/lib/library/analysis-video';

/** Roster and scoreboard keep ClipHub's own palette whatever the HUD; the demo origin picks their layout. */
const OVERLAYS_NOTE = 'Pantalla de jugadores al empezar y marcador final, con el diseño de ClipHub. El origen de la demo decide qué datos salen: con FACEIT, nivel y ELO.';

export type FullPovProducerProps = {
  active: boolean; matchId: string; match: Match; rounds: Play[]; recapFailure: Exclude<FullDemoLoadFailure, null> | null; recBusy: boolean; seriesId: string | null;
  /** From the analysis area: the video covers only these source rounds. */
  selectedRounds?: readonly number[] | null;
};

export function FullPovProducer({ active, matchId, match, rounds: recap, recBusy, seriesId, selectedRounds = null }: FullPovProducerProps): ReactNode {
  const router = useRouter();
  const backHref = seriesId ? seriesHref(seriesId, PRODUCE_FORMAT.full) : flowHref(PRODUCE_FORMAT.full);
  const doneHref = seriesId ? seriesHref(seriesId, PRODUCE_FORMAT.full) : RESULTS_HREF;
  const [document, setDocument] = useState<FullDemoDocument | null>(null);
  const [options, setOptions] = useState<FullDemoOptions | null>(null);
  const [proPlayers, setProPlayers] = useState<readonly FullDemoProPlayer[]>([]);
  /** What "Empezar de cero" goes back to: the saved plan's options, else the defaults. */
  const [baseline, setBaseline] = useState<FullDemoOptions | null>(null);
  /** A local draft differing from the baseline was restored on load. */
  const [restored, setRestored] = useState(false);
  /** A create attempt hit a missing intro/sponsor/outro file: the footer now reports it as an error. */
  const [showMissing, setShowMissing] = useState(false);
  const [busy, setBusy] = useState<'load' | 'create' | 'asset' | null>('load');
  const [error, setError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const createRequest = useRef<AbortController | null>(null);
  const draftKey = `cliphub.full-demo.draft.v1:${matchId}`;
  // A string key, so a new array from the same query does not reload the plan.
  const roundsKey = selectedRounds?.join(',') ?? '';

  useEffect(() => {
    const controller = new AbortController();
    setBusy('load'); setError(null); setDocument(null); setOptions(null); setBaseline(null); setRestored(false); setShowMissing(false);
    void loadFullDemoPlan(matchId, controller.signal).then(async (loaded) => {
      if (controller.signal.aborted) return;
      // A demo that was never planned opens with the intro, sponsor and outro chosen last time.
      let remembered: FullDemoBumperOptions | undefined;
      if (!loaded.document) {
        const recalled = recallFullDemoBumpers();
        remembered = await availableFullDemoBumpers(recalled, controller.signal);
        if (controller.signal.aborted) return;
        if (recalled && JSON.stringify(remembered) !== JSON.stringify(recalled)) rememberFullDemoBumpers(remembered);
      }
      const focus = roundsKey === '' ? null : roundsKey.split(',').map(Number);
      const saved = currentFullDemoOptions(loaded.document?.options ?? withRememberedBumpers(loaded.defaults, remembered));
      const base = focus ? withFullDemoRounds(saved, focus) : saved;
      let draft: unknown = null;
      try {
        const raw = localStorage.getItem(draftKey);
        draft = raw ? JSON.parse(raw) : null;
      } catch { }
      // An old draft may still carry the retired sponsor group: normalizing moves its video to the sponsor slot.
      const drafted = isFullDemoOptions(draft) ? currentFullDemoOptions(draft) : base;
      const initial = focus ? withFullDemoRounds(drafted, focus) : drafted;
      // Saving a plan also stores its options as the draft; only a real divergence is "recovered".
      setRestored(fullDemoOptionsKey(initial) !== fullDemoOptionsKey(base));
      setDocument(loaded.document); setBaseline(base); setOptions(initial); setProPlayers(loaded.pro_players ?? []); setBusy(null);
    }).catch((failure: unknown) => {
      if (controller.signal.aborted) return;
      setError(failure instanceof Error ? failure.message : 'No se pudo cargar el plan.'); setBusy(null);
    });
    return () => controller.abort();
  }, [matchId, draftKey, loadAttempt, roundsKey]);
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
      if (request) {
        createRequest.current = null;
        request.abort();
        setBusy((current) => current === 'create' ? null : current);
      }
    }
  }, [active]);

  function change(next: FullDemoOptions): void {
    next = currentFullDemoOptions(next);
    // Only an actual clip choice updates the memory: editing another option of an older plan without clips must not forget them.
    if (JSON.stringify(next.bumpers ?? null) !== JSON.stringify(options?.bumpers ?? null)) rememberFullDemoBumpers(next.bumpers);
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

  async function saveCurrentPlan(signal?: AbortSignal): Promise<FullDemoDocument> {
    if (!options) throw new Error('No se pudo preparar el plan.');
    const planned = await saveFullDemoPlan(matchId, options, signal);
    if (signal?.aborted) throw new DOMException('La preparación se canceló.', 'AbortError');
    setDocument(planned); setOptions(planned.options);
    try { localStorage.setItem(draftKey, JSON.stringify(planned.options)); } catch { }
    return planned;
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
      const video = await api.createVideo({ matchId, playIds: planned.rounds.map((round) => round.round_id), mode: 'clean', variant: FULL_DEMO_CAPTURE_VARIANT, editConfig: fullDemoPlanEdit(approveFullDemo(planned)), signal: controller.signal });
      if (controller.signal.aborted) return;
      if (video.status === 'failed') toast.error('La demo completa no ha empezado', { description: parseFailureReason(video.failureReason, { fullDemo: true }).message });
      else toast('Demo completa en cola', fullDemoQueueToast(recBusy, planned.warnings));
      router.push(doneHref);
    } catch (failure) {
      if (!controller.signal.aborted) { setError(failure instanceof Error ? failure.message : 'No se pudo encolar el vídeo.'); setBusy(null); }
    } finally {
      if (createRequest.current === controller) createRequest.current = null;
    }
  }

  const chosenRounds = options?.rounds ?? [];
  const briefItems = options ? [
    { label: 'Jugador', value: match.player ?? document?.input.target_steamid64 ?? 'Pendiente' },
    { label: 'Rondas', value: chosenRounds.length > 0 ? roundListLabel(chosenRounds) : 'Todas' },
    { label: 'HUD', value: customHudLabel(options.overlays.hud_theme) },
    { label: 'POV', value: options.capture.trueview ? 'Original 1:1' : 'Estándar' },
    { label: 'Configuración', value: proSettingsSummary(options.capture.pro_settings) },
    { label: 'Voces', value: options.audio.voice.enabled ? 'Incluidas' : 'Sin voces' },
    { label: 'Transiciones', value: options.transitions?.enabled ? 'Dinámico' : 'Corte limpio' },
    { label: FULL_DEMO_BUMPERS_LABEL, value: bumperSummary(options) },
    { label: 'Presentación', value: fullDemoOverlayLabel(fullDemoOverlaySource(options)) },
  ] : [];

  const orderRounds = fullDemoOrderRounds({ chosen: chosenRounds, planned: document ? document.rounds.map((round) => round.source_round_number) : null, recap: recap.map((play) => play.round) });
  // A timeline only describes the saved options; once they change it no longer tells the length.
  const durationSeconds = document && !dirty ? fullDemoDurationSeconds(document) : null;
  const assetBusy = (value: boolean): void => setBusy(value ? 'asset' : null);
  const blockers = document?.blockers ?? [];
  // Blockers or missing files only surface on create, so the ready text would promise too much.
  const canCreate = ready && !missingFiles && blockers.length === 0;
  const planRounds = orderRounds.length > 0 ? orderRounds.length : document?.rounds.length ?? 0;
  const planState = document ? `${planRounds} ${planRounds === 1 ? 'ronda' : 'rondas'} · ${dirty ? 'Se aplicarán al crear' : 'Ajustes guardados'}` : null;
  const readyStatus = recBusy ? 'CS2 ocupado: entrará en cola. Lo encontrarás en Mis vídeos.' : PRODUCE_READY_HINT;
  const nextStep = canCreate ? readyStatus : 'Al crear, ClipHub comprueba las rondas y avisa si falta algo.';
  const status = busy === null ? nextStep : '';
  const cta = <Button size="lg" disabled={!ready} loading={busy === 'create'} loadingText="Preparando demo completa…" onClick={() => void create()}
    className={`w-full font-extrabold ${FORMAT_TONE.full.fill}`}>{recBusy ? PRODUCE_FULL_QUEUE_CTA : PRODUCE_FULL_CTA}</Button>;
  const shownError = error ?? (showMissing && missingFiles ? FULL_DEMO_MISSING_FILES : null);
  const title = match.player ? `POV de ${match.player} en ${prettyMapName(match.map)}` : PRODUCE_FULL_TITLE;

  return <>
    <div className="mb-4 flex min-w-0 flex-col gap-1.5">
      <h1 className="font-display text-display-sm font-extrabold tracking-tight text-fg-1">{title}</h1>
      <p className="text-body-sm text-fg-2">
        {chosenRounds.length > 0
          ? `${chosenRounds.length === 1 ? 'La ronda' : 'Las rondas'} ${roundListLabel(chosenRounds)} desde la vista del jugador, con el audio de la partida.`
          : 'Todas las rondas desde la vista del jugador, con el audio de la partida.'}
        {durationSeconds !== null ? ` Unos ${formatVideoDuration(durationSeconds)} de vídeo.` : null}
        {chosenRounds.length > 0 && options ? (
          <>
            {' '}
            <button type="button" disabled={busy !== null} onClick={() => change(withFullDemoRounds(options, null))}
              className="text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1 focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50">
              Usar todas las rondas
            </button>
          </>
        ) : null}
      </p>
      {restored ? (
        <p role="status" className="text-body-sm text-fg-3">
          {PRODUCE_FULL_DRAFT_RESTORED}{' '}
          <button type="button" disabled={busy !== null} onClick={startOver} className="text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1 focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50">
            {PRODUCE_DRAFT_RESET}
          </button>
        </p>
      ) : null}
    </div>
    {busy === 'load' ? <p role="status" className="text-body-sm text-fg-2">Cargando la preparación guardada…</p> : null}
    {options === null && busy === null && error ? <Button variant="secondary" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>Reintentar conexión</Button> : null}
    <div className="grid min-w-0 items-start gap-6 @[64rem]/content:grid-cols-[minmax(0,1fr)_minmax(300px,360px)]">
      {options ? <fieldset disabled={busy !== null} inert={busy !== null} className="flex min-w-0 flex-col gap-4">
        <FullDemoHudPreview options={options} map={match.map} />
        <FullDemoBumpers options={options} document={document} rounds={orderRounds} onChange={change} onAssetBusy={assetBusy} />
        <FullDemoHud options={options} onChange={change} onAssetBusy={assetBusy} />
        <div className="grid min-w-0 items-start gap-4 grid-cols-[repeat(auto-fit,minmax(min(100%,300px),1fr))]">
          <FullDemoAudio options={options} document={document} onChange={change} />
          <FullDemoGroup title="Grabación">
            <div className="flex flex-col gap-3.5">
              <FullDemoProSettings options={options} players={proPlayers} onChange={change} />
              <FullDemoPart title="Presentación y marcador" note={OVERLAYS_NOTE}>
                <FullDemoOverlays options={options} map={match.map} onChange={change} onAssetBusy={assetBusy} />
              </FullDemoPart>
              <FullDemoTransitions options={options} onChange={change} />
            </div>
          </FullDemoGroup>
        </div>
      </fieldset> : <div className="min-w-0" />}
      <FullDemoSummary durationSeconds={durationSeconds} roundCount={orderRounds.length} briefItems={briefItems} planState={planState} status={status}
        blockers={blockers} warnings={dirty ? [] : document?.warnings ?? []} assetBusy={busy === 'asset'}
        error={shownError} backHref={backHref} cta={cta} />
    </div>
    <FullDemoActionBar planState={planState} status={status} error={shownError} backHref={backHref} cta={cta} />
  </>;
}
