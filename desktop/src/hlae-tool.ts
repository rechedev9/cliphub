// Keep this runtime constant aligned with hlae-tool.json. The unit test compares
// both representations; build scripts consume the JSON manifest directly.
export const PINNED_HLAE_TOOL = {
  version: '2.192.1',
  archiveName: 'hlae_2_192_1.zip',
  url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.1/hlae_2_192_1.zip',
  sha256: '08ae68bb1c42c99bcd441f688d17e24bc52faed27eac07ebea5fc7c98e34b465',
  treeSha256: 'fc5bc770e8492d779fc9599838eab09e781be993de6683872578ddd0660cee54',
  kind: 'zip',
  exeRel: 'HLAE.exe',
  timeoutMs: 90_000,
} as const;
