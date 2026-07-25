'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { CircleCheck, KeyRound, LoaderCircle, RefreshCw, Trash2, TriangleAlert, WifiOff } from 'lucide-react';
import { toast } from 'sonner';
import {
  getDesktopSettingsBridge,
  XAI_KEY_SOURCES,
  type DesktopSettingsBridge,
  type XAIConnectionTestResult,
  type XAIKeySource,
  type XAISettingsStatus,
} from '@/lib/desktop-settings';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { IconTile } from '@/components/studio/icon-tile';
import { StatusTag } from '@/components/studio/status-tag';
import { StudioDataRow } from '@/components/studio/data-row';
import { Button } from '@/components/ui/button';
import { DesktopOnlyCard } from '@/components/settings/desktop-only-card';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

const MAX_XAI_API_KEY_LENGTH = 4096;

const ACTIONS = {
  load: 'load',
  test: 'test',
  save: 'save',
  remove: 'remove',
  restart: 'restart',
} as const;

type Action = typeof ACTIONS[keyof typeof ACTIONS];

const SOURCE_LABELS: Record<XAIKeySource, string> = {
  [XAI_KEY_SOURCES.environment]: 'Variable de entorno',
  [XAI_KEY_SOURCES.stored]: 'Ajustes de Windows',
  [XAI_KEY_SOURCES.team]: 'Edición interna',
  [XAI_KEY_SOURCES.none]: 'Sin configurar',
};

/** What the desktop build does with the key, shown when the browser cannot. */
const DESKTOP_CAPABILITIES = [
  'La clave se cifra para tu usuario de Windows',
  'Nunca pasa por Next.js, el navegador ni la API local',
  'El cambio se aplica al reiniciar Studio',
] as const;

/** xAI credential settings backed exclusively by the Electron preload bridge. */
export function XAISettings(): ReactNode {
  const [bridge, setBridge] = useState<DesktopSettingsBridge | null>();
  const [status, setStatus] = useState<XAISettingsStatus | null>(null);
  const [apiKey, setAPIKey] = useState('');
  const [action, setAction] = useState<Action | null>(ACTIONS.load);
  const [error, setError] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<XAIConnectionTestResult | null>(null);

  const refreshStatus = useCallback(async (desktopBridge: DesktopSettingsBridge): Promise<void> => {
    try {
      setStatus(await desktopBridge.getXAIStatus());
      setError(null);
    } catch {
      setError('No se pudo leer la configuración protegida de xAI.');
    }
  }, []);

  useEffect(() => {
    const desktopBridge = getDesktopSettingsBridge();
    setBridge(desktopBridge);
    if (desktopBridge === null) {
      setAction(null);
      return;
    }

    let mounted = true;
    desktopBridge.getXAIStatus()
      .then((nextStatus) => {
        if (mounted) setStatus(nextStatus);
      })
      .catch(() => {
        if (mounted) setError('No se pudo leer la configuración protegida de xAI.');
      })
      .finally(() => {
        if (mounted) setAction(null);
      });
    return () => {
      mounted = false;
    };
  }, []);

  const runTest = async (): Promise<void> => {
    if (bridge === null || bridge === undefined) return;
    const candidate = apiKey.trim();
    if (candidate === '') {
      setError('Introduce una clave de xAI para probarla.');
      return;
    }
    setAction(ACTIONS.test);
    setError(null);
    setTestResult(null);
    try {
      const result = await bridge.testXAIKey(candidate);
      setTestResult(redactTestResult(result, candidate));
    } catch {
      setError('No se pudo probar la conexión con xAI.');
    } finally {
      setAction(null);
    }
  };

  const save = async (): Promise<void> => {
    if (bridge === null || bridge === undefined) return;
    const candidate = apiKey.trim();
    if (candidate === '') {
      setError('Introduce una clave de xAI para guardarla.');
      return;
    }
    setAction(ACTIONS.save);
    setError(null);
    try {
      const result = await bridge.saveXAIKey(candidate);
      if (!result.ok) {
        setError(result.error ?? 'No se pudo guardar la clave de xAI.');
        return;
      }
      setAPIKey('');
      setTestResult(null);
      if (result.status) {
        setStatus(result.status);
      } else {
        await refreshStatus(bridge);
      }
      toast('Clave de xAI guardada de forma protegida.');
    } catch {
      setError('No se pudo guardar la clave de xAI.');
    } finally {
      setAction(null);
    }
  };

  const remove = async (): Promise<void> => {
    if (bridge === null || bridge === undefined) return;
    setAction(ACTIONS.remove);
    setError(null);
    try {
      const result = await bridge.removeXAIKey();
      if (!result.ok) {
        setError(result.error ?? 'No se pudo eliminar la clave guardada.');
        return;
      }
      setAPIKey('');
      setTestResult(null);
      if (result.status) {
        setStatus(result.status);
      } else {
        await refreshStatus(bridge);
      }
      toast('Clave guardada eliminada. Reinicia FragForge para aplicar el cambio.');
    } catch {
      setError('No se pudo eliminar la clave guardada.');
    } finally {
      setAction(null);
    }
  };

  const restart = async (): Promise<void> => {
    if (bridge === null || bridge === undefined) return;
    const confirmed = window.confirm(
      'Reiniciar FragForge detendrá las grabaciones y renders que estén en curso. ¿Quieres continuar?',
    );
    if (!confirmed) return;
    setAction(ACTIONS.restart);
    setError(null);
    try {
      const result = await bridge.restartStudio();
      if (!result.ok) {
        setError(result.error ?? 'No se pudo reiniciar FragForge Studio.');
        setAction(null);
      }
    } catch {
      setError('No se pudo reiniciar FragForge Studio.');
      setAction(null);
    }
  };

  if (bridge === undefined || action === ACTIONS.load) return <SettingsSkeleton />;
  if (bridge === null) return <DesktopOnlyState />;

  const busy = action !== null;
  const candidateAvailable = apiKey.trim() !== '';
  const restartPending = status?.restartRequired === true;

  return (
    <section className="studio-panel flex flex-col gap-6 p-5 sm:p-6" aria-labelledby="xai-settings-title">
      <div className="flex flex-col gap-4 border-b border-border pb-6 @[40rem]/content:flex-row @[40rem]/content:items-start @[40rem]/content:justify-between @[40rem]/content:gap-6">
        <div className="flex min-w-0 items-start gap-4">
          <IconTile icon={KeyRound} size="lg" depth="inset" />
          <div className="flex min-w-0 flex-col gap-2">
            <SectionEyebrow label="CREDENCIALES" />
            <h2 id="xai-settings-title" className="font-display text-title font-bold uppercase text-fg-1">
              Subtítulos con Grok
            </h2>
            <p className="max-w-xl text-body text-fg-2">
              Usa tu propia clave de xAI para generar subtítulos. Se cifra con la protección de Windows y nunca se
              guarda en el navegador ni pasa por Next.js.
            </p>
          </div>
        </div>
        <ConnectionTag status={status} />
      </div>

      <div className="grid gap-2 @[34rem]/content:grid-cols-2 @[34rem]/content:gap-3">
        <StudioDataRow label="Activa ahora" value={status ? SOURCE_LABELS[status.activeSource] : 'Comprobando…'} />
        <StudioDataRow
          label="Tras reiniciar"
          active={restartPending}
          value={status ? SOURCE_LABELS[status.pendingSource] : 'Comprobando…'}
        />
      </div>

      {status?.storageError ? <Message tone="error">{status.storageError}</Message> : null}

      <Field
        label="Clave API de xAI"
        hint="FragForge nunca vuelve a mostrar una clave guardada. Probar comprueba la clave escrita; Guardar la cifra para este usuario de Windows."
      >
        {(control) => (
          <Input
            {...control}
            name="xai-api-key-new"
            type="password"
            autoComplete="new-password"
            spellCheck={false}
            maxLength={MAX_XAI_API_KEY_LENGTH}
            value={apiKey}
            onChange={(event) => setAPIKey(event.target.value)}
            placeholder={status?.stored ? 'Introduce una clave nueva para sustituir la guardada' : 'xai-…'}
            disabled={busy || status?.storageAvailable === false}
          />
        )}
      </Field>

      {testResult ? <Message tone={testResult.ok ? 'success' : 'error'}>{testResult.message}</Message> : null}
      {error ? <Message tone="error">{error}</Message> : null}

      <div className="flex flex-col gap-3 @[34rem]/content:flex-row @[34rem]/content:flex-wrap">
        <Button
          type="button"
          variant="outline"
          loading={action === ACTIONS.test}
          onClick={() => void runTest()}
          disabled={busy || !candidateAvailable}
        >
          {action === ACTIONS.test ? null : <RefreshCw aria-hidden />}
          Probar conexión
        </Button>
        <Button
          type="button"
          loading={action === ACTIONS.save}
          onClick={() => void save()}
          disabled={busy || !candidateAvailable || status?.storageAvailable === false}
        >
          {action === ACTIONS.save ? null : <KeyRound aria-hidden />}
          Guardar clave
        </Button>
        <Button
          type="button"
          variant="destructive"
          loading={action === ACTIONS.remove}
          onClick={() => void remove()}
          disabled={busy || !status?.stored}
        >
          {action === ACTIONS.remove ? null : <Trash2 aria-hidden />}
          Eliminar clave
        </Button>
      </div>

      <div
        className={cn(
          'flex flex-col gap-4 border p-4 @[40rem]/content:flex-row @[40rem]/content:items-center @[40rem]/content:justify-between',
          restartPending ? 'border-warning/45 bg-warning/5 shadow-[var(--elev-1)]' : 'border-border bg-surface-1',
        )}
      >
        <div className="flex min-w-0 gap-4">
          <IconTile icon={TriangleAlert} size="md" depth="inset" tone={restartPending ? 'warning' : 'neutral'} />
          <div className="flex min-w-0 flex-col gap-1">
            <p className="font-display text-body-sm font-bold uppercase tracking-wide text-fg-1">
              {restartPending ? 'Reinicio pendiente' : 'No hay cambios pendientes'}
            </p>
            <p className="text-body-sm text-fg-2">
              La clave solo se aplica al reiniciar. Reiniciar corta cualquier grabación o render que esté en curso.
            </p>
          </div>
        </div>
        <Button
          type="button"
          variant={restartPending ? 'warning' : 'secondary'}
          className="shrink-0"
          loading={action === ACTIONS.restart}
          onClick={() => void restart()}
          disabled={busy || !restartPending}
        >
          {action === ACTIONS.restart ? null : <RefreshCw aria-hidden />}
          Reiniciar ahora
        </Button>
      </div>
    </section>
  );
}

/** The credential's live state: checking, pending restart, active, or unset. */
function ConnectionTag({ status }: { status: XAISettingsStatus | null }): ReactNode {
  if (status === null) {
    return (
      <StatusTag size="md" icon={LoaderCircle} className="shrink-0 [&_svg]:animate-spin">
        Comprobando
      </StatusTag>
    );
  }
  if (status.restartRequired) {
    return (
      <StatusTag size="md" tone="warning" icon={TriangleAlert} className="shrink-0">
        Pendiente
      </StatusTag>
    );
  }
  if (status.active) {
    return (
      <StatusTag size="md" tone="success" icon={CircleCheck} className="shrink-0">
        Activa
      </StatusTag>
    );
  }
  return (
    <StatusTag size="md" icon={WifiOff} className="shrink-0">
      Sin configurar
    </StatusTag>
  );
}

function Message({ tone, children }: { tone: 'success' | 'error'; children: ReactNode }): ReactNode {
  return (
    <p
      role={tone === 'error' ? 'alert' : 'status'}
      className={cn(
        'border px-4 py-3 text-body-sm',
        tone === 'success'
          ? 'border-success/45 bg-success/8 text-success'
          : 'border-destructive/45 bg-destructive/8 text-destructive',
      )}
    >
      {children}
    </p>
  );
}

function DesktopOnlyState(): ReactNode {
  return (
    <DesktopOnlyCard
      titleId="desktop-only-title"
      title="Credenciales de subtítulos con Grok"
      capabilities={DESKTOP_CAPABILITIES}
    >
      Abre esta pantalla desde la aplicación de escritorio para guardar la clave con la protección de Windows. Por
      seguridad no existe una alternativa HTTP en el navegador.
    </DesktopOnlyCard>
  );
}

function SettingsSkeleton(): ReactNode {
  return (
    <section
      role="status"
      aria-label="Leyendo la configuración protegida de xAI"
      className="studio-panel flex items-center gap-4 p-5 sm:p-6"
    >
      <IconTile icon={KeyRound} size="lg" depth="inset" />
      <div className="flex min-w-0 flex-col gap-1">
        <SectionEyebrow label="CREDENCIALES" />
        <p className="inline-flex items-center gap-2.5 text-body text-fg-2">
          <LoaderCircle aria-hidden className="size-4 animate-spin text-primary" />
          Leyendo la configuración protegida…
        </p>
      </div>
    </section>
  );
}

function redactTestResult(result: XAIConnectionTestResult, candidate: string): XAIConnectionTestResult {
  if (candidate === '' || !result.message.includes(candidate)) return result;
  return { ...result, message: result.message.replaceAll(candidate, '[clave oculta]') };
}
