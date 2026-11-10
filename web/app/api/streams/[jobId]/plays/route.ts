import { NextResponse } from 'next/server';
import { parseControlJSONObject, readBoundedText } from '@/lib/api/bounded-request-body';
import { streamJobUrl, mutationHeaders, forwardError, callOrchestrator, serviceUnavailable } from '../../_lib';

export const runtime = 'nodejs';

/** GET /api/streams/{jobId}/plays: the state and result of the play scan. */
export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/plays');
  if (!url) return NextResponse.json({ error: 'Identificador de vídeo no válido.' }, { status: 400 });

  const res = await callOrchestrator(url);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown);
}

/** POST /api/streams/{jobId}/plays: start scanning the source for plays; the body may switch voice peaks and set their sensitivity. */
export async function POST(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/plays');
  if (!url) return NextResponse.json({ error: 'Identificador de vídeo no válido.' }, { status: 400 });

  const incoming = await readBoundedText(request);
  if (!incoming.ok) return NextResponse.json({ error: incoming.error }, { status: incoming.status });
  const parsed = parseControlJSONObject(incoming.text, ['voice', 'voice_sensitivity']);
  if (!parsed.ok) return NextResponse.json({ error: 'Los ajustes de la búsqueda no son válidos.' }, { status: 400 });
  const { voice, voice_sensitivity: sensitivity } = parsed.value;
  if (voice !== undefined && typeof voice !== 'boolean') return NextResponse.json({ error: 'El ajuste de picos de voz debe ser sí o no.' }, { status: 400 });
  if (sensitivity !== undefined && (typeof sensitivity !== 'number' || !Number.isInteger(sensitivity))) {
    return NextResponse.json({ error: 'La sensibilidad debe ser un número entero.' }, { status: 400 });
  }
  const res = await callOrchestrator(url, {
    method: 'POST',
    headers: { ...mutationHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(parsed.value),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  return NextResponse.json((await res.json()) as unknown, { status: res.status });
}
