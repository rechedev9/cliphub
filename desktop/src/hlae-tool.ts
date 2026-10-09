// Keep this runtime constant aligned with hlae-tool.json. The unit test compares
// both representations; build scripts consume the JSON manifest directly.
export const PINNED_HLAE_TOOL = {
  version: '2.192.7',
  archiveName: 'hlae_2_192_7.zip',
  url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.7/hlae_2_192_7.zip',
  sha256: 'c0e84832a14170718cb3bfa760144b42beb8dee62b95390e1602f61e1b990e62',
  treeSha256: '5153c917e48b273163f68737a39e27055a612da298f2bb8252e3867df7d17d8f',
  kind: 'zip',
  exeRel: 'HLAE.exe',
  timeoutMs: 90_000,
} as const;
