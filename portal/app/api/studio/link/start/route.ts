import { badRequest, clientIp, jsonError, readJson } from "@/lib/http";
import { isRecord } from "@/lib/job-types";
import { linkStartLimiter, parseDeviceName, startLink } from "@/lib/link-store";
import { portalOrigin } from "@/lib/same-origin";

export const runtime = "nodejs";

// No authentication: this is how a Studio that has no account yet asks for a code.
export async function POST(request: Request) {
  const now = Date.now();
  const ip = clientIp(request);
  if (!linkStartLimiter.allow(ip, now)) return jsonError(429, "rate_limited");
  const body = await readJson(request);
  const deviceName = parseDeviceName(isRecord(body) ? body.deviceName : undefined);
  const origin = portalOrigin(request);
  if (deviceName === null || origin === null) return badRequest();
  return Response.json(await startLink({ deviceName, ip, origin, now }));
}
