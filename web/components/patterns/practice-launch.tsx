'use client';

import { useCallback, useState, type ReactNode } from 'react';
import { Copy, Crosshair } from 'lucide-react';
import { launchPractice, type PracticeLaunchOutcome, type PracticeSpot } from '@/lib/api/practice';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export type PracticeState = { kind: 'idle' } | { kind: 'busy' } | { kind: 'done'; outcome: PracticeLaunchOutcome };

/** Launches CS2 practice on a map and keeps the last answer for the panel. */
export function usePracticeLaunch(map: string): {
  state: PracticeState;
  launch: (spots: PracticeSpot[]) => void;
  dismiss: () => void;
} {
  const [state, setState] = useState<PracticeState>({ kind: 'idle' });
  const launch = useCallback(
    (spots: PracticeSpot[]) => {
      if (spots.length === 0) return;
      setState({ kind: 'busy' });
      void launchPractice({ map, spots }).then((outcome) => setState({ kind: 'done', outcome }));
    },
    [map],
  );
  const dismiss = useCallback(() => setState({ kind: 'idle' }), []);
  return { state, launch, dismiss };
}

/** "Practicar en CS2": disabled with the reason as its title when the throw has no position. */
export function PracticeButton({
  onClick,
  disabledReason,
  busy,
  label = 'Practicar en CS2',
}: {
  onClick: () => void;
  disabledReason?: string;
  busy: boolean;
  label?: string;
}): ReactNode {
  const disabled = disabledReason !== undefined || busy;
  return (
    <span title={disabledReason}>
      <Button type="button" variant="outline" size="xs" disabled={disabled} onClick={onClick}>
        <Crosshair aria-hidden />
        {label}
      </Button>
    </span>
  );
}

function CopyCommand({ command }: { command: string }): ReactNode {
  const [copied, setCopied] = useState(false);
  return (
    <span className="flex flex-wrap items-center gap-2">
      <code className="rounded-sm bg-surface-3 px-2 py-1 font-mono text-body-sm text-fg-1">{command}</code>
      <Button
        type="button"
        variant="ghost"
        size="xs"
        onClick={() => {
          void navigator.clipboard
            .writeText(command)
            .then(() => setCopied(true))
            .catch(() => setCopied(false));
        }}
      >
        <Copy aria-hidden />
        {copied ? 'Copiado' : 'Copiar'}
      </Button>
    </span>
  );
}

function OutcomeText({ outcome }: { outcome: PracticeLaunchOutcome }): ReactNode {
  switch (outcome.kind) {
    case 'ok': {
      const { result } = outcome;
      const count = result.spots.length;
      const spots = `${count} ${count === 1 ? 'posición' : 'posiciones'}`;
      const cycle = count > 1 && result.nextKey !== '' ? ` Pulsa ${result.nextKey} para pasar a la siguiente.` : '';
      if (result.alreadyRunning || !result.launched) {
        return (
          <>
            <p>
              {result.alreadyRunning ? 'CS2 ya está abierto.' : 'La práctica está lista.'} Escribe esto en la consola de CS2
              para cargar {spots}.{cycle}
            </p>
            <CopyCommand command={result.execCommand} />
          </>
        );
      }
      return <p>CS2 se está abriendo con {spots}. Si Steam pide confirmación, acéptala.{cycle}</p>;
    }
    case 'cs2_not_found':
      return <p>No se encontró CS2 en este PC. Instálalo desde Steam y vuelve a intentarlo.</p>;
    case 'offline':
      return <p>El servicio local no responde. Comprueba que ClipHub Studio sigue abierto.</p>;
    case 'invalid':
    case 'failed':
      return (
        <p>
          No se pudo preparar la práctica. <span className="text-fg-3">{outcome.message}</span>
        </p>
      );
    default:
      return null;
  }
}

/** The answer of the last launch, shown beside the buttons that started it. */
export function PracticeResult({ state, onDismiss }: { state: PracticeState; onDismiss: () => void }): ReactNode {
  if (state.kind === 'idle') return null;
  const failed = state.kind === 'done' && state.outcome.kind !== 'ok';
  return (
    <div
      role="status"
      className={cn(
        'flex flex-col gap-2 rounded-md border bg-surface-2 px-3 py-2 text-body-sm text-fg-1',
        failed ? 'border-destructive/40' : 'border-border',
      )}
    >
      {state.kind === 'busy' ? <p className="text-fg-2">Preparando la práctica en CS2</p> : <OutcomeText outcome={state.outcome} />}
      {state.kind === 'done' ? (
        <Button type="button" variant="ghost" size="xs" className="self-start" onClick={onDismiss}>
          Cerrar
        </Button>
      ) : null}
    </div>
  );
}
