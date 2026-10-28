// Keep this runtime constant aligned with hlae-tool.json. The unit test compares
// both representations; build scripts consume the JSON manifest directly.
export const PINNED_HLAE_TOOL = {
  version: '2.192.10',
  archiveName: 'hlae_2_192_10.zip',
  url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.10/hlae_2_192_10.zip',
  sha256: 'cac5bb990f59110c31b159b5d008aa85e6332f04bbaf6b87e82b3228606bc28d',
  treeSha256: '3d5c0437511241eacf79b2baf359e9909b55bb3ceae8df9078cb362153ee8ad7',
  kind: 'zip',
  exeRel: 'HLAE.exe',
  timeoutMs: 90_000,
} as const;
