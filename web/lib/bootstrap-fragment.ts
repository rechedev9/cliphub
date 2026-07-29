const LOCAL_CAPABILITY_PATTERN = /^[0-9a-f]{64}$/;

/**
 * Returns the one-launch capability carried in a URL fragment. Fragments are
 * browser-local: they are not sent in the HTTP request or Referer header.
 */
export function bootstrapCapabilityFromHash(hash: string): string | null {
  const capability = hash.startsWith('#') ? hash.slice(1) : hash;
  return LOCAL_CAPABILITY_PATTERN.test(capability) ? capability : null;
}
