'use client';

import { useEffect, useState, type ChangeEvent, type FormEvent, type ReactNode } from 'react';
import {
  CheckCircle2,
  FileAudio,
  FileText,
  ImagePlus,
  Link2,
  LoaderCircle,
  type LucideIcon,
  Mic2,
  Save,
  ShieldCheck,
  Trash2,
  Upload,
} from 'lucide-react';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { IconTile } from '@/components/studio/icon-tile';
import { StatusTag } from '@/components/studio/status-tag';
import { StudioDataRow } from '@/components/studio/data-row';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import {
  deleteNewsVoiceProfile,
  loadNewsDraft,
  loadNewsVoiceProfile,
  saveNewsDraft,
  saveNewsVoiceProfile,
  type NewsShortDraft,
  type NewsVoiceProfile,
} from '@/lib/api/news';

const DEFAULT_SOURCE_URL = 'https://x.com/CounterStrike/status/2077901099961647290';
const DEFAULT_CHANNEL = 'RaizerinhoCS2';
const DEFAULT_TITLE = 'CS2 añade más skins… ¿y el anticheat?';
const DEFAULT_HOOK = '¿OTRA VEZ SKINS? ¿Y EL ANTICHEAT?';
const DEFAULT_SCRIPT = `Valve acaba de anunciar otra actualización para Counter-Strike 2.

¿Un nuevo anticheat? No. Más skins y más pegatinas.

Buscan una colección de armas llamada Fairy Tales y dos colecciones de stickers: Cryptids y Pop Art.

Y en todo el anuncio: cero menciones a VAC, VACnet o VAC Live.

La reacción fue inmediata. Para muchos jugadores, un anticheat nuevo sí que sería un cuento de hadas.

Esto no demuestra que Valve no esté trabajando en él, pero otra vez no hay respuestas públicas.

¿Qué necesita Counter-Strike 2 ahora mismo: más skins o un anticheat que funcione?`;

/** Server-side cap on the stored script; the editor counts against the same number. */
const SCRIPT_MAX_LENGTH = 20000;

/** The DOM ids the label-activated file pickers bind to. */
const IMAGES_INPUT_ID = 'news-images';
const VOICE_INPUT_ID = 'voice-reference';

/** Words in the script, for the editor's counter. Empty script counts as zero. */
function wordCount(script: string): number {
  const trimmed = script.trim();
  return trimmed === '' ? 0 : trimmed.split(/\s+/).length;
}

export function NewsShortWorkspace(): ReactNode {
  const [profile, setProfile] = useState<NewsVoiceProfile | null>();
  const [voiceFile, setVoiceFile] = useState<File | null>(null);
  const [voiceBusy, setVoiceBusy] = useState(false);
  const [voiceError, setVoiceError] = useState('');
  const [sourceUrl, setSourceUrl] = useState(DEFAULT_SOURCE_URL);
  const [channel, setChannel] = useState(DEFAULT_CHANNEL);
  const [title, setTitle] = useState(DEFAULT_TITLE);
  const [hook, setHook] = useState(DEFAULT_HOOK);
  const [script, setScript] = useState(DEFAULT_SCRIPT);
  const [images, setImages] = useState<File[]>([]);
  const [draftSavedAt, setDraftSavedAt] = useState('');
  const [draftError, setDraftError] = useState('');

  useEffect(() => {
    const draft = loadNewsDraft(window.localStorage);
    if (draft !== null) {
      setSourceUrl(draft.sourceUrl);
      setChannel(draft.channel);
      setTitle(draft.title);
      setHook(draft.hook);
      setScript(draft.script);
      setDraftSavedAt(draft.updatedAt);
    }
    void loadNewsVoiceProfile()
      .then((loaded) => {
        setProfile(loaded);
        if (draft === null && loaded !== null) setChannel(loaded.channel);
      })
      .catch((error: unknown) => {
        setProfile(null);
        setVoiceError(error instanceof Error ? error.message : 'No se pudo cargar la voz local.');
      });
  }, []);

  async function uploadVoice(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    if (voiceFile === null) return;
    setVoiceBusy(true);
    setVoiceError('');
    try {
      const saved = await saveNewsVoiceProfile(voiceFile, channel.trim() || DEFAULT_CHANNEL);
      setProfile(saved);
      setVoiceFile(null);
    } catch (error: unknown) {
      setVoiceError(error instanceof Error ? error.message : 'No se pudo guardar la voz.');
    } finally {
      setVoiceBusy(false);
    }
  }

  async function removeVoice(): Promise<void> {
    if (!window.confirm('¿Eliminar la referencia de voz guardada en este equipo?')) return;
    setVoiceBusy(true);
    setVoiceError('');
    try {
      await deleteNewsVoiceProfile();
      setProfile(null);
      setVoiceFile(null);
    } catch (error: unknown) {
      setVoiceError(error instanceof Error ? error.message : 'No se pudo eliminar la voz.');
    } finally {
      setVoiceBusy(false);
    }
  }

  function selectVoice(event: ChangeEvent<HTMLInputElement>): void {
    setVoiceFile(event.target.files?.[0] ?? null);
  }

  function selectImages(event: ChangeEvent<HTMLInputElement>): void {
    setImages(Array.from(event.target.files ?? []));
  }

  function saveDraft(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    setDraftError('');
    const updatedAt = new Date().toISOString();
    const draft: NewsShortDraft = {
      sourceUrl: sourceUrl.trim(),
      channel: channel.trim() || DEFAULT_CHANNEL,
      title: title.trim(),
      hook: hook.trim(),
      script: script.trim(),
      updatedAt,
    };
    if (!saveNewsDraft(window.localStorage, draft)) {
      setDraftError('No se pudo guardar el borrador en este navegador. Comprueba el almacenamiento local.');
      return;
    }
    setDraftSavedAt(updatedAt);
  }

  return (
    <div className="grid items-start gap-6 @[62rem]/content:grid-cols-[minmax(0,1.6fr)_minmax(19rem,0.85fr)]">
      <NewsPanel
        className="@container/form"
        icon={FileText}
        eyebrow="GUION"
        title="Nuevo short de noticias"
        description="Guarda la fuente, el enfoque y el guion. El caso de CS2 ya está precargado como plantilla editable."
      >
        <form className="flex flex-col gap-5" onSubmit={saveDraft}>
          <Field
            label={
              <>
                <Link2 aria-hidden className="size-4" />
                Fuente
              </>
            }
            required
            hint="El enlace que el short cita en pantalla."
          >
            {(control) => (
              <Input
                {...control}
                type="url"
                required
                value={sourceUrl}
                onChange={(event) => setSourceUrl(event.target.value)}
              />
            )}
          </Field>

          <div className="grid gap-5 @[28rem]/form:grid-cols-2">
            <Field label="Canal" required>
              {(control) => (
                <Input {...control} required value={channel} onChange={(event) => setChannel(event.target.value)} />
              )}
            </Field>
            <Field label="Título de YouTube" required>
              {(control) => (
                <Input {...control} required value={title} onChange={(event) => setTitle(event.target.value)} />
              )}
            </Field>
          </div>

          <Field label="Gancho en pantalla" required hint="Los primeros segundos: lo que aparece sobreimpreso.">
            {(control) => (
              <Input {...control} required value={hook} onChange={(event) => setHook(event.target.value)} />
            )}
          </Field>

          <Field label="Guion">
            {(control) => (
              // An editor well, not an input: recessed surface, its own status
              // rail with live counts, and one focus indicator for the whole
              // frame so the toolbar reads as part of the control.
              <div
                className={cn(
                  'flex flex-col overflow-hidden rounded-md border border-border-strong bg-surface-0 shadow-[var(--elev-0)]',
                  'transition-[border-color] duration-(--dur-instant) ease-standard',
                  'has-[:focus-visible]:border-primary has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
                )}
              >
                <div className="flex items-center justify-between gap-3 border-b border-border-subtle bg-surface-2 px-3 py-2 font-mono text-meta uppercase tracking-wider text-fg-3">
                  <span>Narración</span>
                  <span className="tabular-nums">
                    {wordCount(script)} palabras · {script.length}/{SCRIPT_MAX_LENGTH}
                  </span>
                </div>
                <textarea
                  {...control}
                  required
                  rows={15}
                  maxLength={SCRIPT_MAX_LENGTH}
                  spellCheck
                  value={script}
                  onChange={(event) => setScript(event.target.value)}
                  className="min-h-72 w-full resize-y bg-transparent px-3.5 py-3 text-body leading-7 text-fg-1 outline-none placeholder:text-fg-3"
                />
              </div>
            )}
          </Field>

          <div className="flex flex-col gap-2">
            <label htmlFor={IMAGES_INPUT_ID} className="w-fit text-label uppercase tracking-wide text-fg-2">
              Capturas y recursos
            </label>
            <input
              id={IMAGES_INPUT_ID}
              type="file"
              accept="image/png,image/jpeg,image/webp"
              multiple
              className="peer sr-only"
              // Reset so picking the same files again still fires onChange.
              onClick={(event) => {
                event.currentTarget.value = '';
              }}
              onChange={selectImages}
            />
            <div>
              {/* A <label> rather than a button with a scripted .click(): the
                  native association keeps the picker keyboard-operable, and the
                  peer ring puts the focus indicator on the visible 44px target
                  instead of on the 1px sr-only input. */}
              <Button
                asChild
                variant="outline"
                className="peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-ring"
              >
                <label htmlFor={IMAGES_INPUT_ID}>
                  <ImagePlus aria-hidden />
                  Elegir imágenes
                </label>
              </Button>
            </div>
            <p className="text-body-sm text-fg-3">
              {images.length === 0
                ? 'Añade el post, la noticia oficial y reacciones.'
                : `${images.length} recurso(s) seleccionado(s) para esta sesión.`}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-3 border-t border-border-subtle pt-5">
            <Button type="submit" variant="hero">
              <Save aria-hidden />
              Guardar borrador local
            </Button>
            {draftSavedAt !== '' ? (
              <StatusTag size="md" tone="success" icon={CheckCircle2}>
                Guardado {new Date(draftSavedAt).toLocaleString('es-ES')}
              </StatusTag>
            ) : null}
            {draftError !== '' ? (
              <p role="alert" className="text-body-sm text-destructive">
                {draftError}
              </p>
            ) : null}
          </div>
        </form>
      </NewsPanel>

      <div className="flex flex-col gap-6">
        <NewsPanel
          icon={Mic2}
          eyebrow="VOZ"
          title="Tu voz"
          description="Perfil privado para las narraciones de RaizerinhoCS2."
          aside={<VoiceStateTag profile={profile} error={voiceError} />}
        >
          <div className="flex flex-col gap-5">
            {profile === undefined ? (
              <div role="status" aria-label="Cargando el perfil de voz local" className="flex flex-col gap-2">
                <Skeleton className="h-11 w-full rounded-none" />
                <Skeleton className="h-11 w-full rounded-none" />
              </div>
            ) : null}

            {profile === null ? (
              <p className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
                Todavía no hay una referencia guardada en este equipo.
              </p>
            ) : null}

            {profile !== undefined && profile !== null ? (
              <div className="flex flex-col gap-3 border border-success/45 bg-success/8 p-4">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex min-w-0 flex-col gap-1">
                    <p className="truncate font-display text-body-lg font-bold text-fg-1">{profile.name}</p>
                    <p className="font-mono text-meta uppercase tracking-wider text-fg-2">
                      {profile.channel} · {profile.locale} · {formatBytes(profile.size_bytes)}
                    </p>
                  </div>
                  <ShieldCheck aria-label="Guardada localmente" className="size-5 shrink-0 text-success" />
                </div>
                <audio
                  key={profile.updated_at}
                  controls
                  preload="metadata"
                  src={`${profile.audio_url}?v=${encodeURIComponent(profile.updated_at)}`}
                  className="w-full"
                />
                <p className="break-all border-t border-success/25 pt-3 font-mono text-meta text-fg-2">
                  SHA-256 {profile.sha256}
                </p>
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  className="self-start"
                  loading={voiceBusy}
                  onClick={() => void removeVoice()}
                >
                  {voiceBusy ? null : <Trash2 aria-hidden />}
                  Eliminar voz local
                </Button>
              </div>
            ) : null}

            <form className="flex flex-col gap-3" onSubmit={(event) => void uploadVoice(event)}>
              <label htmlFor={VOICE_INPUT_ID} className="w-fit text-label uppercase tracking-wide text-fg-2">
                {profile === null ? 'Guardar referencia' : 'Reemplazar referencia'}
              </label>
              <input
                id={VOICE_INPUT_ID}
                type="file"
                accept="audio/ogg,audio/wav,.ogg,.wav"
                disabled={voiceBusy}
                className="peer sr-only"
                // Reset so picking the same file again still fires onChange.
                onClick={(event) => {
                  event.currentTarget.value = '';
                }}
                onChange={selectVoice}
              />
              <div className="flex flex-wrap items-center gap-3">
                <Button
                  asChild
                  variant="outline"
                  className="peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-ring peer-disabled:pointer-events-none peer-disabled:opacity-50"
                >
                  <label htmlFor={VOICE_INPUT_ID}>
                    <FileAudio aria-hidden />
                    Elegir audio
                  </label>
                </Button>
                {voiceFile !== null ? (
                  <span className="max-w-52 truncate font-mono text-body-sm text-fg-2" title={voiceFile.name}>
                    {voiceFile.name}
                  </span>
                ) : (
                  <span className="text-body-sm text-fg-3">Ningún audio seleccionado.</span>
                )}
              </div>
              <p className="text-body-sm text-fg-3">
                OGG Opus o WAV clásico PCM. Recomendado: entre 10 y 30 segundos, sin música ni ruido. Máximo 25 MB.
              </p>
              <Button type="submit" variant="secondary" className="self-start" loading={voiceBusy} disabled={voiceFile === null}>
                {voiceBusy ? null : <Upload aria-hidden />}
                {profile === null ? 'Guardar voz' : 'Reemplazar voz'}
              </Button>
            </form>

            {voiceError !== '' ? (
              <p role="alert" className="border border-destructive/45 bg-destructive/8 px-4 py-3 text-body-sm text-destructive">
                {voiceError}
              </p>
            ) : null}
          </div>
        </NewsPanel>

        <NewsPanel icon={ShieldCheck} eyebrow="PRIVACIDAD" title="Dónde vive la muestra">
          <div className="flex flex-col gap-2">
            <StudioDataRow label="Ubicación" value="Datos locales de ClipHub" />
            <StudioDataRow label="Repositorio / instalador" value="No incluida" />
            <StudioDataRow label="Envío a terceros" value="Ninguno" />
          </div>
          <p className="mt-4 text-body-sm text-fg-2">
            ClipHub no la envía a xAI, YouTube ni a ningún proveedor de voz. Puedes escucharla, reemplazarla o
            eliminarla aquí.
          </p>
        </NewsPanel>
      </div>
    </div>
  );
}

/** The voice profile's state as one tag: loading, saved, absent or failed. */
function VoiceStateTag({ profile, error }: { profile: NewsVoiceProfile | null | undefined; error: string }): ReactNode {
  if (error !== '') {
    return (
      <StatusTag tone="danger" dot>
        Error
      </StatusTag>
    );
  }
  if (profile === undefined) {
    return (
      <StatusTag icon={LoaderCircle} className="[&_svg]:animate-spin">
        Cargando
      </StatusTag>
    );
  }
  if (profile === null) {
    return <StatusTag dot>Sin referencia</StatusTag>;
  }
  return (
    <StatusTag tone="success" dot>
      Guardada
    </StatusTag>
  );
}

/**
 * The one panel shape /news uses. It replaces the `Card`/`CardHeader`/`CardTitle`
 * stack this screen was the last user of, so a news panel and a Studio panel are
 * now the same object: mono eyebrow, uppercase display title, icon plate.
 */
function NewsPanel({
  icon,
  eyebrow,
  title,
  description,
  aside,
  className,
  children,
}: {
  icon: LucideIcon;
  eyebrow: string;
  title: string;
  description?: string;
  aside?: ReactNode;
  className?: string;
  children: ReactNode;
}): ReactNode {
  return (
    <section className={cn('studio-panel flex flex-col gap-5 p-5 sm:p-6', className)}>
      <div className="flex items-start gap-4">
        <IconTile icon={icon} size="md" depth="inset" />
        <div className="flex min-w-0 flex-1 flex-col gap-1.5">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <SectionEyebrow label={eyebrow} />
            {aside}
          </div>
          <h2 className="font-display text-title font-bold uppercase text-fg-1">{title}</h2>
          {description ? <p className="text-body-sm text-fg-2">{description}</p> : null}
        </div>
      </div>
      <div className="min-w-0">{children}</div>
    </section>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
