import Link from 'next/link';
import type { ReactNode } from 'react';
import type { Video } from '@/lib/api/types';
import { pinRenderRevision } from '@/lib/api/render-revision';
import { PRODUCE_FORMAT, produceHref } from '@/lib/clips/routes';
import { bumperSummary } from '@/lib/full-demo-plan';
import { formatRate } from '@/lib/format';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';

const RENDER_DOCUMENTS = [
  { id: 'approved', label: 'Plan aprobado' },
  { id: 'effective', label: 'Plan efectivo' },
  { id: 'audio', label: 'Mezcla de audio' },
  { id: 'loudness', label: 'Loudness medido' },
  { id: 'delivery', label: 'Verificación del MP4' },
] as const;

const LINK_CLASS = 'underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1';

/** The Full Demo's plan and render documents: closed by default, they are for checking a render, not for publishing. */
export function FullDemoEvidence({ video }: { video: Video }): ReactNode {
  const snapshot = video.editConfig?.fullDemo;
  if (!snapshot || !video.jobId) return null;
  const options = snapshot.document.options;
  const complete = video.status === 'ready';
  return <details className="studio-panel p-4" aria-label="Detalles técnicos del render">
    <DisclosureSummary className="min-h-10 font-display text-body-lg font-semibold text-fg-1">Detalles técnicos del render</DisclosureSummary>
    <div className="mt-3 space-y-3">
      <p className="text-body-sm text-fg-2">{snapshot.document.rounds.length} rondas · voces {options.audio.voice.enabled ? formatRate(options.audio.voice.gain) : 'desactivadas'} · música {options.audio.music.enabled ? `${options.audio.music.assets.length} pistas` : 'desactivada'} · vídeos añadidos: {bumperSummary(options).toLowerCase()}.</p>
      <p className="font-mono text-meta text-fg-3">Plan aprobado: {snapshot.approval.approved_plan_hash.slice(0, 12)}</p>
      <div className="flex flex-wrap gap-x-4 gap-y-2 text-body-sm text-fg-1">
        <Link href={produceHref(video.jobId, PRODUCE_FORMAT.full)} className={LINK_CLASS}>Revisar ajustes y crear otra revisión</Link>
        {complete ? RENDER_DOCUMENTS.map((document) => <a key={document.id}
          href={pinRenderRevision(`/api/demos/${video.jobId}/renders/${video.variant ?? 'gameplay-pov-60'}/full-demo/${document.id}`, video.artifactRevision)}
          target="_blank" rel="noreferrer" className={LINK_CLASS}>
          {document.label}
        </a>) : null}
      </div>
      <p className="text-meta text-fg-3">Los documentos del render muestran sus intervalos efectivos y mediciones. Reintentar conserva el plan aprobado.</p>
    </div>
  </details>;
}
