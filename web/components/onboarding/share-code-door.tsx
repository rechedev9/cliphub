'use client';

import Link from '@/src/compat/link';
import { useRouter } from '@/src/compat/navigation';
import { useState, type FormEvent, type ReactElement, type ReactNode } from 'react';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { StatusTag } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { SteamDownloadDialog } from '@/components/onboarding/steam-download-dialog';
import { resolveShareCode, type ShareCodeResolution } from '@/lib/api/share-code-resolve';
import { importShareCode } from '@/lib/api/steam-import';
import { checkShareCode } from '@/lib/sharecode';

const STEPS = [
  'Abre CS2 y ve a VER → Tus partidas.',
  'Pulsa el icono de descarga de la partida: copia un código que empieza por CSGO-.',
  'Pégalo aquí abajo.',
] as const;

const STEAM_AUTH_CODE_URL = 'https://help.steampowered.com/en/wizard/HelpWithGameIssue/?appid=730&issueid=128';

type DoorState =
  | { phase: 'idle' }
  | { phase: 'format-error'; message: string }
  | { phase: 'pending' }
  | { phase: 'done'; result: ShareCodeResolution };

/** Paste a CS2 match share code, then enqueue its demo. */
export function ShareCodeDoor(): ReactElement {
  const router = useRouter();
  const [state, setState] = useState<DoorState>({ phase: 'idle' });
  const [downloadCode, setDownloadCode] = useState<string | null>(null);
  const [downloadError, setDownloadError] = useState<string | undefined>();
  const [downloading, setDownloading] = useState(false);
  const [lastCode, setLastCode] = useState('');

  async function onSubmit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const raw = new FormData(event.currentTarget).get('sharecode');
    const check = checkShareCode(typeof raw === 'string' ? raw : '');
    if (!check.ok) {
      setState({ phase: 'format-error', message: check.message });
      return;
    }
    setState({ phase: 'pending' });
    const code = typeof raw === 'string' ? raw.trim() : '';
    setLastCode(code);
    setState({ phase: 'done', result: await resolveShareCode(code) });
  }

  async function download(code: string): Promise<void> {
    setDownloading(true);
    setDownloadError(undefined);
    const result = await importShareCode(code);
    setDownloading(false);
    if (result.kind === 'queued') {
      router.push('/matches');
      return;
    }
    if (result.kind === 'needCredentials') {
      setDownloadCode(code);
      return;
    }
    if (result.kind === 'offline') {
      setDownloadError('El servicio local de ClipHub no está en marcha.');
      return;
    }
    setDownloadError(result.message);
  }

  const result = state.phase === 'done' ? state.result : null;
  let fieldError: string | undefined;
  if (state.phase === 'format-error') {
    fieldError = state.message;
  } else if (result !== null && (result.kind === 'invalid' || result.kind === 'failed')) {
    fieldError = result.message;
  }

  return (
    <section className="studio-panel flex flex-col gap-4 p-4 @[34rem]/content:p-6">
      <SectionEyebrow label="¿YA TIENES EL CÓDIGO DE UNA PARTIDA?" />

      <p className="text-body-sm text-fg-2">
        Ese código identifica una partida concreta de CS2; ClipHub lo lee aquí.
      </p>

      <ol className="flex flex-col gap-2">
        {STEPS.map((step, index) => (
          <li key={step} className="flex items-baseline gap-3">
            <span className="w-5 shrink-0 font-mono text-meta tabular-nums text-fg-3">
              {String(index + 1).padStart(2, '0')}
            </span>
            <span className="text-body-sm text-fg-2">{step}</span>
          </li>
        ))}
      </ol>

      <form onSubmit={(event) => { void onSubmit(event); }} noValidate>
        <Field
          label="Código de partida"
          hint="Tiene la forma CSGO-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX."
          error={fieldError}
        >
          {(control) => (
            <div className="flex flex-col gap-3 @[34rem]/content:flex-row">
              <Input
                {...control}
                name="sharecode"
                className="w-full min-w-0 font-mono"
                placeholder="CSGO-"
                autoComplete="off"
                spellCheck={false}
              />
              <Button type="submit" loading={state.phase === 'pending'} loadingText="COMPROBANDO">
                COMPROBAR
              </Button>
            </div>
          )}
        </Field>
      </form>

      <div aria-live="polite" className="flex flex-col gap-2">
        {result !== null ? (
          <ResolutionOutcome
            result={result}
            downloading={downloading}
            downloadError={downloadError}
            onDownload={() => { void download(lastCode); }}
          />
        ) : null}
      </div>

      <SteamDownloadDialog
        open={downloadCode !== null}
        code={downloadCode ?? ''}
        onOpenChange={(open) => { if (!open) setDownloadCode(null); }}
        onQueued={() => { router.push('/matches'); }}
      />

      <p className="text-meta text-fg-3">
        Sincronizar tu historial completo, en vez de pegar códigos uno a uno, necesitaría el{' '}
        <a
          href={STEAM_AUTH_CODE_URL}
          target="_blank"
          rel="noreferrer"
          className={`text-primary underline underline-offset-2 ${FOCUS_RING}`}
        >
          código de autenticación de Steam
        </a>{' '}
        de la cuenta con la que juegas: el historial vive en esa cuenta y en ninguna otra. Esa página
        te pide iniciar sesión en Steam.
      </p>
    </section>
  );
}

function ResolutionOutcome({
  result,
  downloading,
  downloadError,
  onDownload,
}: {
  result: ShareCodeResolution;
  downloading: boolean;
  downloadError?: string;
  onDownload: () => void;
}): ReactNode {
  switch (result.kind) {
    case 'decoded':
    case 'resolved':
      return (
        <>
          <StatusTag tone="success">Código válido</StatusTag>
          <p className="font-mono text-body-sm tabular-nums text-fg-1">Partida {result.matchId}</p>
          <p className="text-body-sm text-fg-2">
            {result.kind === 'resolved'
              ? 'La demo está en los servidores de Valve. Encolarla usa el mismo flujo que Subir demo.'
              : (
                <>
                  ClipHub ya sabe qué partida es. Para bajarla, la primera vez te pedirá el login de
                  Steam de la cuenta con la que juegas. El código de autenticación para listar el
                  historial está en{' '}
                  <Link href="/settings" className={`text-primary underline underline-offset-2 ${FOCUS_RING}`}>
                    Ajustes
                  </Link>
                  .
                </>
              )}
          </p>
          <Button type="button" loading={downloading} loadingText="ENCOLANDO" onClick={onDownload}>
            DESCARGAR DEMO
          </Button>
          {downloadError ? <p className="text-body-sm text-destructive">{downloadError}</p> : null}
        </>
      );
    case 'offline':
      return (
        <>
          <StatusTag tone="warning">Servicio apagado</StatusTag>
          <p className="text-body-sm text-fg-2">
            El servicio local de ClipHub no está en marcha, así que el código no se pudo comprobar.
            Ábrelo y vuelve a intentarlo.
          </p>
        </>
      );
    default:
      return null;
  }
}
