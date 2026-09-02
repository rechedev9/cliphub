import { randomBytes } from 'node:crypto';

const CAPABILITY_BYTES = 32;
const CAPABILITY_PATTERN = /^[a-f0-9]{64}$/;

export const PROXY_CAPABILITY_COOKIE = 'cliphub_ui_capability';

export interface BootSecurityCapabilities {
  mutationToken: string;
  proxyMutationCapability: string;
}

export interface CookieStore {
  set(details: {
    httpOnly: boolean;
    name: string;
    path: string;
    sameSite: 'strict';
    secure: boolean;
    url: string;
    value: string;
  }): Promise<void>;
}

type CapabilityGenerator = () => string;

/** Creates independent, ephemeral capabilities for API auth and the renderer proxy. */
export function createBootSecurityCapabilities(
  generate: CapabilityGenerator = generateCapability,
): BootSecurityCapabilities {
  const capabilities = {
    mutationToken: generate(),
    proxyMutationCapability: generate(),
  };
  const values = Object.values(capabilities);
  if (values.some((value) => !CAPABILITY_PATTERN.test(value))) {
    throw new Error('boot capability generator returned an invalid value');
  }
  if (new Set(values).size !== values.length) {
    throw new Error('boot security capabilities must be distinct');
  }
  return capabilities;
}

export function orchestratorSecurityEnvironment(
  capabilities: BootSecurityCapabilities,
): NodeJS.ProcessEnv {
  return {
    CLIPHUB_PROXY_BOOTSTRAP_CAPABILITY: undefined,
    CLIPHUB_PROXY_MUTATION_CAPABILITY: undefined,
    ORCHESTRATOR_TOKEN: undefined,
    ZV_MUTATION_TOKEN: capabilities.mutationToken,
    ZV_UI_CAPABILITY: capabilities.proxyMutationCapability,
    ZV_UI_BOOTSTRAP_CAPABILITY: undefined,
  };
}

/** Seeds the HttpOnly capability before the first renderer navigation to the local web origin. */
export async function installProxyCapabilityCookie(
  cookies: CookieStore,
  webOrigin: string,
  capability: string,
): Promise<void> {
  if (!CAPABILITY_PATTERN.test(capability)) throw new Error('invalid proxy mutation capability');
  const origin = new URL(webOrigin);
  if (origin.protocol !== 'http:' || origin.hostname !== '127.0.0.1' || origin.pathname !== '/') {
    throw new Error('proxy capability cookie requires the explicit HTTP loopback origin');
  }
  await cookies.set({
    httpOnly: true,
    name: PROXY_CAPABILITY_COOKIE,
    path: '/',
    sameSite: 'strict',
    secure: false,
    url: origin.origin,
    value: capability,
  });
}

function generateCapability(): string {
  return randomBytes(CAPABILITY_BYTES).toString('hex');
}
