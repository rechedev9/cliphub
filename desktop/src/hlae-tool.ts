// Keep this runtime constant aligned with hlae-tool.json. The unit test compares
// both representations; build scripts consume the JSON manifest directly.
export const PINNED_HLAE_TOOL = {
  version: '2.192.3',
  archiveName: 'hlae_2_192_3.zip',
  url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.3/hlae_2_192_3.zip',
  sha256: '680b90dd5bed3e5b17de0945cc62e147696817ecdcf5e80b5de3c3cb77a84ab1',
  treeSha256: 'acd15eb0580674d49c2233448d3734e6f6c74f1c55c9227094952ad4c6f4ddfe',
  kind: 'zip',
  exeRel: 'HLAE.exe',
  timeoutMs: 90_000,
} as const;
