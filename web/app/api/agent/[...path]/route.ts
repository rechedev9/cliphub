import { forwardControlPlane } from '@/app/api/_control-plane';

interface RouteContext {
  params: Promise<{ path: string[] }>;
}

async function forward(request: Request, context: RouteContext): Promise<Response> {
  const { path } = await context.params;
  return forwardControlPlane(request, 'agent', path);
}

export const GET = forward;
export const POST = forward;
