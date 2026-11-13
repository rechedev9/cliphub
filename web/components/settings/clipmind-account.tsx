'use client';

import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { StatusTag } from '@/components/studio/status-tag';
import { SettingsSection } from './settings-section';

interface Status {
  connected: boolean;
  serverUrl: string;
  login: string;
  workspaceName: string;
  lastSyncAt: number;
  lastError: string;
  pending: number;
  syncing: boolean;
}

const DEFAULT_SERVER = 'https://clipmind.example';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function readStatus(value: unknown): Status | undefined {
  if (!isRecord(value) || typeof value.connected !== 'boolean') return undefined;
  return {
    connected: value.connected,
    serverUrl: text(value.server_url),
    login: text(value.login),
    workspaceName: text(value.workspace_name),
    lastSyncAt: typeof value.last_sync_at === 'number' ? value.last_sync_at : 0,
    lastError: text(value.last_error),
    pending: typeof value.pending_changes === 'number' ? value.pending_changes : 0,
    syncing: value.syncing === true,
  };
}

async function call(path: string, method: string, body?: unknown): Promise<{ status?: Status; error?: string }> {
  try {
    const response = await fetch(`/api/clipmind/${path}`, {
      method,
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: 'no-store',
    });
    const data: unknown = await response.json().catch(() => undefined);
    if (!response.ok) {
      const detail = isRecord(data) && typeof data.error === 'string' ? data.error : '';
      return { error: detail === '' ? 'No se pudo completar la acción.' : detail };
    }
    const status = readStatus(data);
    return status === undefined ? { error: 'Respuesta inesperada.' } : { status };
  } catch {
    return { error: 'Studio no responde.' };
  }
}

function lastSync(status: Status): string {
  if (status.syncing) return 'Sincronizando…';
  if (status.lastSyncAt === 0) return 'Aún sin sincronizar';
  return `Última sincronización: ${new Date(status.lastSyncAt).toLocaleString('es-ES')}`;
}

/** Ajustes > Cuenta ClipMind: link this library to a ClipMind account so the team sees it on the web. */
export function ClipMindAccount(): ReactNode {
  const [status, setStatus] = useState<Status | undefined>();
  const [error, setError] = useState<string | undefined>();
  const [busy, setBusy] = useState(false);
  const [server, setServer] = useState(DEFAULT_SERVER);
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');

  const refresh = useCallback(async (): Promise<Status | undefined> => {
    const result = await call('account', 'GET');
    if (result.status !== undefined) setStatus(result.status);
    return result.status;
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // While the first sync runs, keep reading the status until it settles.
  const syncing = status?.syncing === true;
  useEffect(() => {
    if (!syncing) return;
    const timer = window.setInterval(() => void refresh(), 2000);
    return () => window.clearInterval(timer);
  }, [syncing, refresh]);

  async function act(path: string, method: string, body?: unknown): Promise<void> {
    setBusy(true);
    setError(undefined);
    const result = await call(path, method, body);
    if (result.status !== undefined) setStatus(result.status);
    if (result.error !== undefined) setError(result.error);
    setBusy(false);
  }

  async function connect(event: FormEvent): Promise<void> {
    event.preventDefault();
    await act('account', 'POST', { server_url: server, login, password });
    setPassword('');
  }

  return (
    <SettingsSection
      id="clipmind"
      title="Cuenta ClipMind"
      description="Conecta esta biblioteca con tu cuenta de ClipMind: tu equipo ve las partidas y el análisis en la web. Las demos no se suben, solo el análisis."
      status={
        status === undefined ? null : (
          <StatusTag tone={status.connected ? 'success' : 'neutral'}>{status.connected ? 'Conectada' : 'Sin conectar'}</StatusTag>
        )
      }
    >
      {status?.connected === true ? (
        <div className="flex flex-col gap-3">
          <p className="text-body-sm text-fg-1">
            {status.login} <span className="text-fg-3">· {status.workspaceName}</span>
          </p>
          <p className="text-body-sm text-fg-2">
            {lastSync(status)}
            {status.pending > 0 ? ` · ${status.pending} cambios por enviar` : null}
          </p>
          {status.lastError === '' ? null : (
            <p role="alert" className="text-body-sm text-destructive">
              {status.lastError}
            </p>
          )}
          <div className="flex gap-2">
            <Button type="button" variant="outline" disabled={busy || status.syncing} onClick={() => void act('sync', 'POST')}>
              Sincronizar ahora
            </Button>
            <Button type="button" variant="outline" disabled={busy} onClick={() => void act('account', 'DELETE')}>
              Desconectar
            </Button>
          </div>
        </div>
      ) : (
        <form onSubmit={(event) => void connect(event)} className="flex max-w-sm flex-col gap-4">
          <Field label="Servidor" hint="La dirección de tu ClipMind.">
            {(control) => <Input {...control} type="url" value={server} onChange={(event) => setServer(event.target.value)} required />}
          </Field>
          <Field label="Usuario o correo" required>
            {(control) => (
              <Input {...control} autoComplete="username" autoCapitalize="none" spellCheck={false} value={login} onChange={(event) => setLogin(event.target.value)} required />
            )}
          </Field>
          <Field label="Contraseña" required>
            {(control) => <Input {...control} type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required />}
          </Field>
          <Button type="submit" loading={busy}>
            Conectar
          </Button>
        </form>
      )}
      {error === undefined ? null : (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      )}
    </SettingsSection>
  );
}
