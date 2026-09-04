'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { isOverlayTheme, OVERLAY_THEME, type Match, type OverlayTheme, type Play } from '@/lib/api/types';
import { hubHref, seriesHref } from '@/lib/clips/routes';
import { PRODUCE_FULL_ROUNDS_NOTE, PRODUCE_FULL_TITLE } from '@/lib/produce/copy';
import {
  canStartFullDemoCapture,
  FULL_DEMO_EMPTY,
  FULL_DEMO_FORGE_HINT_EMPTY,
  FULL_DEMO_FORGE_HINT_ERROR,
  FULL_DEMO_OVERLAY_THEME_OPTIONS,
  FULL_DEMO_RECAP_ERROR,
  FULL_DEMO_ROUNDS_PENDING,
  FULL_DEMO_VARIANT,
  FULL_DEMO_VOICE_VOLUME,
  fullDemoBriefItems,
  fullDemoEdit,
  fullDemoOverlayThemeLabel,
  type FullDemoLoadFailure,
} from '@/lib/full-demo';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { MapCover } from '@/components/brand/map-cover';
import { MediaFrame } from '@/components/studio/media-frame';
import { StatusTag } from '@/components/studio/status-tag';
import { ProduceFooter } from './produce-footer';

/**
 * The spec's Intro/outro, Comms and HUD toggles are read-only here: those values
 * are the locked Full POV contract (`fullDemoEdit`), not choices.
 */
const LOCKED_ROWS = [
  { label: 'Intro / outro', value: 'No' },
  { label: 'Voces del equipo', value: `${Math.round(FULL_DEMO_VOICE_VOLUME * 100)}%` },
  { label: 'HUD nativo', value: 'Visible' },
] as const;

export type FullPovProducerProps = {
  matchId: string;
  match: Match;
  /** Recap rounds from `api.findRecapClips`; empty while the plan is pending. */
  rounds: Play[];
  recapFailure: Exclude<FullDemoLoadFailure, null> | null;
  /** Another Full POV is on CS2: this one queues behind it. */
  recBusy: boolean;
  /** From a bo3/bo5 series map: producing returns to the series, not the hub. */
  seriesId: string | null;
};

/** The Full POV constructor: every round of the plan, one overlay theme, locked contract, REC. */
export function FullPovProducer({ matchId, match, rounds, recapFailure, recBusy, seriesId }: FullPovProducerProps): ReactNode {
  const router = useRouter();
  const returnHref = seriesId ? seriesHref(seriesId) : hubHref({ open: matchId });
  const [overlayTheme, setOverlayTheme] = useState<OverlayTheme>(OVERLAY_THEME.faceitOrange);
  const [briefApproved, setBriefApproved] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // The rounds array identity changes on every poll, so approval keys on the
  // plan's digest instead: a real round change revokes it, a re-fetch does not.
  const roundsDigest = rounds.map((round) => round.id).join(',');
  useEffect(() => {
    setBriefApproved(false);
  }, [overlayTheme, roundsDigest, recBusy]);

  const roundCount = rounds.length;
  const roundsPending = recapFailure === null && roundCount === 0;
  const themeLabel = fullDemoOverlayThemeLabel(overlayTheme);
  const briefItems = [...fullDemoBriefItems(), { label: 'Tema overlays', value: themeLabel }];
  const ready = canStartFullDemoCapture({ roundCount, briefApproved, creating });

  async function onCreate(): Promise<void> {
    if (!ready) return;
    setCreating(true);
    setCreateError(null);
    try {
      await api.createVideo({
        matchId,
        playIds: rounds.map((round) => round.id),
        mode: 'clean',
        variant: FULL_DEMO_VARIANT,
        editConfig: fullDemoEdit(overlayTheme),
      });
      if (recBusy) toast('Vídeo largo en cola', { description: 'Empieza al acabar el REC actual' });
      else toast('REC iniciado', { description: 'CS2 + HLAE grabando · no toques el juego' });
      router.push(returnHref);
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'No se pudo encolar el vídeo largo.');
      setCreating(false);
    }
  }

  let recapAlert: string | null = null;
  if (recapFailure === 'offline') recapAlert = `${FULL_DEMO_EMPTY.offline.title}. ${FULL_DEMO_EMPTY.offline.description}`;
  else if (recapFailure === 'error') recapAlert = FULL_DEMO_RECAP_ERROR;

  const summary =
    roundCount > 0 ? (
      <>
        {roundCount} {roundCount === 1 ? 'ronda' : 'rondas'}
        <span className="text-fg-3"> · </span>
        <span className="text-stream-text">{themeLabel}</span>
        <span className="text-fg-3"> · FACEIT · 1080p60</span>
        {recBusy ? (
          <>
            <span className="text-fg-3"> · </span>
            <span className="text-stream-text">CS2 ocupado · entrará en cola</span>
          </>
        ) : null}
      </>
    ) : null;

  return (
    <>
      <div className="grid items-start gap-6 @[56rem]/content:grid-cols-[minmax(0,1fr)_320px]">
        <section className="flex min-w-0 flex-col gap-3.5">
          <div className="flex flex-col gap-1.5">
            <p className="font-mono text-meta uppercase tracking-ultra text-fg-3">
              Vídeo largo · {match.map}
              {match.player ? ` · ${match.player}` : ''}
            </p>
            <h1 className="font-display text-display-sm font-bold uppercase text-fg-1">{PRODUCE_FULL_TITLE}</h1>
            <p className="text-body-sm text-fg-2">Un vídeo horizontal con todas las rondas del jugador, HUD nativo y voces del equipo. Revisa el plan y confirma los ajustes para iniciar la grabación.</p>
          </div>

          <div className="flex flex-wrap items-center gap-2.5">
            <StatusTag tone="primary">
              <span className="tabular-nums">{roundCount}</span> {roundCount === 1 ? 'ronda' : 'rondas'}
            </StatusTag>
            <StatusTag>Todas las rondas del POV</StatusTag>
            <span className="font-mono text-meta tracking-normal text-fg-3">{PRODUCE_FULL_ROUNDS_NOTE}</span>
          </div>

          {recapAlert ? (
            <p role="alert" className="border border-destructive/40 bg-destructive/10 px-4 py-3 text-body-sm text-destructive">
              {recapAlert}
            </p>
          ) : null}

          <div className="studio-panel flex flex-col overflow-hidden">
            <div className="flex items-center justify-between gap-3 border-b border-border-subtle bg-surface-3 px-3.5 py-2.5 font-mono text-meta uppercase tracking-ultra text-fg-3">
              Rondas
              <span className="tracking-wider text-primary">
                <span className="tabular-nums">{roundCount}</span> de <span className="tabular-nums">{roundCount}</span>
              </span>
            </div>
            {roundsPending ? (
              <p className="flex items-center gap-2.5 px-3.5 py-4 text-body-sm text-fg-2" role="status" aria-live="polite">
                <span className="studio-spinner text-primary" aria-hidden />
                {FULL_DEMO_ROUNDS_PENDING}
              </p>
            ) : null}
            {rounds.map((round) => (
              <RoundRow key={round.id} round={round} />
            ))}
          </div>
        </section>

        <aside className="flex flex-col gap-3 @[56rem]/content:sticky @[56rem]/content:top-20">
          <MediaFrame
            aspect="16:9"
            className="border border-border-accent"
            fallback={<MapCover map={match.map} />}
            footer={
              <span className="font-mono text-meta uppercase text-fg-2">HUD nativo + overlay {themeLabel}</span>
            }
          />

          <div className="studio-panel flex flex-col gap-2.5 px-3.5 py-3">
            <label htmlFor="full-pov-theme" className="font-mono text-meta uppercase tracking-ultra text-fg-3">
              Overlays
            </label>
            <Select
              value={overlayTheme}
              onValueChange={(value) => {
                if (isOverlayTheme(value)) setOverlayTheme(value);
              }}
              disabled={creating}
            >
              <SelectTrigger id="full-pov-theme" aria-label="Tema de overlays FACEIT" className="h-10 font-display font-semibold uppercase">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {FULL_DEMO_OVERLAY_THEME_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    Tema {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="studio-panel flex flex-col">
            <p className="px-3.5 pt-3 pb-2.5 font-mono text-meta uppercase tracking-ultra text-fg-3">
              Incluido en tu vídeo largo
            </p>
            <dl className="flex flex-col divide-y divide-border-subtle" aria-label="Ajustes del vídeo largo">
              {LOCKED_ROWS.map((row) => (
                <div key={row.label} className="flex items-center justify-between gap-3 px-3.5 py-3">
                  <dt className="font-mono text-meta uppercase tracking-ultra text-fg-3">{row.label}</dt>
                  <dd className="font-display text-body-sm font-semibold uppercase text-fg-1">{row.value}</dd>
                </div>
              ))}
            </dl>
          </div>
        </aside>
      </div>

      <div className="flex-1" />

      <ProduceFooter
        tone="full"
        eyebrow="Vídeo largo · 16:9"
        summary={summary}
        hint={recapFailure ? FULL_DEMO_FORGE_HINT_ERROR : FULL_DEMO_FORGE_HINT_EMPTY}
        briefItems={briefItems}
        briefNote={
          <div className="mt-3 border-l-2 border-primary bg-primary/8 px-3 py-2.5 text-body-sm text-fg-2">
            <span className="font-mono text-meta uppercase tracking-wider text-primary">FACEIT obligatorio</span>
            <span className="ml-2">ClipHub verifica el perfil y el historial de todos los jugadores antes de abrir HLAE.</span>
          </div>
        }
        briefApproved={briefApproved}
        briefReady={roundCount > 0}
        onBriefApprovedChange={setBriefApproved}
        backHref={returnHref}
        busy={creating}
        error={createError}
        cta={
          <Button
            variant="stream"
            size="lg"
            disabled={!ready}
            loading={creating}
            loadingText={recBusy ? 'Encolando…' : 'Iniciando REC…'}
            onClick={() => void onCreate()}
            className="neon-notch shrink-0 font-display uppercase tracking-wide focus-visible:-outline-offset-4"
          >
            <span aria-hidden className="size-2.5 rounded-full bg-current" />
            {recBusy ? 'Poner en cola el vídeo' : 'Crear vídeo largo'}
          </Button>
        }
      />
    </>
  );
}

/* Every recap play is `kind: 'highlight'` on the wire, so the tag keys on kills. */
function roundTag(round: Play): string | null {
  if (round.kills >= 5) return 'ACE';
  if (round.kills >= 3) return 'Highlight';
  return null;
}

function RoundRow({ round }: { round: Play }): ReactNode {
  const tag = roundTag(round);
  const detail = round.weapon
    ? `${round.weapon} · ${round.kills} ${round.kills === 1 ? 'kill' : 'kills'}`
    : `${round.kills} ${round.kills === 1 ? 'kill' : 'kills'}`;
  return (
    <div className="flex items-center gap-3.5 border-b border-border-subtle px-3.5 py-2.5 last:border-b-0">
      <span className="w-11 shrink-0 font-mono text-body-sm tabular-nums text-fg-1">R{String(round.round).padStart(2, '0')}</span>
      <span className="min-w-0 flex-1 truncate font-mono text-meta uppercase tracking-wider text-fg-2">{detail}</span>
      {tag ? <StatusTag tone="primary">{tag}</StatusTag> : null}
    </div>
  );
}
