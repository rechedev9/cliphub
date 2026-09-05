import { NextResponse } from 'next/server';
import { jobUrl, callOrchestrator, forwardError, serviceUnavailable } from '../../../../_lib';

export const runtime = 'nodejs';

export async function GET(_request: Request, { params }: { params: Promise<{ jobId: string; planId: string }> }): Promise<Response> {
  const { jobId, planId } = await params;
  if (!/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(planId)) return NextResponse.json({ code: 'invalid_plan_id', error: 'invalid plan id' }, { status: 400 });
  const url = jobUrl(jobId, `/full-demo/plans/${planId}`);
  if (!url) return NextResponse.json({ code: 'invalid_job_id', error: 'invalid job id' }, { status: 400 });
  const response = await callOrchestrator(url, { cache: 'no-store' });
  if (response === null) return serviceUnavailable();
  if (!response.ok) return forwardError(response);
  const body: unknown = await response.json();
  return NextResponse.json(body);
}
