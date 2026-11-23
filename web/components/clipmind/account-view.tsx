'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { LoadingText } from '@/components/ui/loading-text';
import { FormError } from '@/components/clipmind/auth-forms';
import { CmLoadError } from '@/components/clipmind/cm-load-error';
import {
  accountLabel,
  cm,
  isDone,
  parseApiKeys,
  parseMe,
  parseSecret,
  parseSessions,
  type CmApiKey,
  type CmMe,
  type CmResult,
  type CmSession,
} from '@/lib/clipmind/client';

type Loaded = { me: CmMe; sessions: CmResult<CmSession[]>; keys: CmResult<CmApiKey[]> };

type LoadState = { kind: 'loading' } | { kind: 'failed'; failure: CmResult<never> } | { kind: 'ready'; data: Loaded };

type Action = 'create-key' | 'revoke-key' | 'close-session' | 'sign-out';

const section = 'flex flex-col gap-3 border-t border-border pt-6';

function when(ms: number): string {
  return ms > 0 ? new Date(ms).toLocaleString('es-ES') : 'nunca';
}

export function AccountView(): ReactNode {
  const router = useRouter();
  const [state, setState] = useState<LoadState>({ kind: 'loading' });
  const [error, setError] = useState<string | undefined>();
  const [pending, setPending] = useState<{ action: Action; id?: string } | null>(null);
  const [keyName, setKeyName] = useState('');
  const [secret, setSecret] = useState<string | undefined>();
  const mcpUrl = typeof window === 'undefined' ? '' : `${window.location.origin}/mcp`;

  const load = useCallback(async (): Promise<void> => {
    const [me, sessions, keys] = await Promise.all([
      cm('auth/me', { parse: parseMe }),
      cm('auth/sessions', { parse: parseSessions }),
      cm('auth/tokens', { parse: parseApiKeys }),
    ]);
    if (!me.ok) {
      setState({ kind: 'failed', failure: me });
      return;
    }
    setState({ kind: 'ready', data: { me: me.data, sessions, keys } });
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  function retry(): void {
    setState({ kind: 'loading' });
    void load();
  }

  async function run<T>({
    action,
    id,
    call,
    done,
    after,
  }: {
    action: Action;
    id?: string;
    call: () => Promise<CmResult<T>>;
    done?: string;
    after?: (value: T) => void;
  }): Promise<void> {
    setPending({ action, id });
    setError(undefined);
    const result = await call();
    if (result.ok) {
      after?.(result.data);
      if (done !== undefined) toast.success(done);
      await load();
    } else {
      setError(result.message);
    }
    setPending(null);
  }

  async function signOut(): Promise<void> {
    setPending({ action: 'sign-out' });
    await cm('auth/logout', { method: 'POST', parse: isDone });
    router.replace('/iniciar-sesion');
    router.refresh();
  }

  function createKey(event: FormEvent): void {
    event.preventDefault();
    void run({
      action: 'create-key',
      call: () => cm('auth/tokens', { method: 'POST', body: { name: keyName }, parse: parseSecret }),
      after: (value) => {
        setSecret(value);
        setKeyName('');
      },
    });
  }

  async function deleteAccount(): Promise<void> {
    setError(undefined);
    const result = await cm('auth/account', { method: 'DELETE', parse: isDone });
    if (result.ok) {
      router.replace('/iniciar-sesion');
      router.refresh();
      return;
    }
    setError(result.message);
  }

  if (state.kind === 'loading') return <LoadingText spinner>Cargando tu cuenta</LoadingText>;
  if (state.kind === 'failed') return <CmLoadError failure={state.failure} title="No se pudo leer tu cuenta" onRetry={retry} />;

  const { me, sessions, keys } = state.data;
  const who = accountLabel(me.user);
  const busy = pending !== null;
  const isPending = (action: Action, id?: string): boolean => pending?.action === action && pending.id === id;

  return (
    <div className="measure-work flex flex-col gap-6">
      <FormError message={error} />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-body-sm text-fg-1">
          {who.name} <span className="text-fg-3">{who.detail}</span>
        </p>
        <Button type="button" variant="outline" disabled={busy} loading={isPending('sign-out')} onClick={() => void signOut()}>
          Salir
        </Button>
      </div>

      <div className={section}>
        <h2 className="text-body-lg font-semibold text-fg-1">Asistentes</h2>
        <p className="text-body-sm text-fg-2">
          Una clave deja que un asistente como Claude lea y edite el espacio en uso. Añade este servidor MCP en su configuración: <code className="text-fg-1">{mcpUrl}</code> con la clave como Bearer.
        </p>
        {secret === undefined ? null : (
          <div role="status" className="flex flex-col gap-1 rounded-md border border-primary p-3">
            <p className="text-body-sm text-fg-1">Copia la clave ahora; no volverá a mostrarse.</p>
            <code className="text-body-sm break-all text-primary">{secret}</code>
          </div>
        )}
        <form onSubmit={createKey} className="flex items-end gap-2">
          <Field label="Nombre de la clave" className="max-w-xs">
            {(control) => <Input {...control} value={keyName} maxLength={80} placeholder="Claude" onChange={(event) => setKeyName(event.target.value)} required />}
          </Field>
          <Button type="submit" variant="outline" disabled={busy} loading={isPending('create-key')}>
            Crear clave
          </Button>
        </form>
        {keys.ok ? null : <CmLoadError failure={keys} title="No se pudieron leer tus claves" onRetry={retry} />}
        {keys.ok && keys.data.length > 0 ? (
          <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
            {keys.data.map((key) => (
              <li key={key.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                <span className="min-w-0 truncate text-body-sm text-fg-1">
                  {key.name} <span className="text-fg-3">· último uso {when(key.last_used_at)}</span>
                </span>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={busy}
                  loading={isPending('revoke-key', key.id)}
                  onClick={() =>
                    void run({
                      action: 'revoke-key',
                      id: key.id,
                      call: () => cm(`auth/tokens/${key.id}`, { method: 'DELETE', parse: isDone }),
                      done: `Clave ${key.name} revocada.`,
                    })
                  }
                >
                  Revocar
                </Button>
              </li>
            ))}
          </ul>
        ) : null}
      </div>

      <div className={section}>
        <h2 className="text-body-lg font-semibold text-fg-1">Sesiones</h2>
        {!sessions.ok ? (
          <CmLoadError failure={sessions} title="No se pudieron leer tus sesiones" onRetry={retry} />
        ) : (
          <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
            {sessions.data.map((session) => (
              <li key={session.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                <span className="min-w-0 truncate text-body-sm text-fg-1">
                  {session.device === '' ? 'Dispositivo' : session.device} <span className="text-fg-3">· {when(session.last_used_at)}</span>
                </span>
                {session.current ? (
                  <span className="text-body-sm text-primary">Esta sesión</span>
                ) : (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    disabled={busy}
                    loading={isPending('close-session', session.id)}
                    onClick={() =>
                      void run({
                        action: 'close-session',
                        id: session.id,
                        call: () => cm(`auth/sessions/${session.id}`, { method: 'DELETE', parse: isDone }),
                        done: 'Sesión cerrada en ese dispositivo.',
                      })
                    }
                  >
                    Cerrar
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className={section}>
        <h2 className="text-body-lg font-semibold text-fg-1">Tus datos</h2>
        <p className="text-body-sm text-fg-2">Descarga todo lo que ClipMind guarda de tu cuenta, o bórrala.</p>
        <div className="flex gap-2">
          <Button asChild variant="outline">
            <a href="/api/cm/auth/export" download>
              Descargar mis datos
            </a>
          </Button>
          <ConfirmDialog
            title="¿Borrar tu cuenta?"
            description="Se borran tus datos y los espacios de los que eres el único miembro. No se puede deshacer."
            confirmLabel="Borrar mi cuenta"
            pendingLabel="Borrando…"
            onConfirm={deleteAccount}
            trigger={
              <Button type="button" variant="destructive" disabled={busy}>
                Borrar mi cuenta
              </Button>
            }
          />
        </div>
      </div>
    </div>
  );
}
