// Keep this runtime constant aligned with hlae-tool.json. The unit test compares
// both representations; build scripts consume the JSON manifest directly.
export const PINNED_HLAE_TOOL = {
  version: '2.192.6',
  archiveName: 'hlae_2_192_6.zip',
  url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.6/hlae_2_192_6.zip',
  sha256: 'b3acae70babb536e3b4a34fbbbe4ca8e55a1028068eaaf5fc98817775b72f4fa',
  treeSha256: '8aabba9993a775523802f9b7f42330e90bcd1bb294e84f510e192bbaa9f56e4a',
  kind: 'zip',
  exeRel: 'HLAE.exe',
  timeoutMs: 90_000,
} as const;
