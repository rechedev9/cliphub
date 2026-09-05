'use client';

import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import type { FullDemoDocument, FullDemoOptions } from '@/lib/full-demo-plan';
import { FullDemoAssetInput } from './full-demo-asset-input';
import { FullDemoMediaPreview } from './full-demo-media-preview';
import { FullDemoChoice, FullDemoGroup, FullDemoNumber, FullDemoToggle } from './full-demo-fields';

type Props = { options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void };

export function FullDemoAudio({ options, document, onChange, onAssetBusy }: Props): ReactNode {
  const { audio } = options;
  const { voice, game, music } = audio;
  const change = (patch: Partial<FullDemoOptions['audio']>): void => onChange({ ...options, audio: { ...audio, ...patch } });
  const changeMusic = (patch: Partial<typeof music>): void => change({ music: { ...music, ...patch } });
  return <FullDemoGroup title="Audio" note="Juego y voces del equipo en buses separados. La música se pausa durante el sponsor y continúa donde se quedó.">
    <div className="grid gap-4 sm:grid-cols-2">
      <FullDemoNumber label="Volumen del juego" value={game.gain} max={2} step={0.05} onChange={(gain) => change({ game: { ...game, gain } })} />
      <FullDemoNumber label="Volumen de las voces" value={voice.gain} max={2} step={0.05} onChange={(gain) => change({ voice: { ...voice, gain } })} />
    </div>
    <FullDemoToggle label="Incluir voces del equipo" value={voice.enabled} onChange={(enabled) => change({ voice: { ...voice, enabled } })} />
    <p className="text-body-sm text-fg-2" role="status">Voces: {document ? voiceStatus(document.voice.availability) : 'pendientes de analizar'}.</p>
    <div className="grid gap-4 sm:grid-cols-2">
      <FullDemoChoice label="Si faltan voces utilizables" value={voice.approved_fallback} options={[{ value: 'block', label: 'Bloquear y avisarme' }, { value: 'without-voice', label: 'Acepto continuar sin voces' }]} onChange={(approved_fallback) => change({ voice: { ...voice, approved_fallback } })} />
      <FullDemoChoice label="Nivel de voces" value={voice.normalization} options={[{ value: 'bounded-activity-v1', label: 'Equilibrar actividad (±9 dB)' }, { value: 'none', label: 'Conservar nivel original' }]} onChange={(normalization) => change({ voice: { ...voice, normalization } })} />
    </div>
    <FullDemoToggle label="Bajar el juego cuando habla el equipo" value={game.voice_priority} onChange={(voice_priority) => change({ game: { ...game, voice_priority } })} />
    <div className="border-t border-border-subtle pt-3">
      <FullDemoToggle label="Música de fondo" value={music.enabled} onChange={(enabled) => changeMusic({ enabled })} />
      {music.enabled ? <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <FullDemoNumber label="Fondo musical (dB)" value={music.bed_gain_db} min={-24} max={-18} step={0.5} onChange={(bed_gain_db) => changeMusic({ bed_gain_db })} />
          <FullDemoChoice label="Al terminar la playlist" value={music.loop_policy} options={[{ value: 'ordered-loop', label: 'Repetir en este orden' }, { value: 'once-pad-silence', label: 'Continuar sin música' }]} onChange={(loop_policy) => changeMusic({ loop_policy })} />
        </div>
        <p className="text-meta text-fg-3">Cada pista se normaliza a −16 LUFS antes de aplicar el nivel de fondo.</p>
        {music.assets.length === 0 ? <p className="text-body-sm text-destructive">Añade al menos una pista o desactiva la música.</p> : null}
        <ol className="space-y-2" aria-label="Orden de la playlist">
          {music.assets.map((ref, index) => <li key={`${ref.id}-${index}`} className="flex flex-wrap items-center gap-2 border border-border-subtle p-2">
            <span className="min-w-0 flex-1 text-body-sm text-fg-1">{index + 1}. {document?.assets?.find((asset) => asset.ref.id === ref.id)?.title ?? `Pista ${ref.sha256.slice(0, 8)}`}</span>
            <Button size="sm" variant="ghost" disabled={index === 0} aria-label={`Subir pista ${index + 1}`} onClick={() => {
              const prior = music.assets[index - 1]; if (!prior) return;
              const assets = [...music.assets]; assets[index - 1] = ref; assets[index] = prior; changeMusic({ assets });
            }}>Subir</Button>
            <Button size="sm" variant="ghost" aria-label={`Quitar pista ${index + 1}`} onClick={() => changeMusic({ assets: music.assets.filter((_, item) => item !== index) })}>Quitar</Button>
            <FullDemoMediaPreview asset={ref} label={`Escuchar pista ${index + 1}`} />
          </li>)}
        </ol>
        {music.assets.length < 20 ? <FullDemoAssetInput label="Añadir música y permisos" accept="audio/*" onBusyChange={onAssetBusy} onUploaded={(ref) => changeMusic({ assets: [...music.assets, ref] })} /> : null}
        <FullDemoToggle label="Bajar la música durante voces y acción" value={music.ducking.enabled} onChange={(enabled) => changeMusic({ ducking: { ...music.ducking, enabled } })} />
        <details className="border-t border-border-subtle pt-3">
          <summary className="cursor-pointer text-body-sm text-fg-2">Ajustes de reducción de música</summary>
          <div className="mt-3 grid gap-4 sm:grid-cols-2">
            <FullDemoNumber label="Influencia del juego (0–1)" value={music.ducking.game_contribution} max={1} step={0.05} onChange={(game_contribution) => changeMusic({ ducking: { ...music.ducking, game_contribution } })} />
            <FullDemoNumber label="Ataque (ms)" value={music.ducking.attack_ms} min={1} max={2000} onChange={(attack_ms) => changeMusic({ ducking: { ...music.ducking, attack_ms } })} />
            <FullDemoNumber label="Recuperación (ms)" value={music.ducking.release_ms} min={20} max={5000} onChange={(release_ms) => changeMusic({ ducking: { ...music.ducking, release_ms } })} />
            <FullDemoNumber label="Umbral (0,001–1)" value={music.ducking.threshold} min={0.001} max={1} step={0.001} onChange={(threshold) => changeMusic({ ducking: { ...music.ducking, threshold } })} />
            <FullDemoNumber label="Ratio de reducción" value={music.ducking.ratio} min={1} max={20} step={0.5} onChange={(ratio) => changeMusic({ ducking: { ...music.ducking, ratio } })} />
          </div>
        </details>
      </div> : null}
    </div>
    <p className="border-t border-border-subtle pt-3 text-meta text-fg-3">Entrega completa: −14 LUFS ±0,5 · pico máximo −1,5 dBTP · AAC estéreo, 48 kHz.</p>
  </FullDemoGroup>;
}

function voiceStatus(status: string): string {
  const labels: Record<string, string> = { available: 'disponibles', not_requested: 'desactivadas', no_packets: 'la demo no contiene paquetes', no_team_voice: 'no hay paquetes del equipo', silent: 'pistas silenciosas', unsupported: 'formato no compatible', unsupported_codec: 'códec no compatible', decode_failed: 'falló la decodificación' };
  return labels[status] ?? status;
}

export function FullDemoSponsor({ options, document, onChange, onAssetBusy }: Props): ReactNode {
  const { sponsor } = options;
  const change = (patch: Partial<typeof sponsor>): void => onChange({ ...options, sponsor: { ...sponsor, ...patch } });
  const assetName = (id: string | undefined): string => document?.assets?.find((asset) => asset.ref.id === id)?.title ?? 'Archivo pendiente de revisar en el plan';
  return <FullDemoGroup title="Sponsor" note="Clip de vídeo completo con su audio o una narración de reemplazo. Sin juego, voces del equipo ni música de fondo durante el anuncio.">
    <FullDemoToggle label="Incluir sponsor" value={sponsor.enabled} onChange={(enabled) => change({ enabled })} />
    {sponsor.enabled ? <div className="space-y-4">
      {sponsor.video ? <p className="text-body-sm text-fg-1">Vídeo: {assetName(sponsor.video.id)}</p> : <p className="text-body-sm text-destructive">Añade el vídeo del sponsor o desactívalo.</p>}
      {sponsor.video ? <FullDemoMediaPreview asset={sponsor.video} video label="Previsualizar vídeo del sponsor" gain={sponsor.audio_policy === 'embedded' ? 1 : 0} /> : null}
      <FullDemoAssetInput label={sponsor.video ? 'Cambiar vídeo del sponsor' : 'Añadir vídeo del sponsor y permisos'} accept="video/*" onBusyChange={onAssetBusy} onUploaded={(video) => change({ video })} />
      <FullDemoChoice label="Audio del anuncio" value={sponsor.audio_policy} options={[{ value: 'embedded', label: 'Audio incluido en el vídeo' }, { value: 'replace-narration', label: 'Reemplazar por narración' }]} onChange={(audio_policy) => change({ audio_policy })} />
      {sponsor.audio_policy === 'replace-narration' ? <>
        {sponsor.narration ? <p className="text-body-sm text-fg-1">Narración: {assetName(sponsor.narration.id)}</p> : null}
        {sponsor.narration ? <FullDemoMediaPreview asset={sponsor.narration} label="Escuchar narración del sponsor" /> : null}
        <FullDemoAssetInput label="Añadir o cambiar narración" accept="audio/*" onBusyChange={onAssetBusy} onUploaded={(narration) => change({ narration })} />
        <FullDemoChoice label="Si la narración dura menos que el vídeo" value={sponsor.short_narration_policy} options={[{ value: 'block', label: 'Bloquear y avisarme' }, { value: 'pad-silence', label: 'Acepto silencio al final' }]} onChange={(short_narration_policy) => change({ short_narration_policy })} />
      </> : null}
      <FullDemoChoice label="Colocación" value={sponsor.placement_policy} options={[{ value: 'first-two-rounds', label: 'Después de R2 o R1, en la ventana' }, { value: 'round-boundary', label: 'Después de una ronda concreta' }, { value: 'manual-frame', label: 'Instante exacto del vídeo' }]} onChange={(placement_policy) => change({ placement_policy, ...(placement_policy === 'manual-frame' ? { manual_start_frame: sponsor.manual_start_frame ?? 6000 } : {}), ...(placement_policy === 'round-boundary' ? { after_round_id: sponsor.after_round_id || document?.rounds[0]?.round_id || '' } : {}) })} />
      <div className="grid gap-4 sm:grid-cols-2">
        <FullDemoNumber label="Ventana: desde (s)" value={sponsor.window_start_seconds} max={43200} onChange={(window_start_seconds) => change({ window_start_seconds })} />
        <FullDemoNumber label="Ventana: hasta (s)" value={sponsor.window_end_seconds} max={43200} onChange={(window_end_seconds) => change({ window_end_seconds })} />
      </div>
      {sponsor.placement_policy === 'round-boundary' ? <FullDemoChoice label="Insertar después de" value={sponsor.after_round_id} options={(document?.rounds ?? []).map((round) => ({ value: round.round_id, label: `Ronda ${round.source_round_number}` }))} onChange={(after_round_id) => change({ after_round_id })} /> : null}
      {sponsor.placement_policy === 'manual-frame' ? <>
        <FullDemoNumber label="Fotograma de inserción (60 = 1 segundo)" value={sponsor.manual_start_frame ?? 0} max={2592000} onChange={(manual_start_frame) => change({ manual_start_frame })} />
        <FullDemoToggle label="Acepto dividir una ronda en este punto" value={sponsor.allow_split_round} onChange={(allow_split_round) => change({ allow_split_round })} />
      </> : null}
      {document?.sponsor_placement.duration_frames ? <p className="text-body-sm text-fg-2">Último plan: anuncio en {(document.sponsor_placement.start_frame / 60).toFixed(2)} s · duración {(document.sponsor_placement.duration_frames / 60).toFixed(2)} s.</p> : null}
    </div> : null}
  </FullDemoGroup>;
}
