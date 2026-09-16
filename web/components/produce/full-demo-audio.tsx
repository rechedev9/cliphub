'use client';

import { useState, type ReactNode } from 'react';
import type { FullDemoDocument, FullDemoOptions } from '@/lib/full-demo-plan';
import { certifiedRoundId, isPrepareAbort } from '@/lib/produce/sponsor-boundary';
import { FullDemoAssetInput } from './full-demo-asset-input';
import { FullDemoMediaPreview } from './full-demo-media-preview';
import { FullDemoChoice, FullDemoGain, FullDemoGroup, FullDemoNumber, FullDemoToggle } from './full-demo-fields';

type Props = {
  options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void;
  onPrepareRoundBoundaries?: () => Promise<FullDemoDocument | null>;
};

export function FullDemoAudio({ options, document, onChange, onAssetBusy }: Props): ReactNode {
  const { audio } = options;
  const { voice, game } = audio;
  const change = (patch: Partial<FullDemoOptions['audio']>): void => onChange({ ...options, audio: { ...audio, ...patch } });
  let voiceAvailability = 'pendientes de analizar';
  if (!voice.enabled) voiceAvailability = 'desactivadas';
  else if (document) voiceAvailability = voiceStatus(document.voice.availability);
  void onAssetBusy;
  return <FullDemoGroup title="Sonido" note="Audio de partida equilibrado automáticamente, sin música de fondo.">
    <FullDemoToggle label="Incluir voces del equipo" value={voice.enabled} onChange={(enabled) => change({ voice: { ...voice, enabled } })} />
    <p className="text-body-sm text-fg-2" role="status">Voces: {voiceAvailability}.</p>
    <div className="flex flex-col gap-2.5 border-t border-border-subtle pt-3">
      <FullDemoGain label="Juego" value={game.gain} onChange={(gain) => change({ game: { ...game, gain } })} />
      <FullDemoGain label="Voces" value={voice.gain} disabled={!voice.enabled} onChange={(gain) => change({ voice: { ...voice, gain } })} />
    </div>
  </FullDemoGroup>;
}

function voiceStatus(status: string): string {
  const labels: Record<string, string> = { available: 'disponibles', not_requested: 'desactivadas', no_packets: 'la demo no contiene paquetes', no_team_voice: 'no hay paquetes del equipo', silent: 'pistas silenciosas', unsupported: 'formato no compatible', unsupported_codec: 'códec no compatible', decode_failed: 'falló la decodificación' };
  return labels[status] ?? status;
}

export function FullDemoSponsor({ options, document, onChange, onAssetBusy, onPrepareRoundBoundaries }: Props): ReactNode {
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
      <p className="text-meta text-fg-3">Durante el anuncio no suena el juego ni las voces.</p>
      {sponsor.video ? <p className="text-body-sm text-fg-1">Vídeo: {assetName(sponsor.video.id)}</p> : <p className="text-body-sm text-destructive">Añade el vídeo del sponsor o desactívalo.</p>}
      {sponsor.video ? <FullDemoMediaPreview asset={sponsor.video} video label="Previsualizar vídeo del sponsor" gain={sponsor.audio_policy === 'embedded' ? 1 : 0} /> : null}
      <FullDemoAssetInput label={sponsor.video ? 'Cambiar vídeo del sponsor' : 'Añadir vídeo del sponsor y permisos'} accept="video/*" onBusyChange={onAssetBusy} onUploaded={(video) => change({ video })} />
      {sponsor.video ? <>
      <FullDemoChoice label="Audio del anuncio" value={sponsor.audio_policy} options={[{ value: 'embedded', label: 'Audio incluido en el vídeo' }, { value: 'replace-narration', label: 'Reemplazar por narración' }]} onChange={(audio_policy) => change({ audio_policy })} />
      {sponsor.audio_policy === 'replace-narration' ? <>
        {sponsor.narration ? <p className="text-body-sm text-fg-1">Narración: {assetName(sponsor.narration.id)}</p> : null}
        {sponsor.narration ? <FullDemoMediaPreview asset={sponsor.narration} label="Escuchar narración del sponsor" /> : null}
        <FullDemoAssetInput label="Añadir o cambiar narración" accept="audio/*" onBusyChange={onAssetBusy} onUploaded={(narration) => change({ narration })} />
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
