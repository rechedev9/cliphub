'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { parseFailureReason } from '@/lib/api/failure-reason';
import type { Match, Play } from '@/lib/api/types';
import { checkFullDemoCapture, fetchStorageSpace, listVideoVersions, projectCaptureCheck } from '@/lib/api/video-edit';
import { PRODUCE_FORMAT, RESULTS_HREF, STORAGE_HREF, seriesHref, type VideoEdit } from '@/lib/clips/routes';
import type { FullDemoLoadFailure } from '@/lib/full-demo';
import {
  approveFullDemo, currentFullDemoOptions, fullDemoApprovalKey, fullDemoOptionsKey, fullDemoPlanEdit, isFullDemoOptions, loadFullDemoPlan, loadFullDemoPlanVersion, saveFullDemoPlan, withFullDemoRounds,
  FULL_DEMO_CAPTURE_VARIANT, type FullDemoBumperOptions, type FullDemoDocument, type FullDemoOptions, type FullDemoProPlayer,
} from '@/lib/full-demo-plan';
import { captureNote, editCtaLabel, pickVersion, recapSeconds, spaceWarning, versionDateLabel, type CaptureCheck, type SpaceCheck } from '@/lib/produce/edit-capture';
import { FULL_DEMO_VOICE_LABEL, fullDemoBriefItems } from '@/lib/produce/full-demo-brief';
import { FULL_DEMO_ERROR_CODES } from '@/lib/produce/full-demo-notices';
import { humanErrorText } from '@/lib/human-error';
import { fullDemoSubtitles, withFullDemoSubtitles } from '@/lib/produce/subtitles';
import { INTRO_DESIGN_LABEL, INTRO_DESIGNS } from '@/lib/overlay-design-gallery';
import { availableFullDemoBumpers, recallFullDemoBumpers, rememberFullDemoBumpers, withRememberedBumpers } from '@/lib/produce/full-demo-bumper-memory';
import { fullDemoQueueToast } from '@/lib/produce/full-demo-queue-toast';
import { FULL_DEMO_MISSING_FILES, hasMissingFullDemoFiles } from '@/lib/produce/full-demo-requirements';
import { PRODUCE_DRAFT_RESET, PRODUCE_FULL_CTA, PRODUCE_FULL_DRAFT_RESTORED, PRODUCE_FULL_QUEUE_CTA, PRODUCE_FULL_TITLE, PRODUCE_READY_HINT } from '@/lib/produce/copy';
import { formatVideoDuration, fullDemoDurationSeconds, fullDemoOrderRounds } from '@/lib/produce/full-demo-order';
import { prettyMapName } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { LoadingText } from '@/components/ui/loading-text';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { FullDemoFold, FullDemoGroup } from './full-demo-fields';
import { FullDemoAudio } from './full-demo-audio';
import { SubtitlesPanel } from './subtitles-panel';
import { FullDemoBumpers } from './full-demo-bumpers';
import { FullDemoOverlays } from './full-demo-overlays';
import { FullDemoTransitions } from './full-demo-transitions';
import { FullDemoSecondCamera } from './full-demo-second-camera';
import { FullDemoProSettings } from './full-demo-pro-settings';
import { FullDemoHud, FullDemoHudPreview } from './full-demo-hud';
import { FullDemoBrand } from './full-demo-brand';
import { FullDemoActionBar, FullDemoSummary } from './full-demo-summary';
import { useProduceBack } from './use-produce-back';
import { roundListLabel } from '@/lib/library/analysis-video';

/** The design picks how roster and scoreboard look; the demo origin picks their data. */
const OVERLAYS_DETAIL = 'El diseño solo cambia el aspecto y no obliga a volver a grabar. El origen de la demo decide qué datos salen: con FACEIT, nivel y ELO.';
const OVERLAYS_NOTE = `Pantalla de jugadores al empezar y marcador final. ${OVERLAYS_DETAIL}`;
const ONE_ROUND_OVERLAYS_NOTE = `Pantalla de jugadores al empezar. Un vídeo de una ronda acaba con la ronda, sin marcador final. ${OVERLAYS_DETAIL}`;
/** The capture check plans on the server (it hashes the demo), so it waits for the user to stop changing options. */
const CAPTURE_CHECK_DELAY_MS = 1200;
const LINK_CLASS = 'text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1 focus-visible:outline-2 focus-visible:outline-ring';

export type FullPovProducerProps = {
  active: boolean; matchId: string; match: Match; rounds: Play[]; recapFailure: Exclude<FullDemoLoadFailure, null> | null; recBusy: boolean; seriesId: string | null;
  /** From the analysis area: the video covers only these source rounds. */
  selectedRounds?: readonly number[] | null;
  /** From Editar in Mis vídeos: the producer opens with that version's plan. */
  edit?: VideoEdit | null;
};

export function FullPovProducer({ active, matchId, match, rounds: recap, recBusy, seriesId, selectedRounds = null, edit = null }: FullPovProducerProps): ReactNode {
  const router = useRouter();
  const { back } = useProduceBack({ jobId: matchId, format: PRODUCE_FORMAT.full, seriesId });
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
  const [busy, setBusy] = useState<'load' | 'create' | 'asset' | 'prepare' | null>('load');
  const [error, setError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const createRequest = useRef<AbortController | null>(null);
  /** Which rounds a create would record, for the options whose key is `key`. */
  const [captureState, setCaptureState] = useState<{ key: string | null; check: CaptureCheck | null }>({ key: null, check: null });
  const [editedDate, setEditedDate] = useState<string | null>(null);
  const [libraryVoices, setLibraryVoices] = useState<string | null>(null);
  const editVariant = edit?.variant ?? null;
  const editRevision = edit?.revision ?? null;
  const draftKey = `cliphub.full-demo.draft.v1:${matchId}`;
  // A string key, so a new array from the same query does not reload the plan.
  const roundsKey = selectedRounds?.join(',') ?? '';

  useEffect(() => {
    const controller = new AbortController();
    setBusy('load'); setError(null); setDocument(null); setOptions(null); setBaseline(null); setRestored(false); setShowMissing(false);
    setCaptureState({ key: null, check: null }); setEditedDate(null);
    const loadPlan = async (signal: AbortSignal) => {
      if (editVariant === null) return { loaded: await loadFullDemoPlan(matchId, signal), version: null };
      const version = pickVersion(await listVideoVersions({ jobId: matchId, variant: editVariant, signal }), editRevision);
      const loaded = version?.planId ? await loadFullDemoPlanVersion({ jobId: matchId, planId: version.planId, signal }) : await loadFullDemoPlan(matchId, signal);
      return { loaded, version };
    };
    void loadPlan(controller.signal).then(async ({ loaded, version }) => {
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
      // Editar opens the version's own options, never an older draft.
      if (editVariant === null) {
        try {
          const raw = localStorage.getItem(draftKey);
          draft = raw ? JSON.parse(raw) : null;
        } catch { }
      }
      // An old draft may still carry the retired sponsor group: normalizing moves its video to the sponsor slot.
      const drafted = isFullDemoOptions(draft) ? currentFullDemoOptions(draft) : base;
      const initial = focus ? withFullDemoRounds(drafted, focus) : drafted;
      // Saving a plan also stores its options as the draft; only a real divergence is "recovered".
      setRestored(fullDemoOptionsKey(initial) !== fullDemoOptionsKey(base));
      const loadedCapture = projectCaptureCheck(loaded.capture);
      setCaptureState({ key: loadedCapture && loaded.document ? fullDemoOptionsKey(currentFullDemoOptions(loaded.document.options)) : null, check: loadedCapture });
      setEditedDate(version ? versionDateLabel(version.createdAt) : null);
      setDocument(loaded.document); setBaseline(base); setOptions(initial); setProPlayers(loaded.pro_players ?? []); setBusy(null);
    }).catch((failure: unknown) => {
      if (controller.signal.aborted) return;
      setError(humanErrorText({ error: failure, lead: 'No se pudo cargar la preparación de la demo completa.', codes: FULL_DEMO_ERROR_CODES })); setBusy(null);
    });
    return () => controller.abort();
  }, [matchId, draftKey, loadAttempt, roundsKey, editVariant, editRevision]);
  const optionsKey = options === null ? null : fullDemoOptionsKey(options);
  const checkedKey = captureState.key;
  useEffect(() => {
    if (editVariant === null || options === null || optionsKey === null || optionsKey === checkedKey || !isFullDemoOptions(options)) return;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      void checkFullDemoCapture({ jobId: matchId, options, signal: controller.signal }).catch(() => null).then((check) => {
        if (!controller.signal.aborted) setCaptureState({ key: optionsKey, check });
      });
    }, CAPTURE_CHECK_DELAY_MS);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [matchId, editVariant, options, optionsKey, checkedKey]);
  // A new video records every chosen round: its estimate comes from the recap, without planning on the server.
  const [newSpace, setNewSpace] = useState<SpaceCheck | null>(null);
  const newSeconds = editVariant === null ? Math.round(recapSeconds(recap, options?.rounds ?? [])) : 0;
  useEffect(() => {
    if (newSeconds === 0) return;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      void fetchStorageSpace({ captureSeconds: newSeconds, renderSeconds: newSeconds, signal: controller.signal }).catch(() => null).then((space) => {
        if (!controller.signal.aborted) setNewSpace(space);
      });
    }, CAPTURE_CHECK_DELAY_MS);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [newSeconds]);
  // The label keeps the last answer while a newer check runs; only a fresh one promises that CS2 stays closed.
  const capture = captureState.check;
  const freshCapture = checkedKey === optionsKey ? capture : null;
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
  async function prepareMoments(): Promise<void> {
    if (!options || busy) return;
    setBusy('prepare'); setError(null);
    try {
      await saveCurrentPlan();
    } catch (failure) {
      setError(humanErrorText({ error: failure, lead: 'No se pudieron elegir los momentos.', codes: FULL_DEMO_ERROR_CODES }));
    } finally {
      setBusy(null);
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
      // An edited version's plan may be older than the job's current one, so an edit always plans again.
      const planned = editVariant === null && document && fullDemoApprovalKey(document, options) ? document : await saveCurrentPlan(controller.signal);
      if (controller.signal.aborted) return;
      const video = await api.createVideo({ matchId, playIds: planned.rounds.map((round) => round.round_id), mode: 'clean', variant: FULL_DEMO_CAPTURE_VARIANT, editConfig: fullDemoPlanEdit(approveFullDemo(planned)), signal: controller.signal });
      if (controller.signal.aborted) return;
      if (video.status === 'failed') toast.error('La demo completa no ha empezado', { description: parseFailureReason(video.failureReason, { fullDemo: true }).message });
      else if (editVariant !== null && freshCapture?.needed === false) toast('Montaje en cola', { description: 'Studio vuelve a montar el vídeo con las capturas guardadas: CS2 no se abre. La versión que ya tienes se queda en Versiones.' });
      else toast('Demo completa en cola', fullDemoQueueToast(recBusy, planned.warnings));
      router.push(doneHref);
    } catch (failure) {
      if (!controller.signal.aborted) { setError(humanErrorText({ error: failure, lead: 'No se pudo poner el vídeo en cola.', codes: FULL_DEMO_ERROR_CODES })); setBusy(null); }
    } finally {
      if (createRequest.current === controller) createRequest.current = null;
    }
  }

  const chosenRounds = options?.rounds ?? [];
  const voiceAvailability = libraryVoices ?? document?.voice.availability ?? null;
  const briefItems = options ? [
    { label: 'Jugador', value: match.player ?? document?.input.target_steamid64 ?? 'Pendiente' },
    { label: 'Rondas', value: chosenRounds.length > 0 ? roundListLabel(chosenRounds) : 'Todas' },
    ...fullDemoBriefItems({ options, voiceAvailability, hookSkipped: !dirty && document !== null && !document.hook, cameraSkipped: !dirty && document !== null && options.second_camera?.enabled === true && !document.second_camera }),
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
  const editNote = editVariant !== null && capture !== null ? captureNote(capture, planRounds) : null;
  const readyStatus = recBusy && capture?.needed !== false ? 'CS2 ocupado: entrará en cola. Lo encontrarás en Mis vídeos.' : editNote ?? PRODUCE_READY_HINT;
  const shownSpace = editVariant === null ? newSpace : capture?.space;
  const lowSpace = editVariant === null && newSeconds === 0 ? null : spaceWarning(shownSpace);
  const nextStep = canCreate ? readyStatus : 'Al crear, ClipHub comprueba las rondas y avisa si falta algo.';
  const status = busy === null ? nextStep : '';
  const cta = <Button size="lg" disabled={!ready} loading={busy === 'create'} loadingText="Preparando demo completa…" onClick={() => void create()}
    className={`w-full font-extrabold ${FORMAT_TONE.full.fill}`}>{editCtaLabel({ capture, editing: editVariant !== null, fallback: recBusy ? PRODUCE_FULL_QUEUE_CTA : PRODUCE_FULL_CTA })}</Button>;
  const shownError = error ?? (showMissing && missingFiles ? FULL_DEMO_MISSING_FILES : null);
  const briefValue = (label: string): string | undefined => briefItems.find((item) => item.label === label)?.value;
  const compactParts = [planState ?? (chosenRounds.length > 0 ? `${chosenRounds.length} ${chosenRounds.length === 1 ? 'ronda' : 'rondas'}` : 'Todas las rondas'), briefValue('HUD'), briefValue('Voces del equipo')];
  const compactSummary = options ? compactParts.filter((part) => part !== undefined && part !== '').join(' · ') : null;
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
      {editedDate ? <p className="text-body-sm text-fg-3">Editas la versión del {editedDate}. La que ya tienes se queda en Versiones.</p> : null}
      {lowSpace ? (
        <p role="status" className="text-body-sm text-warning">
          {lowSpace}{' '}<Link href={STORAGE_HREF} className={LINK_CLASS}>Liberar espacio en Almacenamiento</Link>
        </p>
      ) : null}
      {restored ? (
        <p role="status" className="text-body-sm text-fg-3">
          {PRODUCE_FULL_DRAFT_RESTORED}{' '}
          <button type="button" disabled={busy !== null} onClick={startOver} className="text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1 focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50">
            {PRODUCE_DRAFT_RESET}
          </button>
        </p>
      ) : null}
    </div>
    {busy === 'load' ? <LoadingText spinner>Cargando la preparación guardada</LoadingText> : null}
    {options === null && busy === null && error ? <Button variant="secondary" className="self-start" onClick={() => setLoadAttempt((attempt) => attempt + 1)}>Reintentar</Button> : null}
    <div className="grid min-w-0 items-start gap-6 @[64rem]/content:grid-cols-[minmax(0,1fr)_minmax(300px,360px)]">
      {options ? <fieldset disabled={busy !== null} inert={busy !== null} className="flex min-w-0 flex-col gap-4">
        <FullDemoHudPreview options={options} />
        <FullDemoHud options={options} onChange={change} onAssetBusy={assetBusy} />
        <FullDemoBumpers options={options} document={document} rounds={orderRounds} onChange={change} onAssetBusy={assetBusy} />
        <FullDemoBrand options={options} onChange={change} />
        <FullDemoAudio options={options} voiceAvailability={voiceAvailability} onChange={change} />
        <FullDemoGroup title="Subtítulos">
          <SubtitlesPanel jobId={matchId} scope={{ kind: 'rounds', rounds: options.rounds ?? null }} voices={options.audio.voice.enabled} voicesLabel={FULL_DEMO_VOICE_LABEL}
            planAvailability={document?.voice.availability ?? null} onAvailability={setLibraryVoices} value={fullDemoSubtitles(options)} onChange={(next) => change(withFullDemoSubtitles(options, next))} />
        </FullDemoGroup>
        <FullDemoFold title={INTRO_DESIGN_LABEL} value={options.overlays.theme === 'hud' ? INTRO_DESIGNS.hud : INTRO_DESIGNS.neon} note={options.overlays.scoreboard ? OVERLAYS_NOTE : ONE_ROUND_OVERLAYS_NOTE}>
          <FullDemoOverlays options={options} map={match.map} onChange={change} onAssetBusy={assetBusy} />
        </FullDemoFold>
        <FullDemoTransitions options={options} onChange={change} />
        <FullDemoSecondCamera options={options} document={document} omitted={capture?.omittedCameras ?? []} preparing={busy === 'prepare'} onChange={change} onPrepare={() => void prepareMoments()} />
        <FullDemoProSettings options={options} players={proPlayers} onChange={change} />
      </fieldset> : <div className="min-w-0" />}
      <div className="flex min-w-0 flex-col gap-4 @[64rem]/content:sticky @[64rem]/content:top-[calc(var(--shell-strip-height)+0.5rem)] @[64rem]/content:max-h-[calc(100dvh-var(--shell-strip-height)-1rem)] @[64rem]/content:overflow-y-auto">
        <FullDemoSummary durationSeconds={durationSeconds} roundCount={orderRounds.length} briefItems={briefItems} planState={planState} status={status}
          blockers={blockers} warnings={dirty ? [] : document?.warnings ?? []} assetBusy={busy === 'asset'}
          error={shownError} back={back} cta={cta} />
      </div>
    </div>
    <FullDemoActionBar planState={compactSummary} status={status} error={shownError} back={back} cta={cta} />
  </>;
}
