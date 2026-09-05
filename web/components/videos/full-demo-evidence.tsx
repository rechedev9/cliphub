import Link from 'next/link';
import type { ReactNode } from 'react';
import type { Video } from '@/lib/api/types';
import { pinRenderRevision } from '@/lib/api/render-revision';

export function FullDemoEvidence({ video }: { video: Video }): ReactNode {
  const snapshot = video.editConfig?.fullDemo;
  if (!snapshot || !video.jobId) return null;
  const options = snapshot.document.options;
  const complete = video.status === 'ready' || video.status === 'review_required';
  return <section className="studio-panel space-y-3 p-4" aria-label="Plan y verificación de Full Demo">
    <h2 className="font-display text-body-lg font-semibold text-fg-1">Plan y verificación</h2>
    <p className="text-body-sm text-fg-2">{snapshot.document.rounds.length} rondas · voces {options.audio.voice.enabled ? `${options.audio.voice.gain}×` : 'desactivadas'} · música {options.audio.music.enabled ? `${options.audio.music.assets.length} pistas` : 'desactivada'} · sponsor {options.sponsor.enabled ? 'incluido' : 'desactivado'}.</p>
    <p className="font-mono text-meta text-fg-3">Plan aprobado: {snapshot.approval.approved_plan_hash.slice(0, 12)}</p>
    <div className="flex flex-wrap gap-x-4 gap-y-2 text-body-sm text-primary">
      <Link href={`/clips/${video.jobId}/nuevo?formato=full`} className="underline">Revisar ajustes y crear otra revisión</Link>
      {complete ? ['approved', 'effective', 'audio', 'loudness', 'delivery'].map((document, index) => <a key={document}
        href={pinRenderRevision(`/api/demos/${video.jobId}/renders/${video.variant ?? 'gameplay-pov-60'}/full-demo/${document}`, video.artifactRevision)} target="_blank" rel="noreferrer" className="underline">
        {['Plan aprobado', 'Plan efectivo', 'Mezcla de audio', 'Loudness medido', 'Verificación del MP4'][index]}
      </a>) : null}
    </div>
    <p className="text-meta text-fg-3">Los documentos del render muestran sus intervalos efectivos y mediciones. Reintentar conserva el plan aprobado.</p>
  </section>;
}
