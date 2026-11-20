// Keep this runtime constant aligned with hlae-tool.json. The unit test compares
// both representations; build scripts consume the JSON manifest directly.
export const PINNED_HLAE_TOOL = {
  version: '2.193.1',
  archiveName: 'hlae_2_193_1.zip',
  url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.193.1/hlae_2_193_1.zip',
  sha256: '1d07333fa34aca23c9f994d77010591e60bf93e176debdf511a854fd6902c99e',
  treeSha256: '3cc4faaadb7c23a83c175b96f43db54835889960d609482cb478907aa2ccfd3a',
  kind: 'zip',
  exeRel: 'HLAE.exe',
  timeoutMs: 90_000,
} as const;
