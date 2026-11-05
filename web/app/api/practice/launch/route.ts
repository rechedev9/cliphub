import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../demos/_lib';
import { localAPIRequestError } from '@/lib/api/local-request-guard';
import { parseControlJSONObject, readBoundedText } from '@/lib/api/bounded-request-body';
import { INVALID_PRACTICE_CODE, parsePracticeLaunchInput, projectPracticeLaunch } from '@/lib/api/practice';

export const runtime = 'nodejs';

/** POST /api/practice/launch: write the CS2 practice cfg and open CS2 on it. */
export async function POST(request: Request): Promise<Response> {
  const localError = await localAPIRequestError(request.headers, request.method);
  if (localError !== undefined) return NextResponse.json({ error: localError }, { status: 403 });

  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const object = parseControlJSONObject(incoming.text, ['map', 'spots']);
  if (!object.ok) return NextResponse.json({ error: object.error, code: INVALID_PRACTICE_CODE }, { status: 400 });
  const input = parsePracticeLaunchInput(object.value);
  if (!input.ok) return NextResponse.json({ error: input.error, code: INVALID_PRACTICE_CODE }, { status: 400 });

  const res = await callOrchestrator(`${orchestratorUrl()}/api/practice/launch`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(input.value),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);

  const result = projectPracticeLaunch(await res.json().catch(() => null));
  if (result === null) return NextResponse.json({ error: 'practice launch response is invalid' }, { status: 502 });
  return NextResponse.json(result, { headers: { 'cache-control': 'no-store' } });
}
