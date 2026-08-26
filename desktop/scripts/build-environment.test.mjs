import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  environmentWithoutCodeSigningCredentials,
  environmentWithoutXAIAPIKey,
  faceitAPIKeyFromEnvironment,
  goRuntimeBuildEnvironment,
  releaseBuildEnvironment,
} from './build-environment.mjs';

const desktop = join(dirname(fileURLToPath(import.meta.url)), '..');

test('removes every casing of XAI_API_KEY without mutating the build environment', () => {
  const credentialName = ['XAI', 'API', 'KEY'].join('_');
  const original = {
    KEEP_ME: 'yes',
    [credentialName.toLowerCase()]: 'lowercase',
    [credentialName]: 'uppercase',
    Xai_Api_Key: 'mixed',
  };

  const sanitized = environmentWithoutXAIAPIKey(original);

  assert.deepEqual(sanitized, { KEEP_ME: 'yes' });
  assert.equal(original[credentialName], 'uppercase');
});

test('disables code-signing discovery and strips every signing credential casing', () => {
  const certificateName = ['CSC', 'LINK'].join('_');
  const passwordName = ['CSC', 'KEY', 'PASSWORD'].join('_');
  const windowsCertificateName = ['WIN', 'CSC', 'LINK'].join('_');
  const windowsPasswordName = ['WIN', 'CSC', 'KEY', 'PASSWORD'].join('_');
  const original = {
    [certificateName]: 'certificate-input',
    KEEP_ME: 'yes',
    [passwordName.toLowerCase()]: 'password-input',
    [windowsCertificateName]: 'windows-certificate-input',
    [windowsPasswordName.toLowerCase()]: 'windows-password-input',
  };

  assert.deepEqual(environmentWithoutCodeSigningCredentials(original), {
    CSC_IDENTITY_AUTO_DISCOVERY: 'false',
    KEEP_ME: 'yes',
  });
  assert.equal(original[certificateName], 'certificate-input');
});

test('desktop manifest exposes one credential-free distribution path', () => {
  const manifest = JSON.parse(readFileSync(join(desktop, 'package.json'), 'utf8'));
  const scripts = manifest.scripts;
  const resources = manifest.build.extraResources;

  assert.equal(scripts.assemble, 'node scripts/assemble.mjs');
  assert.equal(scripts.dist, 'node scripts/dist.mjs');
  assert.equal(manifest.build.artifactName, 'ClipHub.Studio.Setup.${version}.${ext}');
  assert.equal(Object.keys(scripts).some((name) => name.includes('team')), false);
  assert.equal(
    resources.some((resource) => /(?:credential|xai-api-key)/i.test(`${resource.from} ${resource.to}`)),
    false,
  );
  assert.equal(
    resources.some((resource) => resource.from === 'build-resources/hlae' && resource.to === 'hlae'),
    true,
  );
});

test('release build environment is an allowlist, not a credential denylist', () => {
  const original = {
    Path: 'C:\\tools',
    SystemRoot: 'C:\\Windows',
    TEMP: 'C:\\temp',
    ELECTRON_CACHE: 'C:\\cache\\electron',
    FIRECRAWL_API_KEY: 'fixture',
    CLIPHUB_PROXY_MUTATION_CAPABILITY: 'fixture',
    GH_TOKEN: 'fixture',
    GROQ_API_KEY: 'fixture',
    HTTPS_PROXY: 'http://proxy.invalid',
    NPM_TOKEN: 'fixture',
    ZV_MUTATION_TOKEN: 'fixture',
  };

  assert.deepEqual(releaseBuildEnvironment(original), {
    Path: 'C:\\tools',
    SystemRoot: 'C:\\Windows',
    TEMP: 'C:\\temp',
    ELECTRON_CACHE: 'C:\\cache\\electron',
    CSC_IDENTITY_AUTO_DISCOVERY: 'false',
  });
  assert.equal(original.GH_TOKEN, 'fixture');
});

test('FACEIT_API_KEY is not an electron-builder allowlist member', () => {
  const original = {
    Path: 'C:\\tools',
    FACEIT_API_KEY: 'faceit-installer-secret',
    FIRECRAWL_API_KEY: 'fixture',
  };
  const sanitized = releaseBuildEnvironment(original);
  assert.equal(Object.keys(sanitized).some((name) => name.toUpperCase() === 'FACEIT_API_KEY'), false);
});

test('go runtime rebuild receives FACEIT_API_KEY for ldflags embed', () => {
  const cases = [
    { FACEIT_API_KEY: 'faceit-installer-secret', Path: 'C:\\tools' },
    { faceit_api_key: 'faceit-lower-secret', Path: 'C:\\tools' },
    { Path: 'C:\\tools' },
  ];
  const expected = [
    'faceit-installer-secret',
    'faceit-lower-secret',
    '',
  ];
  for (const [i, original] of cases.entries()) {
    assert.equal(faceitAPIKeyFromEnvironment(original), expected[i]);
    const goEnv = goRuntimeBuildEnvironment(original);
    if (expected[i] === '') {
      assert.equal(Object.keys(goEnv).some((name) => name.toUpperCase() === 'FACEIT_API_KEY'), false);
    } else {
      assert.equal(goEnv.FACEIT_API_KEY, expected[i]);
    }
    assert.equal(goEnv.CSC_IDENTITY_AUTO_DISCOVERY, 'false');
    assert.equal(goEnv.Path, 'C:\\tools');
  }
});
