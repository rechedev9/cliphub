'use client';

import { useCallback, useEffect, useRef, useState, type ReactElement, type ReactNode } from 'react';
import { Check, Copy, Download, ExternalLink, RefreshCw } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import {
  upcomingPublishSlots,
  type PublishAssistant,
  type PublishRecommendation,
} from '@/lib/api/publish-assistant';
import type { Video } from '@/lib/api/types';
import {
  PUBLISH_ASSISTANT_FAILED_COPY,
  PUBLISH_ASSISTANT_WAITING_COPY,
  copyPublishText,
  downloadPublishMP4,
  initialPublishDraft,
  openYouTubeStudio,
  publishAssistantAvailability,
  publishMP4FileName,
  publishTagsText,
  recommendedPublishDraft,
  type PublishDraft,
} from '@/lib/publish-actions';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';

const YOUTUBE_TITLE_MAX_LENGTH = 100;
const YOUTUBE_DESCRIPTION_MAX_LENGTH = 5000;
const YOUTUBE_UPLOAD_GUIDE_URL = 'https://support.google.com/youtube/answer/57407?hl=es';
const COPIED_FEEDBACK_MS = 2000;
const OTHER_SLOTS_MAX = 3;
const LINK_CLASS = 'text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1';

type AssistantView =
  | { status: 'loading' }
  | { status: 'failed' }
  | { status: 'ready'; assistant: PublishAssistant };

type Draft = { choice: string | undefined; title: string; description: string; tagsText: string };

type CopyTarget = 'title' | 'description' | 'tags' | 'all';

const FIELD_TARGETS = ['title', 'description', 'tags'] as const;

const COPIED_ANNOUNCEMENT: Record<CopyTarget, string> = {
  title: 'Título copiado',
  description: 'Descripción copiada',
  tags: 'Etiquetas copiadas',
  all: 'Título, descripción y etiquetas copiados',
};

const COPY_FAILED: Record<CopyTarget, string> = {
  title: 'No se pudo copiar el título.',
  description: 'No se pudo copiar la descripción.',
  tags: 'No se pudieron copiar las etiquetas.',
  all: 'No se pudo copiar el texto.',
};

type StepKey = 'download' | 'upload' | 'texts';

type Flash = { target: CopyTarget } | null;

type PublishStepsProps = { video: Video; file: Pick<Video, 'downloadUrl' | 'title'> };

const STEP_ORDER: readonly StepKey[] = ['download', 'upload', 'texts'];

function localDayLabel({ date, timeZone }: { date: string; timeZone: string }): string {
  const instant = new Date(`${date}T12:00:00Z`);
  if (Number.isNaN(instant.getTime())) return date;
  return new Intl.DateTimeFormat('es-ES', { timeZone, weekday: 'short', day: 'numeric', month: 'short' }).format(instant);
}

function editableDraft(draft: PublishDraft, choice: string | undefined): Draft {
  return { choice, title: draft.title, description: draft.description, tagsText: publishTagsText(draft.tags) };
}

type StepProps = {
  index: number;
  done: boolean;
  title: string;
  detail: ReactNode;
  action?: ReactNode;
  children?: ReactNode;
};

function Step({ index, done, title, detail, action, children }: StepProps): ReactElement {
  return (
    <li className="flex gap-3.5 border-t border-border-subtle p-4 first:border-t-0">
      <span
        aria-hidden
        className={cn(
          'flex size-7 shrink-0 items-center justify-center rounded-md border font-display text-body-sm font-semibold tabular-nums',
          'transition-[background-color,border-color,color] duration-(--dur-fast) ease-standard',
          done ? 'border-success bg-success/14 text-success' : 'border-border-strong text-fg-2',
        )}
      >
        {done ? <Check className="size-4" /> : index}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-4">
        <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2.5">
          <div className="flex min-w-0 flex-1 basis-64 flex-col gap-1">
            <h2 className="font-display text-body-lg font-semibold text-fg-1">
              {title}
              {done ? <span className="sr-only"> (hecho)</span> : null}
            </h2>
            <p className="text-body-sm text-fg-3">{detail}</p>
          </div>
          {action}
        </div>
        {children}
      </div>
    </li>
  );
}

function CopyButton({ copied, onCopy, children }: { copied: boolean; onCopy: () => void; children: ReactNode }): ReactElement {
  return (
    <Button type="button" variant="ghost" size="sm" onClick={onCopy}>
      {copied ? <Check className="size-3.5 text-success" aria-hidden /> : <Copy className="size-3.5" aria-hidden />}
      {copied ? 'Copiado' : children}
    </Button>
  );
}

type FieldRowProps = {
  id: string;
  label: string;
  counter?: string;
  copied: boolean;
  onCopy: () => void;
  children: ReactNode;
};

function FieldRow({ id, label, counter, copied, onCopy, children }: FieldRowProps): ReactElement {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex min-h-7 items-center justify-between gap-3">
        <Label htmlFor={id} className="text-body-sm font-medium text-fg-2">
          {label}
        </Label>
        <div className="flex items-center gap-2">
          {counter ? <span className="text-meta tabular-nums text-fg-3">{counter}</span> : null}
          <CopyButton copied={copied} onCopy={onCopy}>
            Copiar
          </CopyButton>
        </div>
      </div>
      {children}
    </div>
  );
}

type RecommendationPickerProps = {
  recommendations: PublishRecommendation[];
  choice: string | undefined;
  note: string | undefined;
  onChoose: (recommendation: PublishRecommendation) => void;
};

function RecommendationPicker({ recommendations, choice, note, onChoose }: RecommendationPickerProps): ReactElement {
  const templates = recommendations.some((recommendation) => recommendation.template);
  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="mb-1.5 text-body-sm font-medium text-fg-2">{templates ? 'Plantilla' : 'Título recomendado'}</legend>
      {note ? <p className="mb-1 text-meta text-fg-3">{note}</p> : null}
      <div className="divide-y divide-border-subtle overflow-hidden rounded-md border border-border">
        {recommendations.map((recommendation) => {
          const active = recommendation.title === choice;
          return (
            <label
              key={recommendation.title}
              className={cn(
                'flex cursor-pointer items-start gap-3 px-3 py-2.5',
                'transition-[background-color] duration-(--dur-fast) ease-standard',
                'has-[:focus-visible]:outline-2 has-[:focus-visible]:-outline-offset-2 has-[:focus-visible]:outline-ring',
                active ? 'bg-primary/8 hover:bg-primary/12' : 'hover:bg-surface-3',
              )}
            >
              <input
                type="radio"
                name="publish-recommendation"
                value={recommendation.title}
                checked={active}
                onChange={() => onChoose(recommendation)}
                className="mt-0.5 size-4 shrink-0 accent-primary focus-visible:outline-none"
              />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                {recommendation.template ? (
                  <span className="text-body-sm font-medium text-fg-1">{recommendation.template}</span>
                ) : null}
                <span className={cn('break-words', recommendation.template ? 'text-body-sm text-fg-2' : 'text-body-sm text-fg-1')}>
                  {recommendation.title}
                </span>
                {active ? <span className="text-meta text-fg-3">{recommendation.rationale}</span> : null}
              </span>
              {recommendation.template ? null : (
                <span className="shrink-0 text-meta tabular-nums text-fg-3">{Math.round(recommendation.score)}/100</span>
              )}
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}

function TextsSkeleton(): ReactElement {
  return (
    <div className="flex flex-col gap-4" role="status" aria-label="Preparando los textos">
      <Skeleton className="h-40 w-full" />
      <Skeleton className="h-16 w-full" />
      <Skeleton className="h-44 w-full" />
      <Skeleton className="h-20 w-full" />
    </div>
  );
}

function ScheduleBody({ assistant }: { assistant: PublishAssistant }): ReactElement {
  const { schedule } = assistant;
  const [best, ...rest] = upcomingPublishSlots(schedule);
  const others = rest.slice(0, OTHER_SLOTS_MAX);
  const sources = schedule.sources.filter((source) => source.url !== YOUTUBE_UPLOAD_GUIDE_URL);
  return (
    <div className="flex flex-col gap-2.5">
      {best ? (
        <div className="flex flex-wrap items-baseline gap-x-6 gap-y-2 rounded-md border border-border bg-surface-1 px-3.5 py-3">
          <p className="flex items-baseline gap-2">
            <span className="text-meta text-fg-3">Mejor hora</span>
            <strong className="font-display text-body-lg font-semibold text-fg-1 tabular-nums">
              {localDayLabel({ date: best.day.date, timeZone: schedule.timeZone })} · {best.slot.localTime}
            </strong>
          </p>
          {others.length > 0 ? (
            <p className="text-body-sm text-fg-2">
              <span className="text-meta text-fg-3">Otras: </span>
              {others.map(({ day, slot }) => `${localDayLabel({ date: day.date, timeZone: schedule.timeZone })} ${slot.localTime}`).join(' · ')}
            </p>
          ) : null}
        </div>
      ) : (
        <p className="text-body-sm text-fg-2">No hay franjas en los próximos días.</p>
      )}
      <p className="text-meta text-fg-3">{schedule.caveat}</p>
      {sources.length > 0 ? (
        <p className="flex flex-wrap gap-x-3 gap-y-1 text-meta">
          {sources.map((source) => (
            <a key={source.url} href={source.url} target="_blank" rel="noopener noreferrer" className={cn('inline-flex items-center gap-1', LINK_CLASS)}>
              {source.title} <ExternalLink className="size-3" aria-hidden />
            </a>
          ))}
        </p>
      ) : null}
    </div>
  );
}

function ReadySteps({ video, file }: PublishStepsProps): ReactElement {
  const [view, setView] = useState<AssistantView>({ status: 'loading' });
  const [draft, setDraft] = useState<Draft>({ choice: undefined, title: video.title, description: '', tagsText: '' });
  const [downloaded, setDownloaded] = useState<string | null>(null);
  const [opened, setOpened] = useState(false);
  const [copiedTargets, setCopiedTargets] = useState<ReadonlySet<CopyTarget>>(() => new Set());
  const [flash, setFlash] = useState<Flash>(null);
  const loadVersion = useRef(0);

  const load = useCallback(async (): Promise<void> => {
    const version = ++loadVersion.current;
    setView({ status: 'loading' });
    try {
      const assistant = await api.getPublishAssistant(video.id);
      if (version !== loadVersion.current) return;
      const initial = initialPublishDraft(assistant);
      const choice = assistant.recommendations.find((recommendation) => recommendation.title === initial.title)?.title;
      setView({ status: 'ready', assistant });
      setDraft(editableDraft(initial, choice));
      setCopiedTargets(new Set());
    } catch {
      if (version === loadVersion.current) setView({ status: 'failed' });
    }
  }, [video.id]);

  useEffect(() => {
    void load();
  }, [load, video.artifactRevision]);

  useEffect(() => {
    if (flash === null) return;
    const timer = setTimeout(() => setFlash(null), COPIED_FEEDBACK_MS);
    return () => clearTimeout(timer);
  }, [flash]);

  function download(): void {
    if (!file.downloadUrl) return;
    downloadPublishMP4({ url: file.downloadUrl, title: file.title });
    toast('Descargando MP4', { description: `${publishMP4FileName(file.title)} → Descargas` });
    setDownloaded(file.downloadUrl);
  }

  function openStudio(): void {
    openYouTubeStudio();
    setOpened(true);
  }

  async function copy(target: CopyTarget, value: string): Promise<void> {
    try {
      await copyPublishText(value);
    } catch {
      toast(COPY_FAILED[target]);
      return;
    }
    setFlash({ target });
    setCopiedTargets((current) => (current.has(target) ? current : new Set([...current, target])));
  }

  const textsDone = copiedTargets.has('all') || FIELD_TARGETS.every((field) => copiedTargets.has(field));
  const downloadDone = downloaded !== null && downloaded === file.downloadUrl;
  const stepDone: Record<StepKey, boolean> = { download: downloadDone, upload: opened, texts: textsDone };
  const next = STEP_ORDER.find((step) => !stepDone[step]);
  const assistant = view.status === 'ready' ? view.assistant : undefined;
  const recommendation = assistant?.recommendations.find((candidate) => candidate.title === draft.choice);
  const keywords = recommendation?.keywords ?? assistant?.keywords ?? [];
  const templates = assistant?.recommendations.some((candidate) => candidate.template) === true;
  const trends = assistant?.trends;

  let textsBody: ReactNode = <TextsSkeleton />;
  if (view.status === 'failed') {
    textsBody = (
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-warning/35 bg-warning/10 p-3" role="alert">
        <p className="text-body-sm text-warning">No se pudieron preparar los textos.</p>
        <Button type="button" variant="outline" size="sm" onClick={() => void load()}>
          <RefreshCw className="size-3.5" aria-hidden /> Reintentar
        </Button>
      </div>
    );
  } else if (assistant) {
    textsBody = (
      <div className="flex flex-col gap-4">
        {assistant.recommendations.length > 1 ? (
          <RecommendationPicker
            recommendations={assistant.recommendations}
            choice={draft.choice}
            note={templates ? trends?.reason : undefined}
            onChoose={(choice) => setDraft(editableDraft(recommendedPublishDraft(choice), choice.title))}
          />
        ) : null}
        <FieldRow
          id="publish-title"
          label="Título"
          counter={`${draft.title.length}/${YOUTUBE_TITLE_MAX_LENGTH}`}
          copied={flash?.target === 'title'}
          onCopy={() => void copy('title', draft.title)}
        >
          <Input
            id="publish-title"
            value={draft.title}
            onChange={(event) => {
              const value = event.currentTarget.value;
              setDraft((current) => ({ ...current, title: value }));
            }}
            maxLength={YOUTUBE_TITLE_MAX_LENGTH}
          />
        </FieldRow>
        <FieldRow id="publish-description" label="Descripción" copied={flash?.target === 'description'} onCopy={() => void copy('description', draft.description)}>
          <Textarea
            id="publish-description"
            value={draft.description}
            onChange={(event) => {
              const value = event.currentTarget.value;
              setDraft((current) => ({ ...current, description: value }));
            }}
            maxLength={YOUTUBE_DESCRIPTION_MAX_LENGTH}
            className="min-h-40 max-h-[32rem] [field-sizing:content]"
          />
        </FieldRow>
        <FieldRow id="publish-tags" label="Etiquetas" copied={flash?.target === 'tags'} onCopy={() => void copy('tags', draft.tagsText)}>
          <Textarea
            id="publish-tags"
            value={draft.tagsText}
            onChange={(event) => {
              const value = event.currentTarget.value;
              setDraft((current) => ({ ...current, tagsText: value }));
            }}
            className="min-h-0 [field-sizing:content]"
            aria-describedby={keywords.length > 0 ? 'publish-keywords' : undefined}
          />
          {keywords.length > 0 ? (
            <p id="publish-keywords" className="text-meta text-fg-3">
              Palabras clave: <span className="text-fg-2">{keywords.join(' · ')}</span>
            </p>
          ) : null}
        </FieldRow>
        {trends?.available && trends.terms.length > 0 ? (
          <p className="text-meta text-fg-3">
            Tendencias recientes: <span className="text-fg-2">{trends.terms.join(' · ')}</span>
            {trends.sources.map((source) => (
              <a key={source.url} href={source.url} target="_blank" rel="noopener noreferrer" className={cn('ml-3 inline-flex items-center gap-1', LINK_CLASS)}>
                {source.title} <ExternalLink className="size-3" aria-hidden />
              </a>
            ))}
          </p>
        ) : null}
        {!templates && !trends?.available && trends?.reason ? <p className="text-meta text-fg-3">{trends.reason}</p> : null}
      </div>
    );
  }

  let scheduleBody: ReactNode = null;
  if (assistant) scheduleBody = <ScheduleBody assistant={assistant} />;
  else if (view.status === 'loading') scheduleBody = <Skeleton className="h-12 w-full" />;

  return (
    <>
    <ol className="studio-panel" aria-label="Pasos para publicar en YouTube">
      <Step
        index={1}
        done={downloadDone}
        title="Descarga el MP4"
        detail={<span className="font-mono text-meta [overflow-wrap:anywhere]">{publishMP4FileName(file.title)}</span>}
        action={
          <Button type="button" variant={next === 'download' ? 'hero' : 'outline'} size="sm" disabled={!file.downloadUrl} onClick={download}>
            <Download className="size-4" aria-hidden /> Descargar MP4
          </Button>
        }
      />
      <Step
        index={2}
        done={opened}
        title="Súbelo en YouTube Studio"
        detail={
          <>
            Crear → Subir vídeos y elige el MP4. ClipHub no se conecta a tu canal.{' '}
            <a href={YOUTUBE_UPLOAD_GUIDE_URL} target="_blank" rel="noopener noreferrer" className={LINK_CLASS}>
              Guía de YouTube
            </a>
          </>
        }
        action={
          <Button type="button" variant={next === 'upload' ? 'hero' : 'outline'} size="sm" onClick={openStudio}>
            Abrir YouTube Studio <ExternalLink className="size-3.5" aria-hidden />
          </Button>
        }
      />
      <Step
        index={3}
        done={textsDone}
        title="Copia título, descripción y etiquetas"
        detail="Pégalos en Detalles. Las etiquetas están en Mostrar más."
        action={
          assistant ? (
            <CopyButton copied={flash?.target === 'all'} onCopy={() => void copy('all', [draft.title, '', draft.description, '', draft.tagsText].join('\n'))}>
              Copiar todo
            </CopyButton>
          ) : null
        }
      >
        {textsBody}
      </Step>
      <Step index={4} done={false} title="Prográmalo" detail="En Visibilidad → Programar. Horas de Madrid.">
        {scheduleBody}
      </Step>
    </ol>
    <p className="sr-only" aria-live="polite">
      {flash === null ? '' : COPIED_ANNOUNCEMENT[flash.target]}
    </p>
    </>
  );
}

export function PublishSteps({ video, file }: PublishStepsProps): ReactElement {
  const availability = publishAssistantAvailability(video.status);
  if (availability === 'waiting') {
    return (
      <p className="studio-panel p-4 text-body-sm text-fg-2" role="status">
        {PUBLISH_ASSISTANT_WAITING_COPY}
      </p>
    );
  }
  if (availability === 'failed') {
    return (
      <p className="rounded-lg border border-warning/35 bg-warning/10 p-4 text-body-sm text-warning" role="alert">
        {PUBLISH_ASSISTANT_FAILED_COPY}
      </p>
    );
  }
  return <ReadySteps video={video} file={file} />;
}
