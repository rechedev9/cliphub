'use client';

import { useState, type ReactNode } from 'react';
import type { FullDemoDocument, FullDemoOptions } from '@/lib/full-demo-plan';
import { certifiedRoundId, isPrepareAbort } from '@/lib/produce/sponsor-boundary';
import { FullDemoAssetInput } from './full-demo-asset-input';
import { FullDemoMediaPreview } from './full-demo-media-preview';
import { FullDemoChoice, FullDemoGain, FullDemoGroup, FullDemoMissing, FullDemoNumber, FullDemoToggle } from './full-demo-fields';

export const FULL_DEMO_SPONSOR_MISSING = 'Añade el vídeo del sponsor o desactívalo.';

type Props = {
  options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void;
  onPrepareRoundBoundaries?: () => Promise<FullDemoDocument | null>;
  /** A create attempt was blocked: a missing required file turns from a hint into an error. */
  showMissing?: boolean;
};

export function FullDemoAudio({ options, document, onChange, onAssetBusy }: Props): ReactNode {
  const { audio } = options;
  const { voice, game } = audio;
  const change = (patch: Partial<FullDemoOptions['audio']>): void => onChange({ ...options, audio: { ...audio, ...patch } });
  // The checkbox already says when voices are off; the line only reports what the demo holds.
  const availability = voice.enabled ? voiceStatus(document?.voice.availability ?? null) : null;
  void onAssetBusy;
  return <FullDemoGroup title="Sonido" note="Audio de partida equilibrado automáticamente, sin música de fondo.">
    <FullDemoToggle label="Incluir voces del equipo" value={voice.enabled} onChange={(enabled) => change({ voice: { ...voice, enabled } })} />
    {availability ? <p className={availability.problem ? 'text-body-sm text-warning' : 'text-body-sm text-fg-2'} role="status">{availability.text}</p> : null}
    <div className="flex flex-col gap-2.5 border-t border-border-subtle pt-3">
      <FullDemoGain label="Juego" value={game.gain} onChange={(gain) => change({ game: { ...game, gain } })} />
      <FullDemoGain label="Voces" value={voice.gain} disabled={!voice.enabled} onChange={(gain) => change({ voice: { ...voice, gain } })} />
    </div>
  </FullDemoGroup>;
}

const VOICE_PENDING = 'Las voces del equipo se comprueban al preparar el vídeo.';
const VOICE_STATUS: Record<string, { text: string; problem: boolean }> = {
  available: { text: 'Voces del equipo disponibles en esta demo.', problem: false },
  // Planned while voices were off: the next preparation checks them.
  not_requested: { text: VOICE_PENDING, problem: false },
  no_packets: { text: 'Esta demo no contiene voces.', problem: true },
  no_team_voice: { text: 'Esta demo no tiene voces del equipo del jugador.', problem: true },
  silent: { text: 'Las voces del equipo están en silencio en esta demo.', problem: true },
  unsupported: { text: 'El formato de voz de esta demo no es compatible.', problem: true },
  unsupported_codec: { text: 'El códec de voz de esta demo no es compatible.', problem: true },
  decode_failed: { text: 'No se pudieron leer las voces de esta demo.', problem: true },
};

/** `null` means no plan has analysed the demo yet. */
function voiceStatus(status: string | null): { text: string; problem: boolean } {
  if (status === null) return { text: VOICE_PENDING, problem: false };
  return VOICE_STATUS[status] ?? { text: `Estado de las voces: ${status}.`, problem: true };
}

export function FullDemoSponsor({ options, document, onChange, onAssetBusy, onPrepareRoundBoundaries, showMissing = false }: Props): ReactNode {
  const { sponsor } = options;
  const [preparingBoundary, setPreparingBoundary] = useState(false);
  const [boundaryError, setBoundaryError] = useState<string | null>(null);
  const change = (patch: Partial<typeof sponsor>): void => onChange({ ...options, sponsor: { ...sponsor, ...patch } });
  const assetName = (id: string | undefined): string => document?.assets?.find((asset) => asset.ref.id === id)?.title ?? 'Archivo pendiente de revisar en el plan';
  const candidates = document?.sponsor_placement.candidates ?? [];
  const boundaryOptions = candidates.map((candidate) => {
    const round = document?.rounds.find((entry) => entry.round_id === candidate.after_round_id);
    return { value: candidate.after_round_id, label: round ? `Ronda ${round.source_round_number}` : candidate.after_round_id };
  });
  async function selectPlacement(placement_policy: typeof sponsor.placement_policy): Promise<void> {
    if (placement_policy !== 'round-boundary') {
      setBoundaryError(null);
      change({ placement_policy, ...(placement_policy === 'manual-frame' ? { manual_start_frame: sponsor.manual_start_frame ?? 6000 } : {}) });
      return;
    }
    const after_round_id = certifiedRoundId(candidates.map((candidate) => candidate.after_round_id), sponsor.after_round_id);
    if (after_round_id) {
      setBoundaryError(null);
      change({ placement_policy, after_round_id });
      return;
    }
    if (!onPrepareRoundBoundaries) return;
    setPreparingBoundary(true); setBoundaryError(null);
    try {
      const planned = await onPrepareRoundBoundaries();
      const prepared = certifiedRoundId((planned?.sponsor_placement.candidates ?? []).map((candidate) => candidate.after_round_id), sponsor.after_round_id);
      if (!planned || !prepared) {
        setBoundaryError('No hay una ronda certificada disponible para el sponsor.');
        return;
      }
      onChange({ ...planned.options, sponsor: { ...planned.options.sponsor, placement_policy, after_round_id: prepared } });
    } catch (failure) {
      if (!isPrepareAbort(failure)) setBoundaryError(failure instanceof Error ? failure.message : 'No hay una ronda certificada disponible para el sponsor.');
    } finally {
      setPreparingBoundary(false);
    }
  }
  return <>
    <FullDemoToggle label="Incluir sponsor" value={sponsor.enabled} onChange={(enabled) => change({ enabled })} />
    {sponsor.enabled ? <div className="space-y-4">
      <p className="text-body-sm text-fg-2">Durante el anuncio no suena el juego ni las voces.</p>
      {sponsor.video ? <p className="text-body-sm text-fg-1">Vídeo: {assetName(sponsor.video.id)}</p> : <FullDemoMissing error={showMissing}>{FULL_DEMO_SPONSOR_MISSING}</FullDemoMissing>}
      {sponsor.video ? <FullDemoMediaPreview asset={sponsor.video} video label="Previsualizar vídeo del sponsor" gain={sponsor.audio_policy === 'embedded' ? 1 : 0} /> : null}
      <FullDemoAssetInput label={sponsor.video ? 'Cambiar vídeo del sponsor' : 'Añadir vídeo del sponsor y permisos'} open={!sponsor.video} accept="video/*" onBusyChange={onAssetBusy} onUploaded={(video) => change({ video })} />
      {sponsor.video ? <>
      <FullDemoChoice label="Audio del anuncio" value={sponsor.audio_policy} options={[{ value: 'embedded', label: 'Audio incluido en el vídeo' }, { value: 'replace-narration', label: 'Reemplazar por narración' }]} onChange={(audio_policy) => change({ audio_policy })} />
      {sponsor.audio_policy === 'replace-narration' ? <>
        {sponsor.narration ? <p className="text-body-sm text-fg-1">Narración: {assetName(sponsor.narration.id)}</p> : null}
        {sponsor.narration ? <FullDemoMediaPreview asset={sponsor.narration} label="Escuchar narración del sponsor" /> : null}
        <FullDemoAssetInput label="Añadir o cambiar narración" open={!sponsor.narration} accept="audio/*" onBusyChange={onAssetBusy} onUploaded={(narration) => change({ narration })} />
        <FullDemoChoice label="Si la narración dura menos que el vídeo" value={sponsor.short_narration_policy} options={[{ value: 'block', label: 'Bloquear y avisarme' }, { value: 'pad-silence', label: 'Acepto silencio al final' }]} onChange={(short_narration_policy) => change({ short_narration_policy })} />
      </> : null}
      <FullDemoChoice label="Colocación" value={sponsor.placement_policy} options={[{ value: 'first-two-rounds', label: 'Después de R2 o R1, en la ventana' }, { value: 'round-boundary', label: 'Después de una ronda concreta' }, { value: 'manual-frame', label: 'Instante exacto del vídeo' }]} onChange={(placement_policy) => void selectPlacement(placement_policy)} />
      {preparingBoundary ? <p role="status" className="text-body-sm text-fg-2">Preparando las rondas disponibles…</p> : null}
      {boundaryError ? <p role="alert" className="text-body-sm text-destructive">{boundaryError}</p> : null}
      <div className="grid gap-4 sm:grid-cols-2">
        <FullDemoNumber label="Ventana: desde (s)" value={sponsor.window_start_seconds} max={43200} onChange={(window_start_seconds) => change({ window_start_seconds })} />
        <FullDemoNumber label="Ventana: hasta (s)" value={sponsor.window_end_seconds} max={43200} onChange={(window_end_seconds) => change({ window_end_seconds })} />
      </div>
      {sponsor.placement_policy === 'round-boundary' ? <FullDemoChoice label="Insertar después de" value={sponsor.after_round_id} options={boundaryOptions} onChange={(after_round_id) => change({ after_round_id })} /> : null}
      {sponsor.placement_policy === 'manual-frame' ? <>
        <FullDemoNumber label="Fotograma de inserción (60 = 1 segundo)" value={sponsor.manual_start_frame ?? 0} max={2592000} onChange={(manual_start_frame) => change({ manual_start_frame })} />
        <FullDemoToggle label="Acepto dividir una ronda en este punto" value={sponsor.allow_split_round} onChange={(allow_split_round) => change({ allow_split_round })} />
      </> : null}
      {document?.sponsor_placement.duration_frames ? <p className="text-body-sm text-fg-2">Último plan: anuncio en {(document.sponsor_placement.start_frame / 60).toFixed(2)} s · duración {(document.sponsor_placement.duration_frames / 60).toFixed(2)} s.</p> : null}
      </> : null}
    </div> : null}
  </>;
}
