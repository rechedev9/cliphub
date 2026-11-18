import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import { orchestratorUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

/** PUT /api/demos/storage/capture-retention {keep}: how many rendered Full Demos keep their captures. */
export async function PUT(request: Request): Promise<Response> {
  const incoming = await readBoundedText(request, 1024);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const response = await callOrchestrator(`${orchestratorUrl()}/api/storage/capture-retention`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: incoming.text,
  });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body);
}
