'use client';

import { useEffect, useId, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { KeyRound } from 'lucide-react';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { IconTile } from '@/components/studio/icon-tile';
import { StatusTag } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { SteamDownloadDialog } from '@/components/onboarding/steam-download-dialog';
import {
  clearSteamAccount,
  loadSteamAccount,
  saveSteamAccount,
  steamAccountFailure,
  syncSteamMatches,
  validateSteamAccountInput,
  type SteamAccount as SteamAccountState,
  type SteamAccountField,
  type SteamAccountFieldErrors,
} from '@/lib/api/steam-account';
import { importShareCode } from '@/lib/api/steam-import';
import { CLIPS_HREF, NEW_DEMO_HREF } from '@/lib/clips/routes';

const STEAM_AUTH_CODE_URL = 'https://help.steampowered.com/en/wizard/HelpWithGameIssue/?appid=730&issueid=128';
const STEAM_API_KEY_URL = 'https://steamcommunity.com/dev/apikey';

/** A share code can equal any string, so the download lane carries its code instead of being one. */
type Pending = { kind: 'save' | 'sync' | 'clear' } | { kind: 'download'; code: string } | null;

export function SteamAccount(): ReactNode {
  const router = useRouter();
  const [account, setAccount] = useState<SteamAccountState | null>(null);
  const [offline, setOffline] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [pending, setPending] = useState<Pending>(null);
  const [downloadCode, setDownloadCode] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<SteamAccountFieldErrors>({});
  const syncHintId = useId();

  function clearFieldError(field: SteamAccountField): void {
    setFieldErrors((current) => {
      if (current[field] === undefined) return current;
      const next = { ...current };
      delete next[field];
      return next;
    });
  }

  async function refresh(): Promise<void> {
    const result = await loadSteamAccount();
    if (result.kind === 'ok') {
      setAccount(result.account);
      setOffline(false);
      return;
    }
    if (result.kind === 'offline') {
      setOffline(true);
      return;
    }
    setError(steamAccountFailure(result, 'load').message);
  }

  useEffect(() => {
    void refresh();
  }, []);

  async function onSave(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const input = {
      steamId: String(form.get('steamId') ?? ''),
      authCode: String(form.get('authCode') ?? ''),
      apiKey: String(form.get('apiKey') ?? ''),
      knownCode: String(form.get('knownCode') ?? ''),
    };
    setError(undefined);
    // Same rules the orchestrator applies, checked first so an empty or
    // malformed form never becomes a PUT whose English error text reaches the UI.
    const invalid = validateSteamAccountInput(input, account);
    setFieldErrors(invalid);
    if (Object.keys(invalid).length > 0) return;
    setPending({ kind: 'save' });
    const result = await saveSteamAccount(input);
    setPending(null);
    if (result.kind === 'ok') {
      setAccount(result.account);
      return;
    }
    if (result.kind === 'offline') {
      setOffline(true);
      return;
    }
    showFailure(result, 'save');
  }

  function showFailure(failure: { message: string; code?: string }, action: 'save' | 'sync' | 'clear'): void {
    const mapped = steamAccountFailure(failure, action);
    const field = mapped.field;
    if (field) {
      setFieldErrors((current) => ({ ...current, [field]: mapped.message }));
      return;
    }
    setError(mapped.message);
  }

  async function onSync(): Promise<void> {
    setPending({ kind: 'sync' });
    setError(undefined);
    const result = await syncSteamMatches();
    setPending(null);
    if (result.kind === 'ok') {
      setAccount(result.account);
      return;
    }
    if (result.kind === 'offline') {
      setOffline(true);
      return;
    }
    showFailure(result, 'sync');
  }

  async function onClear(): Promise<void> {
    setPending({ kind: 'clear' });
    setError(undefined);
    const result = await clearSteamAccount();
    setPending(null);
    if (result.kind === 'ok') {
      setAccount(result.account);
      return;
    }
    if (result.kind === 'offline') {
      setError('El servicio local no está en marcha.');
      return;
    }
    showFailure(result, 'clear');
  }

  async function onDownload(code: string): Promise<void> {
    setPending({ kind: 'download', code });
    setError(undefined);
    const result = await importShareCode(code);
    setPending(null);
    if (result.kind === 'queued') {
      router.push(CLIPS_HREF);
      return;
    }
    if (result.kind === 'needCredentials') {
      setDownloadCode(code);
      return;
    }
    if (result.kind === 'offline') {
      setOffline(true);
      return;
    }
    setError(result.message);
  }

  let status: ReactNode;
  if (account === null && !offline) {
    status = <Skeleton aria-label="Comprobando la cuenta de Steam" className="h-7 w-32 rounded-none" />;
  } else if (offline) {
    status = <StatusTag tone="warning">Sin datos</StatusTag>;
  } else if (account?.historyConfigured) {
    status = <StatusTag tone="success">Historial conectado</StatusTag>;
  } else {
    status = <StatusTag tone="neutral">Sin configurar</StatusTag>;
  }

  return (
    <section className="studio-panel flex flex-col gap-5 p-4 @[34rem]/content:p-6" aria-labelledby="steam-account-title">
      <div className="flex items-center gap-4">
        <IconTile icon={KeyRound} size="md" depth="inset" />
        <div className="flex min-w-0 flex-col gap-1">
          <SectionEyebrow label="CUENTA CON LA QUE JUEGAS" />
          <h2 id="steam-account-title" className="font-display text-title font-bold uppercase text-fg-1">
            Steam
          </h2>
        </div>
        <div className="ml-auto shrink-0">{status}</div>
      </div>

      <p className="text-body-sm text-fg-2">
        El código de autenticación lista tus partidas recientes. No es tu contraseña y se revoca desde
        Steam. La contraseña solo se pide al descargar una demo, y no se guarda en este PC.
      </p>

      <form key={account?.steamId ?? 'empty'} onSubmit={(event) => { void onSave(event); }} className="flex flex-col gap-4">
        <Field label="SteamID64" hint="El número de 17 cifras de tu perfil, o la URL /profiles/…" error={fieldErrors.steamId}>
          {(control) => (
            <Input
              {...control}
              name="steamId"
              onChange={() => clearFieldError('steamId')}
              defaultValue={account?.steamId ?? ''}
              autoComplete="off"
              spellCheck={false}
              className="font-mono"
            />
          )}
        </Field>
        <Field
          label="Código de autenticación"
          hint={(
            <>
              Lo sacas en{' '}
              <a href={STEAM_AUTH_CODE_URL} target="_blank" rel="noreferrer" className={`text-primary underline underline-offset-2 ${FOCUS_RING}`}>
                el asistente de Steam
              </a>
              . {account?.authCodeSet ? 'Ya hay uno guardado; déjalo vacío para no cambiarlo.' : null}
            </>
          )}
          error={fieldErrors.authCode}
        >
          {(control) => (
            <Input
              {...control}
              name="authCode"
              onChange={() => clearFieldError('authCode')}
              autoComplete="off"
              spellCheck={false}
              className="font-mono"
              placeholder="AAAA-AAAAA-AAAA"
            />
          )}
        </Field>
        <Field
          label="Clave de la Web API"
          hint={(
            <>
              Valve la exige para enumerar códigos.{' '}
              <a href={STEAM_API_KEY_URL} target="_blank" rel="noreferrer" className={`text-primary underline underline-offset-2 ${FOCUS_RING}`}>
                steamcommunity.com/dev/apikey
              </a>
              . {account?.apiKeySet ? 'Ya hay una guardada; déjala vacía para no cambiarla.' : null}
            </>
          )}
          error={fieldErrors.apiKey}
        >
          {(control) => (
            <Input {...control} name="apiKey" type="password" autoComplete="off" onChange={() => clearFieldError('apiKey')} />
          )}
        </Field>
        <Field
          label="Un código de partida conocido"
          hint="El primero que copies de CS2. Sirve para arrancar la cadena."
          error={fieldErrors.knownCode}
        >
          {(control) => (
            <Input
              {...control}
              name="knownCode"
              onChange={() => clearFieldError('knownCode')}
              defaultValue={account?.knownCode ?? ''}
              autoComplete="off"
              spellCheck={false}
              className="font-mono"
              placeholder="CSGO-"
            />
          )}
        </Field>
        {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
        <div className="flex flex-col gap-2">
          <div className="flex flex-wrap gap-3">
            <Button type="submit" loading={pending?.kind === 'save'} loadingText="Guardando…">
              Guardar
            </Button>
            {/*
              Outline with a dashed edge when disabled: the secondary fill at 50%
              opacity read as "muted but clickable". The reason sits under the
              row rather than in a tooltip, since a disabled button gets no hover.
            */}
            <Button
              type="button"
              variant="outline"
              disabled={!account?.historyConfigured}
              aria-describedby={account?.historyConfigured ? undefined : syncHintId}
              loading={pending?.kind === 'sync'}
              loadingText="Sincronizando…"
              className="disabled:border-dashed"
              onClick={() => { void onSync(); }}
            >
              Sincronizar partidas
            </Button>
            {account?.historyConfigured ? (
              <Button type="button" variant="ghost" loading={pending?.kind === 'clear'} onClick={() => { void onClear(); }}>
                Desconectar
              </Button>
            ) : null}
          </div>
          {account?.historyConfigured ? null : (
            <p id={syncHintId} className="text-body-sm text-fg-3">
              Guarda primero tu SteamID64, el código de autenticación y la clave de la Web API para sincronizar tus partidas.
            </p>
          )}
        </div>
      </form>

      {account && account.matches.length > 0 ? (
        <ol className="flex flex-col gap-2">
          {account.matches.map((match) => (
            <li key={match.shareCode} className="flex flex-col gap-2 border border-border bg-surface-1 px-3 py-3 @[34rem]/content:flex-row @[34rem]/content:items-center">
              <div className="min-w-0 flex-1">
                <p className="font-mono text-body-sm tabular-nums text-fg-1">Partida {match.matchId}</p>
                <p className="truncate font-mono text-meta text-fg-3">{match.shareCode}</p>
              </div>
              <Button
                type="button"
                size="sm"
                loading={pending?.kind === 'download' && pending.code === match.shareCode}
                loadingText="Encolando…"
                onClick={() => { void onDownload(match.shareCode); }}
              >
                Descargar
              </Button>
            </li>
          ))}
        </ol>
      ) : null}

      <p className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
        {account?.gcConfigured
          ? 'La sesión de descarga ya está lista en este PC. ClipHub solo la abre cuando pulsas Descargar.'
          : 'La primera descarga te pedirá usuario, contraseña y Steam Guard. No se escriben a disco.'}{' '}
        Si estás jugando, Steam te echará de la partida. Sigue el flujo en{' '}
        <Link href={NEW_DEMO_HREF} className={`text-primary underline underline-offset-2 ${FOCUS_RING}`}>
          Cargar demo
        </Link>
        .
      </p>

      <SteamDownloadDialog
        open={downloadCode !== null}
        code={downloadCode ?? ''}
        onOpenChange={(open) => { if (!open) setDownloadCode(null); }}
        onQueued={() => { router.push(CLIPS_HREF); }}
      />
    </section>
  );
}
