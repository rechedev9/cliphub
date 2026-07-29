export interface SuccessfulNavigationEvidence {
  url: string;
  httpResponseCode: number;
}

export function isAbortedNavigation(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  const coded = error as Error & { code?: unknown; errno?: unknown };
  return coded.code === 'ERR_ABORTED'
    || coded.code === -3
    || coded.errno === -3
    || error.message.includes('ERR_ABORTED (-3)');
}

export function isSuccessfulInternalReplacement(
  requestedURL: string,
  replacement: SuccessfulNavigationEvidence | null,
  expectedOrigin: string,
): boolean {
  if (replacement === null) return false;
  if (replacement.httpResponseCode < 200 || replacement.httpResponseCode >= 400) return false;
  try {
    const requested = new URL(requestedURL);
    const completed = new URL(replacement.url);
    return completed.origin === new URL(expectedOrigin).origin
      && completed.href !== requested.href;
  } catch {
    return false;
  }
}

// Electron may reject the first loadURL with ERR_ABORTED when the renderer
// replaces it. Accept that only after did-navigate proves a distinct,
// successful main-frame navigation inside the Studio origin.
export function isSupersededInternalNavigation(
  error: unknown,
  requestedURL: string,
  replacement: SuccessfulNavigationEvidence | null,
  expectedOrigin: string,
): boolean {
  return isAbortedNavigation(error)
    && isSuccessfulInternalReplacement(requestedURL, replacement, expectedOrigin);
}
