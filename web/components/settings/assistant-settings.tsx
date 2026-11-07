'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Check, Copy } from 'lucide-react';
import {
  claudeCodeCommand,
  claudeDesktopConfigSnippet,
  getDesktopSettingsBridge,
  parseAssistantAccess,
  type StudioAssistantAccess,
} from '@/lib/desktop-settings';
import { writeClipboardText } from '@/lib/clipboard-write';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { SettingsSection } from './settings-section';

type AssistantState =
  | { kind: 'loading' }
  | { kind: 'unavailable' }
  | { kind: 'ready'; access: StudioAssistantAccess };

/** Lets an assistant such as Claude drive this Studio; off until the user turns it on. */
export function AssistantSettings(): ReactNode {
  const [state, setState] = useState<AssistantState>({ kind: 'loading' });
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState(false);

  useEffect(() => {
    const bridge = getDesktopSettingsBridge();
    if (bridge?.getAssistantAccess === undefined) {
      setState({ kind: 'unavailable' });
      return;
    }
    let active = true;
    void bridge.getAssistantAccess()
      .then((value) => {
        if (!active) return;
        const access = parseAssistantAccess(value);
        setState(access === null ? { kind: 'unavailable' } : { kind: 'ready', access });
      })
      .catch(() => { if (active) setState({ kind: 'unavailable' }); });
    return () => { active = false; };
  }, []);

  const update = (enabled: boolean): void => {
    const bridge = getDesktopSettingsBridge();
    if (bridge?.updateAssistantAccess === undefined) return;
    setPending(true);
    setFailure(false);
    void bridge.updateAssistantAccess(enabled)
      .then((value) => {
        const access = parseAssistantAccess(value);
        if (access === null) setFailure(true);
        else setState({ kind: 'ready', access });
      })
      .catch(() => setFailure(true))
      .finally(() => setPending(false));
  };

  let body: ReactNode;
  if (state.kind === 'ready') {
    const { access } = state;
    body = (
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-4 border border-border bg-surface-2 px-3.5 py-3">
          <span id="assistant-switch-label" className="font-display text-body-sm font-bold text-fg-1">
            Permitir que un asistente use Studio
          </span>
          <Switch
            checked={access.enabled}
            disabled={pending}
            aria-labelledby="assistant-switch-label"
            onCheckedChange={update}
          />
        </div>
        {failure ? (
          <p role="alert" className="text-body-sm text-destructive">
            No se pudo guardar el cambio. Inténtalo de nuevo.
          </p>
        ) : null}
        {access.enabled ? (
          <div className="flex flex-wrap gap-2">
            <CopyButton
              label="Copiar configuración de Claude Desktop"
              text={claudeDesktopConfigSnippet(access.mcpPath)}
            />
            <CopyButton label="Copiar comando de Claude Code" text={claudeCodeCommand(access.mcpPath)} />
          </div>
        ) : null}
      </div>
    );
  } else if (state.kind === 'unavailable') {
    body = (
      <p className="border border-dashed border-border-strong bg-surface-1 px-4 py-3 text-body-sm text-fg-2">
        El acceso de asistentes solo está disponible en la app de escritorio, en su última versión.
      </p>
    );
  } else {
    body = (
      <div role="status" aria-label="Leyendo acceso de asistentes" className="flex flex-col gap-4">
        <Skeleton className="h-14 w-full rounded-none" />
        <Skeleton className="h-12 w-full rounded-none" />
      </div>
    );
  }

  return (
    <SettingsSection
      id="assistants"
      title="Asistentes (Claude, ChatGPT)"
      description="Un asistente conectado puede importar demos, crear Shorts y consultar tus partidas y vídeos. Antes de grabar te pide tu aprobación en el chat. Solo funciona mientras Studio está abierto."
    >
      {body}
    </SettingsSection>
  );
}

function CopyButton({ label, text }: { label: string; text: string }): ReactNode {
  const [outcome, setOutcome] = useState<'idle' | 'copied' | 'failed'>('idle');

  useEffect(() => {
    if (outcome === 'idle') return;
    const timer = setTimeout(() => setOutcome('idle'), 2_000);
    return () => clearTimeout(timer);
  }, [outcome]);

  const copy = (): void => {
    void writeClipboardText(text)
      .then(() => setOutcome('copied'))
      .catch(() => setOutcome('failed'));
  };

  return (
    <div className="flex flex-col gap-1">
      <Button type="button" variant="outline" size="sm" onClick={copy}>
        {outcome === 'copied' ? <Check aria-hidden /> : <Copy aria-hidden />}
        {outcome === 'copied' ? 'Copiado' : label}
      </Button>
      {outcome === 'failed' ? (
        <p role="alert" className="text-meta text-destructive">No se pudo copiar al portapapeles.</p>
      ) : null}
    </div>
  );
}
