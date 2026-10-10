import { badRequest, clientIp, jsonError, notFound, readJson } from "@/lib/http";
import { isRecord } from "@/lib/job-types";
import { decideLink, linkCodeLimiter } from "@/lib/link-store";
import { resolveSessionUser } from "@/lib/user-context";

export const runtime = "nodejs";

export async function POST(request: Request) {
  const resolved = await resolveSessionUser(request);
  if (!resolved.ok) return resolved.response;
  const { userId } = resolved.user;
  const now = Date.now();
  if (!linkCodeLimiter.allow(userId, now)) return jsonError(429, "rate_limited");
  const body = await readJson(request);
  if (!isRecord(body)) return badRequest();
  const { userCode, approve, typedCode } = body;
  if (typeof userCode !== "string" || typeof approve !== "boolean") return badRequest();
  const decision = await decideLink({
    userCode,
    approve,
    userId,
    ip: clientIp(request),
    typedCode: typeof typedCode === "string" ? typedCode : null,
    now,
  });
  if (decision === "code_confirmation_required") return jsonError(422, decision);
  return decision === "ok" ? Response.json({ ok: true }) : notFound();
}
