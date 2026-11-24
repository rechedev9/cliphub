// Keep this runtime constant aligned with hlae-tool.json. The unit test compares
// both representations; build scripts consume the JSON manifest directly.
export const PINNED_HLAE_TOOL = {
  version: '2.193.2',
  archiveName: 'hlae_2_193_2.zip',
  url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.193.2/hlae_2_193_2.zip',
  sha256: '44eec643c4abbe2965b17a0b5e0a9afce5cb77f437b1907a5dd359c021e0dd86',
  treeSha256: '95611999d06febf85c0a946c5bee484a6691e9db3908db12929f89d1c8fd0fbc',
  kind: 'zip',
  exeRel: 'HLAE.exe',
  timeoutMs: 90_000,
} as const;
