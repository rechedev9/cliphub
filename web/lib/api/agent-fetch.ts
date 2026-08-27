import { AGENT_TRANSPORT_STORAGE_KEY, parseStoredAgentConfig } from '../agent-bootstrap.ts';

/**
 * Browser upload streams cannot be re-streamed by a service worker to the
 * agent's HTTP/1.1 loopback server. Send FormData directly so Chromium owns
 * the upload stream and never buffers a multi-gigabyte demo or VOD in JS.
 */
export function agentAwareFetch(input: string, init?: RequestInit): Promise<Response> {
  if (typeof window === 'undefined' || !(init?.body instanceof FormData)) return fetch(input, init);
  const config = parseStoredAgentConfig(localStorage.getItem(AGENT_TRANSPORT_STORAGE_KEY));
  if (config === null || !input.startsWith('/api/')) return fetch(input, init);

  const headers = new Headers(init.headers);
  headers.set('Authorization', `Bearer ${config.capability}`);
  return fetch(new URL(input, config.origin), {
    ...init,
    headers,
    credentials: 'omit',
    mode: 'cors',
    redirect: 'manual',
    referrerPolicy: 'no-referrer',
  });
}
