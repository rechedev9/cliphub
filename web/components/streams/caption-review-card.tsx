'use client';

import { useRef, type ReactNode } from 'react';
import { CircleCheck, Plus, Trash2 } from 'lucide-react';
import { CAPTION_CLIP_STATUS, type CaptionCandidateClip, type StreamCaptionWord, type StreamClipRange } from '@/lib/api/streams';
import { captionWordsIssue, clipHasAudibleSource } from '@/lib/caption-review';
import { formatStreamTimestamp, groupCaptionWords } from '@/lib/streams/plan';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

const STATUS_TEXT_CLASS = {
  neutral: 'text-fg-3',
  primary: 'text-primary',
  success: 'text-success',
  warning: 'text-warning',
  danger: 'text-destructive',
  stream: 'text-stream-text',
} as const satisfies Record<StatusTagTone, string>;

/**
 * Per-clip caption review.
 *
 * The pipeline's contract is that machine words are EVIDENCE, never publishable
 * text, so this card has to keep the imported candidates visually separate from
 * a human decision: an unreviewed clip is amber everywhere (tag, rail, footer)
 * and says the words are unverified; approving or confirming no-speech turns
 * the same surfaces mint. Editing a reviewed word invalidates the review
 * upstream, and the card follows it straight back to amber.
 *
 * The word list itself is grouped into spoken lines instead of one flat row per
 * word — a 30 second clip is 60-90 words, and a wall of anonymous triples is
 * unreadable. Each line can be opened to correct its individual words; the flat
 * indices are preserved, so every edit still writes the same word in the same
 * order it always did.
 */
export function StreamCaptionReviewCard({
  videoSrc,
  clip,
  clipNumber,
  candidate,
  words,
  disabled,
  reviewing,
  onWordsChange,
  onApprove,
  onNoSpeech,
}: {
  videoSrc: string;
  clip: StreamClipRange;
  clipNumber: number;
  candidate?: CaptionCandidateClip;
  words: StreamCaptionWord[];
  disabled: boolean;
  reviewing: boolean;
  onWordsChange: (words: StreamCaptionWord[]) => void;
  onApprove: () => void;
  onNoSpeech: () => void;
}): ReactNode {
  const audioRef = useRef<HTMLAudioElement>(null);
  const duration = Math.max(0, clip.end_seconds - clip.start_seconds);
  const audible = clipHasAudibleSource(clip);
  const reviewed = clip.caption_reviewed === true;
  const reviewedNoSpeech = reviewed && (clip.caption_words?.length ?? 0) === 0;
  const issue = captionWordsIssue(words, duration);
  const segments = groupCaptionWords(words);

  const updateWord = (index: number, patch: Partial<StreamCaptionWord>) =>
    onWordsChange(words.map((word, wordIndex) => (wordIndex === index ? { ...word, ...patch } : word)));
  const removeWord = (index: number) => onWordsChange(words.filter((_word, wordIndex) => wordIndex !== index));
  const addWord = () => {
    const start = words[words.length - 1]?.end_seconds ?? 0;
    const end = Math.min(duration, start + 0.5);
    if (end <= start) return;
    onWordsChange([...words, { word: '', start_seconds: start, end_seconds: end }]);
  };

  let status: string;
  let tone: StatusTagTone = 'warning';
  let tag = 'REQUIERE REVISIÓN';
  if (!audible) {
    status = 'Audio silenciado: este clip no necesita subtítulos.';
    tone = 'neutral';
    tag = 'SIN AUDIO';
  } else if (reviewedNoSpeech) {
    status = 'Revisado: confirmado sin voz.';
    tone = 'success';
    tag = 'SIN VOZ';
  } else if (reviewed) {
    status = `Revisado: ${clip.caption_words?.length ?? 0} palabras aprobadas.`;
    tone = 'success';
    tag = 'REVISADO';
  } else if (candidate?.status === CAPTION_CLIP_STATUS.failed) {
    status = candidate.error || 'El proveedor de transcripción falló. Escucha el tramo y vuelve a generar o corrígelo a mano.';
    tone = 'danger';
    tag = 'FALLÓ';
  } else if (candidate?.status === CAPTION_CLIP_STATUS.noSpeech) {
    status = 'El proveedor analizó el audio pero no detectó voz. Escucha el tramo; si sí hay voz, vuelve a generar.';
    tag = 'SIN VOZ DETECTADA';
  } else if (candidate) {
    status = `${words.length} palabras candidatas pendientes de revisión.`;
    tag = 'CANDIDATOS';
  } else {
    status = 'Este clip todavía no tiene candidatos actuales.';
    tone = 'neutral';
    tag = 'SIN CANDIDATOS';
  }

  const verified = reviewed && audible;

  return (
    <section
      aria-labelledby={`${clip.id}-caption-title`}
      className={cn(
        '@container/caption flex flex-col gap-3 border bg-surface-2 p-4 shadow-[var(--elev-0)]',
        verified ? 'border-success/45' : 'border-border',
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2.5">
            <h3
              id={`${clip.id}-caption-title`}
              className="font-display text-body-lg font-bold text-fg-1"
            >
              Clip {clipNumber}
              {clip.title?.trim() ? ` · ${clip.title.trim()}` : ''}
            </h3>
            <StatusTag tone={tone}>{tag}</StatusTag>
          </div>
          <p className={cn('mt-1.5 text-body-sm', STATUS_TEXT_CLASS[tone])}>{status}</p>
        </div>
        {candidate?.provider ? (
          <span className="shrink-0 font-mono text-meta uppercase tracking-wider text-fg-3">
            {candidate.provider}
            {candidate.stt_model ? ` · ${candidate.stt_model}` : ''}
          </span>
        ) : null}
      </div>

      {audible && candidate ? (
        <>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`${clip.id}-audio-review`} className="font-mono text-meta uppercase tracking-wider text-fg-3">
              Escuchar tramo original ({formatStreamTimestamp(clip.start_seconds)}–{formatStreamTimestamp(clip.end_seconds)})
            </Label>
            <audio
              id={`${clip.id}-audio-review`}
              ref={audioRef}
              src={videoSrc}
              controls
              preload="metadata"
              onPlay={(event) => {
                if (
                  event.currentTarget.currentTime < clip.start_seconds ||
                  event.currentTarget.currentTime >= clip.end_seconds
                ) {
                  event.currentTarget.currentTime = clip.start_seconds;
                }
              }}
              onTimeUpdate={(event) => {
                if (event.currentTarget.currentTime >= clip.end_seconds) event.currentTarget.pause();
              }}
              className="h-10 w-full"
            />
          </div>

          <div
            className={cn(
              'flex flex-col border-l-2 bg-surface-1',
              verified ? 'border-l-success/60' : 'border-l-warning/60',
            )}
          >
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border-subtle px-3 py-2">
              <span className="font-mono text-meta uppercase tracking-wider text-fg-3">
                {verified ? 'Texto verificado' : 'Texto importado · sin verificar'}
              </span>
              <span className="font-mono text-meta tabular-nums text-fg-3">
                {words.length} {words.length === 1 ? 'PALABRA' : 'PALABRAS'} · {segments.length}{' '}
                {segments.length === 1 ? 'LÍNEA' : 'LÍNEAS'}
              </span>
            </div>

            {words.length === 0 ? (
              <p className="p-3 text-body-sm text-fg-3">
                No hay palabras. Puedes añadirlas manualmente o confirmar que el clip no contiene voz.
              </p>
            ) : (
              <ul className="flex max-h-[28rem] flex-col divide-y divide-border-subtle overflow-auto">
                {segments.map((segment) => (
                  <li key={segment.entries[0].index}>
                    <details className="group/segment">
                      <summary className="flex min-h-11 cursor-pointer list-none items-baseline gap-3 px-3 py-2 transition-colors duration-(--dur-fast) ease-standard hover:bg-surface-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring">
                        <span className="shrink-0 font-mono text-meta tabular-nums text-stream-text">
                          {formatStreamTimestamp(segment.startSeconds)}
                        </span>
                        <span className="min-w-0 flex-1 text-body text-fg-1">
                          {segment.text === '' ? (
                            <span className="text-fg-3">(sin texto)</span>
                          ) : (
                            segment.text
                          )}
                        </span>
                        <span className="shrink-0 font-mono text-meta tabular-nums text-fg-3 group-open/segment:text-primary">
                          {segment.entries.length}
                        </span>
                      </summary>

                      <div className="flex flex-col gap-2 border-t border-border-subtle bg-surface-2 p-3">
                        <div className="grid grid-cols-[minmax(0,1fr)_5rem_5rem_2.5rem] gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
                          <span>Palabra</span>
                          <span>Inicio</span>
                          <span>Fin</span>
                          <span className="sr-only">Acciones</span>
                        </div>
                        {segment.entries.map(({ index, word }) => (
                          <div key={index} className="grid grid-cols-[minmax(0,1fr)_5rem_5rem_2.5rem] items-center gap-2">
                            <Input
                              id={`${clip.id}-caption-${index}-word`}
                              aria-label={`Palabra ${index + 1}`}
                              value={word.word}
                              maxLength={80}
                              disabled={disabled}
                              onChange={(event) => updateWord(index, { word: event.target.value })}
                              className="h-10"
                            />
                            <Input
                              id={`${clip.id}-caption-${index}-start`}
                              aria-label={`Inicio de la palabra ${index + 1}`}
                              type="number"
                              min={0}
                              max={duration}
                              step="0.05"
                              value={word.start_seconds}
                              disabled={disabled}
                              onChange={(event) => updateWord(index, { start_seconds: Number(event.target.value) })}
                              className="h-10 px-2 tabular-nums"
                            />
                            <Input
                              id={`${clip.id}-caption-${index}-end`}
                              aria-label={`Fin de la palabra ${index + 1}`}
                              type="number"
                              min={0}
                              max={duration}
                              step="0.05"
                              value={word.end_seconds}
                              disabled={disabled}
                              onChange={(event) => updateWord(index, { end_seconds: Number(event.target.value) })}
                              className="h-10 px-2 tabular-nums"
                            />
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-sm"
                              disabled={disabled}
                              onClick={() => removeWord(index)}
                              aria-label={`Eliminar palabra ${index + 1}`}
                            >
                              <Trash2 className="size-4" aria-hidden />
                            </Button>
                          </div>
                        ))}
                      </div>
                    </details>
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" variant="outline" size="sm" disabled={disabled} onClick={addWord}>
              <Plus className="size-4" aria-hidden />
              AÑADIR PALABRA
            </Button>
            <Button
              type="button"
              size="sm"
              disabled={disabled || issue !== null}
              loading={reviewing}
              onClick={onApprove}
            >
              {reviewing ? null : <CircleCheck className="size-4" aria-hidden />}
              {reviewed ? 'GUARDAR REVISIÓN' : 'APROBAR TEXTO'}
            </Button>
            <Button type="button" variant="ghost" size="sm" disabled={disabled} onClick={onNoSpeech}>
              CONFIRMAR SIN VOZ
            </Button>
          </div>
          {issue && words.length > 0 ? (
            <p role="alert" className="text-body-sm text-destructive">
              {issue}
            </p>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
