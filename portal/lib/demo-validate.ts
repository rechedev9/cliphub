// Mirrors internal/httpapi/handlers.go's isDemoHeader: a CS2 (Source 2) or
// legacy GOTV (Source 1) demo starts with one of these two magic strings.
// Compressed uploads (.dem.bz2/.dem.zst) are intentionally rejected here —
// the portal only ever forwards a raw .dem to the bridge/local pipeline.
const DEMO_MAGIC_PREFIXES = ["PBDEMS2", "HL2DEMO"] as const;

export function isDemoHeader(header: Uint8Array): boolean {
  const text = Buffer.from(header).toString("latin1");
  return DEMO_MAGIC_PREFIXES.some((prefix) => text.startsWith(prefix));
}
