import { createHash } from 'node:crypto';
import * as fs from 'node:fs';
import * as http from 'node:http';
import * as https from 'node:https';

export interface DownloadOptions {
  signal?: AbortSignal;
  headers?: http.OutgoingHttpHeaders;
  onProgress?: (received: number, total: number | undefined) => void;
}

export interface FetchTextOptions {
  signal?: AbortSignal;
  headers?: http.OutgoingHttpHeaders;
  maxBytes?: number;
}

interface FetchStreamOptions {
  redirectsLeft?: number;
  signal?: AbortSignal;
  headers?: http.OutgoingHttpHeaders;
}

const DEFAULT_TEXT_MAX_BYTES = 256 * 1024;

// A stalled socket should not consume the caller's larger provisioning budget.
const DOWNLOAD_SOCKET_IDLE_TIMEOUT_MS = 60_000;

// Rename into place only after the complete body arrives, then return its SHA-256.
export async function downloadFile(
  url: string,
  destination: string,
  { signal, headers, onProgress }: DownloadOptions = {},
): Promise<string> {
  const temporary = `${destination}.tmp`;
  fs.rmSync(temporary, { force: true });

  try {
    const response = await fetchStream(url, { signal, headers });
    const total = Number(response.headers['content-length']) || undefined;
    const hash = createHash('sha256');
    let received = 0;
    await new Promise<void>((resolve, reject) => {
      const output = fs.createWriteStream(temporary);
      let settled = false;
      const onAbort = (): void => fail(new Error('download aborted'));
      const finish = (err?: Error): void => {
        if (settled) return;
        settled = true;
        signal?.removeEventListener('abort', onAbort);
        if (err) {
          reject(err);
        } else {
          resolve();
        }
      };
      const fail = (err: Error): void => {
        response.destroy();
        output.destroy();
        finish(err);
      };

      response.on('data', (chunk: Buffer) => {
        hash.update(chunk);
        received += chunk.length;
        onProgress?.(received, total);
      });
      response.pipe(output);
      response.on('error', fail);
      output.on('error', fail);
      output.on('finish', () => finish());

      if (signal) {
        if (signal.aborted) {
          onAbort();
        } else {
          signal.addEventListener('abort', onAbort, { once: true });
        }
      }
    });

    fs.renameSync(temporary, destination);
    return hash.digest('hex');
  } catch (err) {
    fs.rmSync(temporary, { force: true });
    throw err;
  }
}

// Bounded UTF-8 body; oversized payloads are rejected.
export async function fetchText(
  url: string,
  { signal, headers, maxBytes = DEFAULT_TEXT_MAX_BYTES }: FetchTextOptions = {},
): Promise<string> {
  const response = await fetchStream(url, { signal, headers });
  const chunks: Buffer[] = [];
  let received = 0;
  await new Promise<void>((resolve, reject) => {
    let settled = false;
    const onAbort = (): void => fail(new Error('download aborted'));
    const finish = (err?: Error): void => {
      if (settled) return;
      settled = true;
      signal?.removeEventListener('abort', onAbort);
      if (err) reject(err);
      else resolve();
    };
    const fail = (err: Error): void => {
      response.destroy();
      finish(err);
    };
    response.on('data', (chunk: Buffer) => {
      received += chunk.length;
      if (received > maxBytes) {
        fail(new Error(`GET ${url}: response too large`));
        return;
      }
      chunks.push(chunk);
    });
    response.on('error', fail);
    response.on('end', () => finish());
    if (signal) {
      if (signal.aborted) onAbort();
      else signal.addEventListener('abort', onAbort, { once: true });
    }
  });
  return Buffer.concat(chunks).toString('utf8');
}

// Opens an HTTP(S) response and follows a small, bounded redirect chain.
function fetchStream(
  url: string,
  { redirectsLeft = 5, signal, headers }: FetchStreamOptions = {},
): Promise<http.IncomingMessage> {
  return new Promise((resolve, reject) => {
    const handleResponse = (response: http.IncomingMessage): void => {
      const code = response.statusCode;
      if (code !== undefined && code >= 300 && code < 400 && response.headers.location && redirectsLeft > 0) {
        response.resume();
        resolve(fetchStream(new URL(response.headers.location, url).toString(), {
          redirectsLeft: redirectsLeft - 1,
          signal,
          headers,
        }));
        return;
      }
      if (code !== 200) {
        response.resume();
        reject(new Error(`GET ${url}: HTTP ${code}`));
        return;
      }
      resolve(response);
    };

    const requestOptions = { signal, headers };
    const request = url.startsWith('https:')
      ? https.get(url, requestOptions, handleResponse)
      : http.get(url, requestOptions, handleResponse);
    request.on('error', reject);
    request.setTimeout(DOWNLOAD_SOCKET_IDLE_TIMEOUT_MS, () => {
      request.destroy(new Error(`GET ${url}: timed out`));
    });
  });
}
