import { createHash, timingSafeEqual } from "node:crypto";

// Digests both sides to a fixed-size buffer before comparing: Node's
// timingSafeEqual throws on a length mismatch, and a raw length check would
// itself leak timing information about the secret's length.
function constantTimeEqual(a: string, b: string): boolean {
  const digestA = createHash("sha256").update(a).digest();
  const digestB = createHash("sha256").update(b).digest();
  return timingSafeEqual(digestA, digestB);
}

// The bridge is one static shared secret for one permanent machine (the
// owner's own PC) — not per-device credentials. Never accept it from a
// browser-facing route.
export function isBridgeAuthorized(request: Request): boolean {
  const expected = process.env.BRIDGE_SHARED_SECRET;
  if (!expected) return false;

  const header = request.headers.get("authorization") ?? "";
  const prefix = "Bearer ";
  if (!header.startsWith(prefix)) return false;

  const provided = header.slice(prefix.length);
  return constantTimeEqual(provided, expected);
}
