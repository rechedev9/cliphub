'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { FormError } from '@/components/clipmind/auth-forms';
import {
  cm,
  isDone,
  parseApiKeys,
  parseMe,
  parseSecret,
  parseSessions,
  type CmApiKey,
  type CmMe,
  type CmSession,
} from '@/lib/clipmind/client';

interface Loaded {
  me: CmMe;
  sessions: CmSession[];
  keys: CmApiKey[];
}

const section = 'flex flex-col gap-3 border-t border-border pt-6';

function when(ms: number): string {
  return ms > 0 ? new Date(ms).toLocaleString('es-ES') : 'nunca';
}

/** Account page: sessions, assistant keys, data export and deleting the account. */
export function AccountView(): ReactNode {
  const router = useRouter();
  const [data, setData] = useState<Loaded | undefined>();
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
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
      setError(me.message);
      return;
    }
    setData({ me: me.data, sessions: sessions.ok ? sessions.data : [], keys: keys.ok ? keys.data : [] });
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function run<T>(action: Promise<{ ok: true; data: T } | { ok: false; message: string }>, after?: (value: T) => void): Promise<void> {
    setBusy(true);
    setError(undefined);
    const result = await action;
    if (result.ok) {
      after?.(result.data);
      await load();
    } else {
      setError(result.message);
    }
    setBusy(false);
  }

  async function signOut(): Promise<void> {
    await cm('auth/logout', { method: 'POST', parse: isDone });
    router.replace('/iniciar-sesion');
    router.refresh();
  }

  async function createKey(event: FormEvent): Promise<void> {
    event.preventDefault();
    await run(cm('auth/tokens', { method: 'POST', body: { name: keyName }, parse: parseSecret }), (value) => {
      setSecret(value);
      setKeyName('');
    });
  }

  async function deleteAccount(): Promise<void> {
    if (!window.confirm('¿Borrar tu cuenta? Se eliminan tus datos y los espacios de los que eres el único miembro. No se puede deshacer.')) return;
    setBusy(true);
    const result = await cm('auth/account', { method: 'DELETE', parse: isDone });
    if (result.ok) {
      router.replace('/iniciar-sesion');
      router.refresh();
      return;
    }
    setError(result.message);
    setBusy(false);
  }

  if (data === undefined) {
    return error ? <FormError message={error} /> : <p className="text-body-sm text-fg-3">Cargando…</p>;
  }
  const { me, sessions, keys } = data;

  return (
    <div className="measure-work flex flex-col gap-6">
      <FormError message={error} />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-body-sm text-fg-1">
          {me.user.display_name === '' ? me.user.email : me.user.display_name} <span className="text-fg-3">{me.user.email}</span>
        </p>
        <Button type="button" variant="outline" onClick={() => void signOut()}>
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
        <form onSubmit={(event) => void createKey(event)} className="flex items-end gap-2">
          <Field label="Nombre de la clave" className="max-w-xs">
            {(control) => <Input {...control} value={keyName} maxLength={80} placeholder="Claude" onChange={(event) => setKeyName(event.target.value)} required />}
          </Field>
          <Button type="submit" variant="outline" disabled={busy}>
            Crear clave
          </Button>
        </form>
        {keys.length > 0 ? (
          <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
            {keys.map((key) => (
              <li key={key.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
                <span className="min-w-0 truncate text-body-sm text-fg-1">
                  {key.name} <span className="text-fg-3">· último uso {when(key.last_used_at)}</span>
                </span>
                <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void run(cm(`auth/tokens/${key.id}`, { method: 'DELETE', parse: isDone }))}>
                  Revocar
                </Button>
              </li>
            ))}
          </ul>
        ) : null}
      </div>

      <div className={section}>
        <h2 className="text-body-lg font-semibold text-fg-1">Sesiones</h2>
        <ul className="flex flex-col divide-y divide-border rounded-md border border-border">
          {sessions.map((session) => (
            <li key={session.id} className="flex items-center justify-between gap-4 px-4 py-2.5">
              <span className="min-w-0 truncate text-body-sm text-fg-1">
                {session.device === '' ? 'Dispositivo' : session.device} <span className="text-fg-3">· {when(session.last_used_at)}</span>
              </span>
              {session.current ? (
                <span className="text-body-sm text-primary">Esta sesión</span>
              ) : (
                <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void run(cm(`auth/sessions/${session.id}`, { method: 'DELETE', parse: isDone }))}>
                  Cerrar
                </Button>
              )}
            </li>
          ))}
        </ul>
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
          <Button type="button" variant="destructive" disabled={busy} onClick={() => void deleteAccount()}>
            Borrar mi cuenta
          </Button>
        </div>
      </div>
    </div>
  );
}
