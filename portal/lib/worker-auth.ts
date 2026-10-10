import { and, eq, isNull } from "drizzle-orm";

import { db } from "../db/client.ts";
import { workers } from "../db/schema.ts";
import { badRequest, unauthorized } from "./http.ts";
import { bearerToken, hashToken, isWorkerToken } from "./tokens.ts";
import { parseAttemptHeader } from "./worker-protocol.ts";

export type AuthenticatedWorker = typeof workers.$inferSelect;

// The worker behind the Bearer token, or null for an unknown or revoked one.
// Never accepts cookies, and does not touch lastSeenAt: only the heartbeat does.
export async function authenticateWorker(
  request: Request,
): Promise<AuthenticatedWorker | null> {
  const token = bearerToken(request);
  if (token === null || !isWorkerToken(token)) return null;
  const [worker] = await db
    .select()
    .from(workers)
    .where(and(eq(workers.tokenHash, hashToken(token)), isNull(workers.revokedAt)));
  return worker ?? null;
}

export type WorkerJobCall =
  | { ok: true; worker: AuthenticatedWorker; attempt: number }
  | { ok: false; response: Response };

// For every route under /api/worker/jobs/:id: the worker and the attempt it is working on.
export async function authenticateWorkerJobCall(request: Request): Promise<WorkerJobCall> {
  const worker = await authenticateWorker(request);
  if (!worker) return { ok: false, response: unauthorized() };
  const attempt = parseAttemptHeader(request);
  if (attempt === null) return { ok: false, response: badRequest() };
  return { ok: true, worker, attempt };
}
