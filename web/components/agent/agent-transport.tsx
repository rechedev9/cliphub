'use client';

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactElement,
  type ReactNode,
} from 'react';
import {
  AGENT_TRANSPORT_STORAGE_KEY,
  parseAgentFragment,
  parseStoredAgentConfig,
  type AgentTransportConfig,
} from '@/lib/agent-bootstrap';

export type AgentTransportState =
  | { readonly status: 'local' }
  | { readonly status: 'connecting' }
  | { readonly status: 'disconnected' }
  | { readonly status: 'ready'; readonly config: AgentTransportConfig }
  | { readonly status: 'error'; readonly error: string };

interface AgentTransportContextValue {
  readonly state: AgentTransportState;
  readonly reconnect: () => void;
}

const AgentTransportContext = createContext<AgentTransportContextValue | null>(null);

export function AgentTransportProvider({ hosted, children }: { hosted: boolean; children: ReactNode }): ReactElement {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<AgentTransportState>(hosted ? { status: 'connecting' } : { status: 'local' });
  const reconnect = useCallback(() => setAttempt((value) => value + 1), []);

  useEffect(() => {
    if (!hosted) return;
    const controller = new AbortController();
    void connectTransport(controller.signal).then(setState).catch((error: unknown) => {
      if (controller.signal.aborted) return;
      setState({ status: 'error', error: error instanceof Error ? error.message : 'No se pudo conectar con ClipHub Agent.' });
    });
    return () => controller.abort();
  }, [attempt, hosted]);

  const value = useMemo<AgentTransportContextValue>(() => ({ state, reconnect }), [state, reconnect]);
  return <AgentTransportContext.Provider value={value}>{children}</AgentTransportContext.Provider>;
}

export function useAgentTransport(): AgentTransportContextValue {
  const value = useContext(AgentTransportContext);
  if (value === null) throw new Error('useAgentTransport requires AgentTransportProvider');
  return value;
}

async function connectTransport(signal: AbortSignal): Promise<AgentTransportState> {
  if (!('serviceWorker' in navigator)) {
    return { status: 'error', error: 'Este navegador no admite la conexión local requerida por ClipHub.' };
  }
  const fragmentConfig = parseAgentFragment(window.location.hash);
  if (fragmentConfig !== null) {
    localStorage.setItem(AGENT_TRANSPORT_STORAGE_KEY, JSON.stringify(fragmentConfig));
    window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}`);
  }
  const config = fragmentConfig ?? parseStoredAgentConfig(localStorage.getItem(AGENT_TRANSPORT_STORAGE_KEY));
  if (config === null) return { status: 'disconnected' };

  const registration = await navigator.serviceWorker.register('/generated/agent-sw.js', {
    scope: '/',
    type: 'module',
    updateViaCache: 'none',
  });
  await navigator.serviceWorker.ready;
  if (signal.aborted) throw new DOMException('aborted', 'AbortError');
  const worker = await controlledWorker(registration, signal);
  if (worker === null) {
    return { status: 'error', error: 'Recarga la página para terminar de activar la conexión local.' };
  }
  await configureWorker(worker, config, signal);
  const response = await fetch('/api/capabilities', { cache: 'no-store', signal });
  if (!response.ok) return { status: 'error', error: 'ClipHub Agent está instalado, pero no responde en este PC.' };
  return { status: 'ready', config };
}

async function controlledWorker(registration: ServiceWorkerRegistration, signal: AbortSignal): Promise<ServiceWorker | null> {
  if (navigator.serviceWorker.controller !== null) return navigator.serviceWorker.controller;
  if (registration.active === null && registration.waiting === null) return null;
  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(() => finish(null), 5_000);
    const finish = (worker: ServiceWorker | null): void => {
      window.clearTimeout(timeout);
      navigator.serviceWorker.removeEventListener('controllerchange', changed);
      signal.removeEventListener('abort', aborted);
      resolve(worker);
    };
    const changed = (): void => finish(navigator.serviceWorker.controller);
    const aborted = (): void => {
      window.clearTimeout(timeout);
      navigator.serviceWorker.removeEventListener('controllerchange', changed);
      reject(new DOMException('aborted', 'AbortError'));
    };
    navigator.serviceWorker.addEventListener('controllerchange', changed);
    signal.addEventListener('abort', aborted, { once: true });
  });
}

function configureWorker(worker: ServiceWorker, config: AgentTransportConfig, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const channel = new MessageChannel();
    const timeout = window.setTimeout(() => finish(new Error('ClipHub Agent no pudo guardar la conexión local.')), 5_000);
    const finish = (error?: Error): void => {
      window.clearTimeout(timeout);
      signal.removeEventListener('abort', aborted);
      channel.port1.close();
      if (error) reject(error); else resolve();
    };
    const aborted = (): void => finish(new DOMException('aborted', 'AbortError'));
    channel.port1.onmessage = () => finish();
    signal.addEventListener('abort', aborted, { once: true });
    worker.postMessage(config, [channel.port2]);
  });
}
