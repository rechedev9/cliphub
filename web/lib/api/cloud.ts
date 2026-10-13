import {
  cloudAccountState,
  parseCloudJob,
  parseCloudJobs,
  type CloudAccountState,
  type CloudJob,
} from '../cloud/parse.ts';
import type { CloudSubmitBody } from '../cloud/submit.ts';
import { dataPlane } from './dataplane.ts';

/** A rejected cloud call, with the stable `code` the UI maps to a sentence. */
export class CloudApiError extends Error {
  readonly code: string | undefined;
  readonly status: number;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'CloudApiError';
    this.status = status;
    this.code = code;
  }
}

async function readBody(res: Response): Promise<unknown> {
  try {
    const body: unknown = await res.json();
    return body;
  } catch {
    return null;
  }
}

function field(body: unknown, key: string): string | undefined {
  if (typeof body !== 'object' || body === null || !(key in body)) return undefined;
  const value: unknown = Reflect.get(body, key);
  return typeof value === 'string' ? value : undefined;
}

async function failure(res: Response): Promise<CloudApiError> {
  const body = await readBody(res);
  return new CloudApiError(field(body, 'error') ?? `request failed (${res.status})`, res.status, field(body, 'code'));
}

function post(url: string, body?: unknown): Promise<Response> {
  const init: RequestInit = { method: 'POST', cache: 'no-store' };
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' };
    init.body = JSON.stringify(body);
  }
  return fetch(url, init);
}

async function accountFrom(res: Response): Promise<CloudAccountState> {
  return cloudAccountState({ status: res.status, ok: res.ok, body: await readBody(res) });
}

async function jobFrom(res: Response): Promise<CloudJob> {
  if (!res.ok) throw await failure(res);
  const job = parseCloudJob(await readBody(res));
  if (job === null) throw new CloudApiError('unexpected cloud job response', res.status);
  return job;
}

/**
 * The ClipHub cloud, through the local orchestrator. The browser never calls
 * the portal and never sees the device token.
 */
export const cloudApi = {
  /** Never throws: a failed read is a state the UI shows. */
  async account(): Promise<CloudAccountState> {
    try {
      return await accountFrom(await fetch(dataPlane().cloudAccountUrl, { cache: 'no-store' }));
    } catch {
      return { kind: 'offline' };
    }
  },

  /** Starts the device link; the returned account carries the code to show. */
  async startLink(): Promise<CloudAccountState> {
    const res = await post(dataPlane().cloudLinkUrl, {});
    if (!res.ok) throw await failure(res);
    return accountFrom(res);
  },

  async unlink(): Promise<void> {
    const res = await fetch(dataPlane().cloudLinkUrl, { method: 'DELETE', cache: 'no-store' });
    if (!res.ok) throw await failure(res);
  },

  async submit(body: CloudSubmitBody): Promise<CloudJob> {
    return jobFrom(await post(dataPlane().cloudJobsUrl, body));
  },

  /** A Studio without a cloud client has no cloud jobs: that is an empty list, not an error. */
  async jobs(): Promise<CloudJob[]> {
    const res = await fetch(dataPlane().cloudJobsUrl, { cache: 'no-store' });
    if (res.ok) return parseCloudJobs(await readBody(res));
    const state = cloudAccountState({ status: res.status, ok: false, body: await readBody(res) });
    if (state.kind === 'unavailable') return [];
    throw new CloudApiError(`request failed (${res.status})`, res.status);
  },

  async cancel(id: string): Promise<CloudJob> {
    return jobFrom(await post(dataPlane().cloudJobCancelUrl(id)));
  },

  /** Removes a finished job from this PC, with its downloaded videos. */
  async remove(id: string): Promise<void> {
    const res = await fetch(dataPlane().cloudJobUrl(id), { method: 'DELETE', cache: 'no-store' });
    if (!res.ok && res.status !== 404) throw await failure(res);
  },

  videoUrl(id: string, name: string): string {
    return dataPlane().cloudVideoUrl(id, name);
  },
};
