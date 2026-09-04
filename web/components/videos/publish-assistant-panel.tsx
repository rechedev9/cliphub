'use client';

import { useCallback, useEffect, useState, type ReactElement, type ReactNode } from 'react';
import { Check, Clock3, Copy, ExternalLink, RefreshCw, Sparkles, Tags, Youtube } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import {
  upcomingPublishSlots,
  type PublishAssistant,
  type PublishRecommendation,
} from '@/lib/api/publish-assistant';
import type { Video } from '@/lib/api/types';
import {
  copyPublishText,
  initialPublishDraft,
  openYouTubeStudio,
  publishTagsText,
  recommendedPublishDraft,
} from '@/lib/publish-actions';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

const YOUTUBE_TITLE_MAX_LENGTH = 100;
const YOUTUBE_DESCRIPTION_MAX_LENGTH = 5000;
const YOUTUBE_UPLOAD_GUIDE_URL = 'https://support.google.com/youtube/answer/57407?hl=es';
/** How long "✓ Copiado" replaces the copy-all label. */
const COPIED_FEEDBACK_MS = 2000;

export type PublishAssistantPanelProps = {
  video: Video;
  /** Slot rendered between the texts and the note, e.g. a dialog's download button. */
  actions?: ReactNode;
};

function localDayLabel(date: string, timeZone: string): string {
  const instant = new Date(`${date}T12:00:00Z`);
  if (Number.isNaN(instant.getTime())) return date;
  return new Intl.DateTimeFormat('es-ES', { timeZone, weekday: 'short', day: 'numeric', month: 'short' }).format(instant);
}

function confidenceLabel(confidence: number): string {
  return new Intl.NumberFormat('es-ES', { style: 'percent', maximumFractionDigits: 0 }).format(confidence);
}

function SchedulePanel({ assistant }: { assistant: PublishAssistant }): ReactElement {
  const upcoming = upcomingPublishSlots(assistant.schedule);
  const sourceLinks = [...assistant.schedule.sources, ...assistant.trends.sources].filter(
    (source, index, links) => links.findIndex((candidate) => candidate.url === source.url) === index,
  );
  let trendContent: ReactElement | null = null;
  if (assistant.trends.available && assistant.trends.terms.length > 0) {
    trendContent = (
      <div>
        <p className="font-mono text-meta uppercase tracking-wider text-fg-3">Tendencias públicas recientes</p>
        <div className="mt-1.5 flex flex-wrap gap-1.5">
          {assistant.trends.terms.map((term) => (
            <span key={term} className="border border-border bg-surface-1 px-2 py-1 text-meta text-fg-1">
              {term}
            </span>
          ))}
        </div>
      </div>
    );
  } else if (assistant.trends.reason) {
    trendContent = <p className="text-meta leading-relaxed text-fg-3">{assistant.trends.reason}</p>;
  }
  return (
    <section className="flex flex-col gap-3 border-t border-border-subtle pt-3" aria-labelledby="publish-schedule-title">
      <h3 id="publish-schedule-title" className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
        <Clock3 className="size-3.5 text-primary" aria-hidden /> Más franjas · {assistant.schedule.timeZone}
      </h3>
      {upcoming.length > 0 ? (
        <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-4">
          {upcoming.slice(0, 4).map(({ day, slot }) => (
            <div key={day.date} className="border border-border bg-surface-1 px-2.5 py-2 text-meta">
              <span className="block text-fg-3">{localDayLabel(day.date, assistant.schedule.timeZone)}</span>
              <strong className="font-mono text-fg-1">{slot.localTime}</strong>
              <span className="block text-fg-3">confianza {confidenceLabel(slot.confidence)}</span>
            </div>
          ))}
        </div>
      ) : null}
      {trendContent}
      {sourceLinks.length > 0 ? (
        <div className="flex flex-wrap gap-x-3 gap-y-1">
          {sourceLinks.map((source) => (
            <a
              key={source.url}
              href={source.url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-meta text-primary underline underline-offset-4"
            >
              {source.title} <ExternalLink className="size-3" aria-hidden />
            </a>
          ))}
        </div>
      ) : null}
      <p className="text-meta leading-relaxed text-fg-3">{assistant.schedule.caveat}</p>
    </section>
  );
}

/**
 * The manual YouTube publication assistant: factual reel-derived texts, the
 * Madrid schedule, copy-all and the only outbound link (YouTube Studio).
 * Rendered as the /publicar aside and inside `PublishAssistantDialog`.
 */
export function PublishAssistantPanel({ video, actions }: PublishAssistantPanelProps): ReactElement {
  const [assistant, setAssistant] = useState<PublishAssistant>();
  const [selectedRecommendation, setSelectedRecommendation] = useState<PublishRecommendation>();
  const [title, setTitle] = useState(video.title);
  const [description, setDescription] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [copied, setCopied] = useState(false);

  const load = useCallback(async (): Promise<void> => {
    setLoading(true);
    setError(undefined);
    try {
      const next = await api.getPublishAssistant(video.id);
      const draft = initialPublishDraft(next);
      setAssistant(next);
      setSelectedRecommendation(undefined);
      setTitle(draft.title);
      setDescription(draft.description);
      setTags(draft.tags);
    } catch {
      setError('No se pudo preparar la publicación. El MP4 sigue disponible para descargar.');
    } finally {
      setLoading(false);
    }
  }, [video.id]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), COPIED_FEEDBACK_MS);
    return () => clearTimeout(timer);
  }, [copied]);

  function applyRecommendation(recommendation: PublishRecommendation): void {
    const draft = recommendedPublishDraft(recommendation);
    setSelectedRecommendation(recommendation);
    setTitle(draft.title);
    setDescription(draft.description);
    setTags(draft.tags);
  }

  async function copy(value: string, label: string): Promise<void> {
    try {
      await copyPublishText(value);
      toast(`${label} copiado al portapapeles.`);
    } catch {
      toast(`No se pudo copiar ${label.toLowerCase()}.`);
    }
  }

  async function copyAll(): Promise<void> {
    try {
      await copyPublishText([title, '', description, '', tagsText].join('\n'));
      setCopied(true);
      toast('Copiado al portapapeles', { description: 'Título, descripción y etiquetas' });
    } catch {
      toast('No se pudo copiar el texto.');
    }
  }

  const keywords = selectedRecommendation?.keywords ?? assistant?.keywords ?? [];
  const tagsText = publishTagsText(tags);
  const best = assistant ? upcomingPublishSlots(assistant.schedule)[0] : undefined;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between font-mono text-meta uppercase tracking-ultra">
        <span className="text-primary">Publicar en YouTube</span>
        <span className="text-fg-3">Asistente</span>
      </div>

      {loading ? (
        <p className="flex min-h-24 items-center justify-center gap-2 text-body-sm text-fg-3" role="status">
          <span className="studio-spinner text-primary" aria-hidden /> Preparando metadatos y horario…
        </p>
      ) : null}

      {!loading && error ? (
        <div className="flex flex-col gap-3 border border-warning/35 bg-warning/10 p-3.5" role="alert">
          <p className="text-body-sm text-warning">{error}</p>
          <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
            <RefreshCw className="size-3.5" /> Reintentar
          </Button>
        </div>
      ) : null}

      {!loading && assistant ? (
        <>
          <section className="flex flex-col gap-2" aria-labelledby="publish-title-recommendations">
            <h3 id="publish-title-recommendations" className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
              <Sparkles className="size-3.5 text-primary" aria-hidden /> Títulos recomendados
            </h3>
            <div className="grid gap-1.5">
              {assistant.recommendations.map((recommendation) => (
                <Button
                  key={recommendation.title}
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-auto justify-between gap-3 whitespace-normal px-3 py-2 text-left font-normal"
                  onClick={() => applyRecommendation(recommendation)}
                  aria-label={`Usar título recomendado: ${recommendation.title}`}
                  aria-pressed={selectedRecommendation?.title === recommendation.title}
                >
                  <span>{recommendation.title}</span>
                  <span className="shrink-0 font-mono text-meta tabular-nums text-fg-3">{Math.round(recommendation.score)}/100</span>
                </Button>
              ))}
            </div>
            {selectedRecommendation ? (
              <p className="border border-border bg-surface-1 p-2.5 text-meta text-fg-3" role="status">
                {selectedRecommendation.rationale}
              </p>
            ) : null}
          </section>

          <div className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="publish-title" className="font-mono text-meta uppercase tracking-wider text-fg-3">
                Título
              </Label>
              <Button type="button" variant="ghost" size="xs" onClick={() => void copy(title, 'Título')}>
                <Copy className="size-3" /> Copiar
              </Button>
            </div>
            <Input
              id="publish-title"
              value={title}
              onChange={(event) => setTitle(event.currentTarget.value)}
              maxLength={YOUTUBE_TITLE_MAX_LENGTH}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="publish-description" className="font-mono text-meta uppercase tracking-wider text-fg-3">
                Descripción
              </Label>
              <Button type="button" variant="ghost" size="xs" onClick={() => void copy(description, 'Descripción')}>
                <Copy className="size-3" /> Copiar
              </Button>
            </div>
            <textarea
              id="publish-description"
              value={description}
              onChange={(event) => setDescription(event.currentTarget.value)}
              maxLength={YOUTUBE_DESCRIPTION_MAX_LENGTH}
              rows={5}
              className="w-full resize-y rounded-md border border-border-strong bg-surface-3 px-3.5 py-3 text-body-sm text-fg-1 shadow-[var(--elev-0)] outline-none transition-[border-color,box-shadow,background-color] duration-(--dur-instant) ease-standard placeholder:text-fg-3 focus-visible:border-primary focus-visible:bg-surface-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            />
          </div>

          <section className="flex flex-col gap-1.5" aria-labelledby="publish-tags-title">
            <div className="flex items-center justify-between gap-3">
              <h3 id="publish-tags-title" className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
                <Tags className="size-3.5 text-primary" aria-hidden /> Etiquetas
              </h3>
              <Button type="button" variant="ghost" size="xs" onClick={() => void copy(tagsText, 'Etiquetas')}>
                <Copy className="size-3" /> Copiar
              </Button>
            </div>
            <p className="border border-border-strong bg-surface-1 px-3 py-2.5 text-body-sm text-fg-1">{tagsText}</p>
            {keywords.length > 0 ? (
              <p className="text-meta text-fg-3">
                <strong className="text-fg-2">Palabras clave:</strong> {keywords.join(' · ')}
              </p>
            ) : null}
          </section>

          <div className="flex items-center justify-between gap-3 border border-border-accent bg-primary/8 px-3 py-2.5">
            <div className="flex min-w-0 flex-col gap-0.5">
              <span className="font-mono text-meta uppercase tracking-wider text-fg-3">Mejor hora (Madrid)</span>
              <span className="font-display text-body-sm font-semibold uppercase text-fg-1">
                {best ? `${localDayLabel(best.day.date, assistant.schedule.timeZone)} · ${best.slot.localTime}` : 'Sin franjas futuras'}
              </span>
            </div>
            <Button type="button" variant="outline-primary" size="xs" onClick={() => void copyAll()} aria-live="polite">
              {copied ? (
                <>
                  <Check className="size-3" /> Copiado
                </>
              ) : (
                'Copiar todo'
              )}
            </Button>
          </div>

          <SchedulePanel assistant={assistant} />
        </>
      ) : null}

      {actions}

      <Button type="button" variant="hero" onClick={openYouTubeStudio} className="neon-notch w-full focus-visible:-outline-offset-4">
        <Youtube className="size-4" /> Abrir YouTube Studio →
      </Button>
      <p className="text-center font-mono text-meta uppercase tracking-wider text-fg-3">Sin cuenta conectada · subes tú el MP4</p>
      <p className="text-meta leading-relaxed text-fg-3">
        YouTube Studio te guía por <strong className="text-fg-2">CREAR → Subir vídeos</strong>, audiencia, visibilidad y programación.{' '}
        <a href={YOUTUBE_UPLOAD_GUIDE_URL} target="_blank" rel="noopener noreferrer" className="text-primary underline underline-offset-4">
          Guía oficial de YouTube
        </a>
        .
      </p>
    </div>
  );
}
