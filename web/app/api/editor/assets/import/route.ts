import { NextResponse } from 'next/server';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { parseControlJSONObject, readBoundedText } from '@/lib/api/bounded-request-body';
import { orchestratorUrl, callOrchestrator, mutationHeaders, forwardError, serviceUnavailable } from '../../_lib';
import { publicEditorAsset } from '@/lib/api/public-projections';

export const runtime = 'nodejs';

const JOB_ID_RE = /^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;
const IMPORT_BODY_KEYS = ['source', 'job_id', 'variant', 'name'] as const;
const IMPORT_SOURCE = {
  demo: 'demo',
  stream: 'stream',
} as const;

type ImportSource = (typeof IMPORT_SOURCE)[keyof typeof IMPORT_SOURCE];

function isImportSource(value: unknown): value is ImportSource {
  return value === IMPORT_SOURCE.demo || value === IMPORT_SOURCE.stream;
}

export async function POST(request: Request): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });

  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const parsed = parseControlJSONObject(incoming.text, IMPORT_BODY_KEYS);
  if (!parsed.ok) return NextResponse.json({ error: parsed.error }, { status: 400 });

  const source = parsed.value.source;
  const jobId = parsed.value.job_id;
  if (!isImportSource(source)) {
    return NextResponse.json({ error: 'source must be demo or stream' }, { status: 400 });
  }
  if (typeof jobId !== 'string' || !JOB_ID_RE.test(jobId)) {
    return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  }

  const payload: { source: ImportSource; job_id: string; variant?: string; name?: string } = {
    source,
    job_id: jobId,
  };
  if (typeof parsed.value.variant === 'string' && parsed.value.variant !== '') {
    payload.variant = parsed.value.variant;
  }
  if (typeof parsed.value.name === 'string' && parsed.value.name !== '') {
    payload.name = parsed.value.name;
  }

  const res = await callOrchestrator(`${orchestratorUrl()}/api/editor/assets/import`, {
    method: 'POST',
    headers: { ...mutationHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json(publicEditorAsset(await res.json()), { status: res.status });
}
