/// <reference lib="webworker" />

export {};

declare global {
  interface WorkerGlobalScope {
    readonly clients: Clients;
    skipWaiting(): Promise<void>;
    addEventListener(type: 'install', listener: (event: ExtendableEvent) => void): void;
    addEventListener(type: 'activate', listener: (event: ExtendableEvent) => void): void;
    addEventListener(type: 'message', listener: (event: ExtendableMessageEvent) => void): void;
    addEventListener(type: 'fetch', listener: (event: FetchEvent) => void): void;
  }
}

const CONFIG_CACHE = 'cliphub-agent-config-v1';
const CONFIG_KEY = '/__cliphub_agent_config__';
const CAPABILITY_PATTERN = /^[a-f0-9]{64}$/;
const LOOPBACK_ORIGIN_PATTERN = /^http:\/\/(127(?:\.\d{1,3}){3}|localhost|\[::1\]):\d{1,5}$/i;

interface AgentConfig {
  readonly version: 1;
  readonly origin: string;
  readonly capability: string;
}

interface StreamingRequestInit extends RequestInit {
  readonly duplex: 'half';
}

const workerScope = self;

workerScope.addEventListener('install', (event: ExtendableEvent) => {
  event.waitUntil(workerScope.skipWaiting());
});

workerScope.addEventListener('activate', (event: ExtendableEvent) => {
  event.waitUntil(workerScope.clients.claim());
});

workerScope.addEventListener('message', (event: ExtendableMessageEvent) => {
  const config = parseConfig(event.data);
  if (config === null) return;
  event.waitUntil(storeConfig(config).then(() => event.ports[0]?.postMessage({ configured: true })));
});

workerScope.addEventListener('fetch', (event: FetchEvent) => {
  const url = new URL(event.request.url);
  if (url.origin !== workerScope.location.origin || !shouldProxy(url.pathname)) return;
  event.respondWith(proxyToAgent(event.request, url));
});

function shouldProxy(pathname: string): boolean {
  return pathname.startsWith('/api/')
    && !pathname.startsWith('/api/account/')
    && !pathname.startsWith('/api/agent/')
    && pathname !== '/api/installer';
}

async function proxyToAgent(request: Request, sourceURL: URL): Promise<Response> {
  const config = await loadConfig();
  if (config === null) return unavailable('ClipHub Agent no está conectado.');

  const target = new URL(`${sourceURL.pathname}${sourceURL.search}`, config.origin);
  const headers = new Headers(request.headers);
  headers.set('Authorization', `Bearer ${config.capability}`);
  const init: StreamingRequestInit = {
    method: request.method,
    headers,
    body: request.method === 'GET' || request.method === 'HEAD' ? null : request.body,
    cache: 'no-store',
    credentials: 'omit',
    mode: 'cors',
    redirect: 'manual',
    referrerPolicy: 'no-referrer',
    signal: request.signal,
    duplex: 'half',
  };
  try {
    return await fetch(target, init);
  } catch (error: unknown) {
    const detail = error instanceof Error ? ` (${error.message})` : '';
    return unavailable(`ClipHub Agent no responde en este PC.${detail}`);
  }
}

async function storeConfig(config: AgentConfig): Promise<void> {
  const cache = await caches.open(CONFIG_CACHE);
  await cache.put(CONFIG_KEY, Response.json(config, { headers: { 'Cache-Control': 'no-store' } }));
}

async function loadConfig(): Promise<AgentConfig | null> {
  const cache = await caches.open(CONFIG_CACHE);
  const response = await cache.match(CONFIG_KEY);
  if (response === undefined) return null;
  try {
    return parseConfig(await response.json());
  } catch {
    return null;
  }
}

function parseConfig(value: unknown): AgentConfig | null {
  if (typeof value !== 'object' || value === null) return null;
  const candidate = value as Record<string, unknown>;
  if (
    candidate.version !== 1
    || typeof candidate.origin !== 'string'
    || !validLoopbackOrigin(candidate.origin)
    || typeof candidate.capability !== 'string'
    || !CAPABILITY_PATTERN.test(candidate.capability)
  ) return null;
  return { version: 1, origin: candidate.origin, capability: candidate.capability };
}

function validLoopbackOrigin(value: string): boolean {
  if (!LOOPBACK_ORIGIN_PATTERN.test(value)) return false;
  try {
    const parsed = new URL(value);
    const port = Number(parsed.port);
    return parsed.origin === value && Number.isInteger(port) && port >= 1 && port <= 65_535;
  } catch {
    return false;
  }
}

function unavailable(error: string): Response {
  return Response.json(
    { code: 'service_unavailable', error },
    { status: 503, headers: { 'Cache-Control': 'no-store' } },
  );
}
