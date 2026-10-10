import { clientIp, jsonError, notFound } from "@/lib/http";
import { describeLink, linkCodeLimiter } from "@/lib/link-store";
import { resolveSessionUser } from "@/lib/user-context";

export const runtime = "nodejs";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ userCode: string }> },
) {
  const resolved = await resolveSessionUser(request);
  if (!resolved.ok) return resolved.response;
  const now = Date.now();
  if (!linkCodeLimiter.allow(resolved.user.userId, now)) return jsonError(429, "rate_limited");
  const { userCode } = await params;
  const link = await describeLink({ userCode, ip: clientIp(request), now });
  return link ? Response.json(link) : notFound();
}
