// Mirrors internal/httpapi/handlers.go's isDemoHeader: only a CS2 (Source 2)
// demo is accepted. Legacy CS:GO (HL2DEMO) demos are rejected because the
// parser (demoinfocs v5) cannot read them and the bridge refuses them too.
// Compressed uploads (.dem.bz2/.dem.zst) are intentionally rejected here —
// the portal only ever forwards a raw .dem to the bridge/local pipeline.
const DEMO_MAGIC_PREFIXES = ["PBDEMS2"] as const;

export function isDemoHeader(header: Uint8Array): boolean {
  const text = Buffer.from(header).toString("latin1");
  return DEMO_MAGIC_PREFIXES.some((prefix) => text.startsWith(prefix));
}
