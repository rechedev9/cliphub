'use client';

import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import Link from '@/src/compat/link';
import { useRouter } from '@/src/compat/navigation';
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
  syncSteamMatches,
  type SteamAccount as SteamAccountState,
} from '@/lib/api/steam-account';
import { importShareCode } from '@/lib/api/steam-import';

const STEAM_AUTH_CODE_URL = 'https://help.steampowered.com/en/wizard/HelpWithGameIssue/?appid=730&issueid=128';
const STEAM_API_KEY_URL = 'https://steamcommunity.com/dev/apikey';

export function SteamAccount(): ReactNode {
  const router = useRouter();
  const [account, setAccount] = useState<SteamAccountState | null>(null);
  const [offline, setOffline] = useState(false);
  const [error, setError] = useState<string | undefined>();
  const [pending, setPending] = useState<'save' | 'sync' | 'clear' | string | null>(null);
  const [downloadCode, setDownloadCode] = useState<string | null>(null);

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
    setError(result.message);
  }

  useEffect(() => {
    void refresh();
  }, []);

  async function onSave(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setPending('save');
    setError(undefined);
    const result = await saveSteamAccount({
      steamId: String(form.get('steamId') ?? ''),
      authCode: String(form.get('authCode') ?? ''),
      apiKey: String(form.get('apiKey') ?? ''),
      knownCode: String(form.get('knownCode') ?? ''),
    });
    setPending(null);
    if (result.kind === 'ok') {
      setAccount(result.account);
      return;
    }
    if (result.kind === 'offline') {
      setOffline(true);
      return;
    }
    setError(result.message);
  }

  async function onSync(): Promise<void> {
    setPending('sync');
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
    setError(result.message);
  }

  async function onClear(): Promise<void> {
    setPending('clear');
    setError(undefined);
    const result = await clearSteamAccount();
    setPending(null);
    if (result.kind === 'ok') {
      setAccount(result.account);
      return;
    }
    setError(result.kind === 'offline' ? 'El servicio local no está en marcha.' : result.message);
  }

  async function onDownload(code: string): Promise<void> {
    setPending(code);
    setError(undefined);
    const result = await importShareCode(code);
    setPending(null);
    if (result.kind === 'queued') {
      router.push('/matches');
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
    <section className="studio-panel flex flex-col gap-5 p-5 sm:p-6" aria-labelledby="steam-account-title">
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
        <Field label="SteamID64" hint="El número de 17 cifras de tu perfil, o la URL /profiles/…">
          {(control) => (
            <Input
              {...control}
              name="steamId"
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
        >
          {(control) => (
            <Input {...control} name="authCode" autoComplete="off" spellCheck={false} className="font-mono" />
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
        >
          {(control) => (
            <Input {...control} name="apiKey" type="password" autoComplete="off" />
          )}
        </Field>
        <Field label="Un código de partida conocido" hint="El primero que copies de CS2. Sirve para arrancar la cadena.">
          {(control) => (
            <Input
              {...control}
              name="knownCode"
              defaultValue={account?.knownCode ?? ''}
              autoComplete="off"
              spellCheck={false}
              className="font-mono"
              placeholder="CSGO-"
            />
          )}
        </Field>
        {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
        <div className="flex flex-wrap gap-3">
          <Button type="submit" loading={pending === 'save'} loadingText="GUARDANDO">
            GUARDAR
          </Button>
          <Button
            type="button"
            variant="secondary"
            disabled={!account?.historyConfigured}
            loading={pending === 'sync'}
            loadingText="SINCRONIZANDO"
            onClick={() => { void onSync(); }}
          >
            SINCRONIZAR PARTIDAS
          </Button>
          {account?.historyConfigured ? (
            <Button type="button" variant="ghost" loading={pending === 'clear'} onClick={() => { void onClear(); }}>
              DESCONECTAR
            </Button>
          ) : null}
        </div>
      </form>

      {account && account.matches.length > 0 ? (
        <ol className="flex flex-col gap-2">
          {account.matches.map((match) => (
            <li key={match.shareCode} className="flex flex-col gap-2 border border-border bg-surface-1 px-3 py-3 sm:flex-row sm:items-center">
              <div className="min-w-0 flex-1">
                <p className="font-mono text-body-sm tabular-nums text-fg-1">Partida {match.matchId}</p>
                <p className="truncate font-mono text-meta text-fg-3">{match.shareCode}</p>
              </div>
              <Button
                type="button"
                size="sm"
                loading={pending === match.shareCode}
                loadingText="ENCOLANDO"
                onClick={() => { void onDownload(match.shareCode); }}
              >
                DESCARGAR
              </Button>
            </li>
          ))}
        </ol>
      ) : null}

      <p className="border border-dashed border-warning/45 bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
        {account?.gcConfigured
          ? 'La sesión de descarga ya está lista en este PC. ClipHub solo la abre cuando pulsas Descargar.'
          : 'La primera descarga te pedirá usuario, contraseña y Steam Guard. No se escriben a disco.'}{' '}
        Si estás jugando, Steam te echará de la partida. Sigue el flujo en{' '}
        <Link href="/onboarding" className={`text-primary underline underline-offset-2 ${FOCUS_RING}`}>
          Inicio
        </Link>
        .
      </p>

      <SteamDownloadDialog
        open={downloadCode !== null}
        code={downloadCode ?? ''}
        onOpenChange={(open) => { if (!open) setDownloadCode(null); }}
        onQueued={() => { router.push('/matches'); }}
      />
    </section>
  );
}
