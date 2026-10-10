// Mirrors internal/httpapi/handlers.go's isDemoHeader: only a CS2 (Source 2)
// demo is accepted. Legacy CS:GO (HL2DEMO) demos are rejected because the
// parser (demoinfocs v5) cannot read them and the worker refuses them too.
// Compressed uploads (.dem.bz2/.dem.zst) are intentionally rejected here:
// the portal only ever forwards a raw .dem to the worker.
const DEMO_MAGIC_PREFIXES = ["PBDEMS2"] as const;
const CSGO_MAGIC_PREFIX = "HL2DEMO";

export function isDemoHeader(header: Uint8Array): boolean {
  const text = Buffer.from(header).toString("latin1");
  return DEMO_MAGIC_PREFIXES.some((prefix) => text.startsWith(prefix));
}

// Lets the upload route tell a CS:GO demo apart from a file that is no demo at all.
export function isCsgoDemoHeader(header: Uint8Array): boolean {
  return Buffer.from(header).toString("latin1").startsWith(CSGO_MAGIC_PREFIX);
}
