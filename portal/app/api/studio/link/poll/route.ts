import { badRequest, readJson } from "@/lib/http";
import { isRecord } from "@/lib/job-types";
import { pollLink } from "@/lib/link-store";

export const runtime = "nodejs";

export async function POST(request: Request) {
  const body = await readJson(request);
  if (!isRecord(body)) return badRequest();
  const { linkId, pollToken } = body;
  if (typeof linkId !== "string" || typeof pollToken !== "string") return badRequest();
  return Response.json(await pollLink({ linkId, pollToken, now: Date.now() }));
}
