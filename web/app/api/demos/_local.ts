import { NextResponse } from 'next/server';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { parseControlJSONObject, prepareLocalUploadBody, readBoundedText } from '@/lib/api/bounded-request-body';
import { ANTICHEAT_DOCUMENT_KEYS } from '@/lib/api/anticheat';
import {
  TACTICAL_DOCUMENT_KEYS,
  TACTICAL_FILTER_PARAM_NAMES,
  TACTICAL_MAX_SAMPLE_HZ,
  TACTICAL_ROUND_KEYS,
  TACTICAL_SAMPLE_HZ_FIELD,
  TACTICAL_STATUS_KEYS,
  TACTICAL_TENDENCIES_KEYS,
} from '@/lib/api/tactical';
import { parseJobProgress } from '@/lib/job-progress';
import {
  orchestratorUrl,
  forwardError,
  callOrchestrator,
  callOrchestratorStreamingUpload,
  proxyStream,
  serviceUnavailable,
  jobUrl,
  jobsListUrl,
  seriesJobsUrl,
  UPLOAD_BODY_LIMIT_EXCEEDED,
} from './_lib';

/** Same-origin `/api/demos/*` proxy to the local orchestrator. */

// Orchestrator 700 MiB demo cap plus 1 MiB for multipart overhead.
const MAX_DEMO_REQUEST_BYTES = 701 * 1024 * 1024;

/** POST /api/demos/scan — forward the .dem as field `demo` to start a roster scan. */
export async function localScan(request: Request): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });

  const contentType = request.headers.get('content-type') ?? '';
  if (!contentType.toLowerCase().startsWith('multipart/form-data;')) {
    return NextResponse.json({ error: 'multipart/form-data required' }, { status: 400 });
  }

  const upload = await prepareLocalUploadBody(request, MAX_DEMO_REQUEST_BYTES);
  if (!upload.ok) return NextResponse.json({ error: upload.error }, { status: upload.status });

  const headers: Record<string, string> = { 'Content-Type': contentType };
  if (upload.contentLength !== undefined) headers['Content-Length'] = upload.contentLength;
  const res = await callOrchestratorStreamingUpload(`${orchestratorUrl()}/api/jobs`, {
    method: 'POST',
    headers,
    body: upload.body,
    duplex: 'half',
  }, upload.exceeded);
  if (res === UPLOAD_BODY_LIMIT_EXCEEDED) {
    return NextResponse.json({ error: 'file too large' }, { status: 413 });
  }
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  const { id } = (await res.json()) as { id: string };
  return NextResponse.json({ jobId: id }, { status: 201 });
}

/** GET /api/demos/{jobId}/status (local) - proxy the job's current status. */
export async function localStatus(jobId: string): Promise<Response> {
  const url = jobUrl(jobId, '?view=status');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  // Whitelist status, failure_reason, and live worker progress; drop anything else.
  const data = (await res.json()) as {
    status: string;
    failure_reason?: string;
    progress?: { done?: number; total?: number; percent?: number; unit?: string; label?: string; stage?: string };
  };
  const body: {
    status: string;
    failure_reason?: string;
    progress?: { done: number; total: number; percent?: number; unit?: string; label?: string; stage?: string };
  } = { status: data.status };
  if (data.failure_reason) body.failure_reason = data.failure_reason;
  const parsed = parseJobProgress(data.progress);
  if (parsed) body.progress = parsed;
  return NextResponse.json(body);
}

/** DELETE /api/demos/{jobId} — 204, or 409 while the job is still running. */
export async function localDeleteJob(jobId: string): Promise<Response> {
  const url = jobUrl(jobId);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url, { method: 'DELETE' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return new Response(null, { status: 204 });
}

/** GET /api/demos/{jobId}/roster — pass through players and optional match. */
export async function localRoster(jobId: string): Promise<Response> {
  const url = jobUrl(jobId, '/roster');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  // Keep match for toRosterMatch; omit it when the scan produced none.
  const body = (await res.json()) as { players: unknown[]; match?: unknown };
  const out: { players: unknown[]; match?: unknown } = { players: body.players };
  if (body.match !== undefined) out.match = body.match;
  return NextResponse.json(out);
}

/** GET /api/demos/series/{seriesId} — whitelist per-demo fields in upload order. */
export async function localSeries(seriesId: string): Promise<Response> {
  const url = seriesJobsUrl(seriesId);
  if (!url) return NextResponse.json({ error: 'invalid series id' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  type UpstreamJob = {
    id: string;
    status: string;
    failure_reason?: string;
    demo_file_name?: string;
    progress?: { done?: number; total?: number; percent?: number; unit?: string; label?: string; stage?: string };
  };
  const body = (await res.json()) as { jobs: UpstreamJob[] };
  const demos = body.jobs.map((job) => {
    const demo: {
      jobId: string;
      status: string;
      failureReason?: string;
      fileName?: string;
      progress?: { done: number; total: number; percent?: number; unit?: string; label?: string; stage?: string };
    } = {
      jobId: job.id,
      status: job.status,
    };
    if (job.failure_reason) demo.failureReason = job.failure_reason;
    if (job.demo_file_name) demo.fileName = job.demo_file_name;
    const parsed = parseJobProgress(job.progress);
    if (parsed) demo.progress = parsed;
    return demo;
  });
  return NextResponse.json({ demos });
}

/** GET /api/demos/jobs — whitelist recent jobs; never forward the kill plan. */
export async function localJobs(): Promise<Response> {
  const res = await callOrchestrator(jobsListUrl());
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  type UpstreamJob = {
    id: string;
    status: string;
    failure_reason?: string;
    demo_file_name?: string;
    series_id?: string;
    target_steamid?: string;
    created_at?: string;
    progress?: { done?: number; total?: number; percent?: number; unit?: string; label?: string; stage?: string };
  };
  const body = (await res.json()) as { jobs: UpstreamJob[] };
  const jobs = body.jobs.map((job) => {
    const out: {
      jobId: string;
      status: string;
      failureReason?: string;
      fileName?: string;
      seriesId?: string;
      targetSteamId?: string;
      createdAt?: string;
      progress?: { done: number; total: number; percent?: number; unit?: string; label?: string; stage?: string };
    } = { jobId: job.id, status: job.status };
    if (job.failure_reason) out.failureReason = job.failure_reason;
    if (job.demo_file_name) out.fileName = job.demo_file_name;
    if (job.series_id) out.seriesId = job.series_id;
    if (job.target_steamid) out.targetSteamId = job.target_steamid;
    if (job.created_at) out.createdAt = job.created_at;
    const parsed = parseJobProgress(job.progress);
    if (parsed) out.progress = parsed;
    return out;
  });
  return NextResponse.json({ jobs });
}

/** Copy only listed top-level keys so new orchestrator fields cannot leak. */
function forwardKeys(body: Record<string, unknown>, keys: readonly string[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const key of keys) {
    if (body[key] !== undefined) out[key] = body[key];
  }
  return out;
}

/** Reads a whitelisted upstream JSON object, or the error/503 that replaces it. */
async function forwardJson(url: string | null, keys: readonly string[], init?: RequestInit): Promise<Response> {
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url, init);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  const body = (await res.json()) as Record<string, unknown>;
  return NextResponse.json(forwardKeys(body, keys), { status: res.status });
}

/** Rounds are 1-based and a match never reaches four digits. */
const TACTICAL_ROUND_RE = /^[1-9][0-9]{0,2}$/;

/** GET /api/demos/{jobId}/tactical — proxy the analysis document. */
export async function localTacticalDocument(jobId: string): Promise<Response> {
  return forwardJson(jobUrl(jobId, '/tactical'), TACTICAL_DOCUMENT_KEYS);
}

/** POST /api/demos/{jobId}/tactical — start analysis; only sample Hz is accepted. */
export async function localStartTactical(request: Request, jobId: string): Promise<Response> {
  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });

  let init: RequestInit = { method: 'POST' };
  if (incoming.text.trim() !== '') {
    const parsed = parseControlJSONObject(incoming.text, [TACTICAL_SAMPLE_HZ_FIELD]);
    if (!parsed.ok) return NextResponse.json({ error: parsed.error }, { status: 400 });
    const body = parsed.value;
    const sampleHz = body[TACTICAL_SAMPLE_HZ_FIELD];
    if (sampleHz !== undefined) {
      if (typeof sampleHz !== 'number' || !Number.isFinite(sampleHz) || sampleHz < 0 || sampleHz > TACTICAL_MAX_SAMPLE_HZ) {
        return NextResponse.json(
          { error: `${TACTICAL_SAMPLE_HZ_FIELD} must be a number between 0 and ${TACTICAL_MAX_SAMPLE_HZ}` },
          { status: 400 },
        );
      }
      init = {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ [TACTICAL_SAMPLE_HZ_FIELD]: sampleHz }),
      };
    }
  }
  return forwardJson(jobUrl(jobId, '/tactical'), TACTICAL_STATUS_KEYS, init);
}

/** GET /api/demos/{jobId}/tactical/status (local) - proxy the analysis lifecycle state. */
export async function localTacticalStatus(jobId: string): Promise<Response> {
  return forwardJson(jobUrl(jobId, '/tactical/status'), TACTICAL_STATUS_KEYS);
}

/** GET tactical round — validate the round segment before building the URL. */
export async function localTacticalRound(jobId: string, round: string): Promise<Response> {
  if (!TACTICAL_ROUND_RE.test(round)) {
    return NextResponse.json({ error: 'invalid round' }, { status: 400 });
  }
  return forwardJson(jobUrl(jobId, `/tactical/rounds/${round}`), TACTICAL_ROUND_KEYS);
}

/** GET tactical aggregate — forward only known filter parameters. */
export async function localTacticalAggregate(jobId: string, search: URLSearchParams): Promise<Response> {
  const filter = new URLSearchParams();
  for (const name of TACTICAL_FILTER_PARAM_NAMES) {
    for (const value of search.getAll(name)) {
      if (value !== '') filter.append(name, value);
    }
  }
  const query = filter.toString();
  const url = jobUrl(jobId, query ? `/tactical/aggregate?${query}` : '/tactical/aggregate');
  return forwardJson(url, TACTICAL_TENDENCIES_KEYS);
}

/** GET tactical positions — stream the zvpos1 blob and keep Range. */
export async function localTacticalPositions(jobId: string, request: Request): Promise<Response> {
  const url = jobUrl(jobId, '/tactical/positions');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  return proxyStream(url, 'application/octet-stream', request);
}

/** A SteamID64 as the dossier route accepts it: 17 digits, Steam's user range. */
const STEAM_ID64_RE = /^7656119\d{10}$/;

/** POST /api/demos/{jobId}/anticheat (local) - queue the CheaterDetect pass. */
export async function localStartAnticheat(jobId: string): Promise<Response> {
  const url = jobUrl(jobId, '/anticheat');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url, { method: 'POST' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  const body = (await res.json()) as { id: string; status: string };
  return NextResponse.json({ jobId: body.id, status: body.status }, { status: 202 });
}

/** GET /api/demos/{jobId}/anticheat (local) - proxy the analysis document. */
export async function localAnticheat(jobId: string): Promise<Response> {
  return forwardJson(jobUrl(jobId, '/anticheat'), ANTICHEAT_DOCUMENT_KEYS);
}

/** GET anticheat dossier — validate SteamID64 before building the upstream URL. */
export async function localAnticheatDossier(jobId: string, steamId: string): Promise<Response> {
  if (!STEAM_ID64_RE.test(steamId)) {
    return NextResponse.json({ error: 'invalid steam id' }, { status: 400 });
  }
  const url = jobUrl(jobId, `/anticheat/dossier/${steamId}`);
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json(await res.json());
}
