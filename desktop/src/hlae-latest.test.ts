import test from 'node:test';
import assert from 'node:assert/strict';
import { compareHLAEVersions, parseLatestHLAERelease } from './hlae-latest.ts';

const DIGEST = 'b3acae70babb536e3b4a34fbbbe4ca8e55a1028068eaaf5fc98817775b72f4fa';

// Shape of GET /repos/advancedfx/advancedfx/releases/latest for v2.192.6.
function release(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    tag_name: 'v2.192.6',
    draft: false,
    prerelease: false,
    assets: [
      {
        name: 'hlae_2_192_6.zip',
        digest: `sha256:${DIGEST}`,
        browser_download_url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.6/hlae_2_192_6.zip',
      },
      { name: 'hlae_2_192_6.zip.asc', digest: 'sha256:00', browser_download_url: 'https://example.invalid' },
      { name: 'HLAE_Setup.exe', digest: `sha256:${DIGEST}`, browser_download_url: 'https://example.invalid' },
    ],
    ...overrides,
  };
}

test('reads the portable zip and its GitHub digest from the latest release', () => {
  assert.deepEqual(parseLatestHLAERelease(release()), {
    version: '2.192.6',
    archiveName: 'hlae_2_192_6.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.6/hlae_2_192_6.zip',
    sha256: DIGEST,
  });
});

test('refuses releases Studio cannot verify or should not run', () => {
  const zip = (asset: Record<string, unknown>) => release({ assets: [{ ...(release().assets as object[])[0], ...asset }] });
  for (const [label, value] of [
    ['prerelease', release({ prerelease: true })],
    ['draft', release({ draft: true })],
    ['non-numeric tag', release({ tag_name: 'v2.193.0-beta' })],
    ['missing digest', zip({ digest: undefined })],
    ['non-sha256 digest', zip({ digest: 'md5:abc' })],
    ['foreign download host', zip({ browser_download_url: 'https://evil.example/hlae_2_192_6.zip' })],
    ['foreign repository', zip({ browser_download_url: 'https://github.com/someone/advancedfx/releases/download/v2.192.6/hlae_2_192_6.zip' })],
    ['no portable zip', release({ assets: [] })],
    ['not an object', 'rate limited'],
  ] as const) {
    assert.equal(parseLatestHLAERelease(value), null, label);
  }
});

test('orders HLAE versions numerically', () => {
  assert.ok(compareHLAEVersions('2.192.10', '2.192.9') > 0);
  assert.ok(compareHLAEVersions('2.193.0', '2.192.6') > 0);
  assert.equal(compareHLAEVersions('2.192.6', '2.192.6'), 0);
  assert.ok(compareHLAEVersions('2.192.5', '2.192.6') < 0);
  assert.ok(Number.isNaN(compareHLAEVersions('2.192.2-cliphub.1', '2.192.6')));
});
