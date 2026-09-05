export interface StudioPermissionRequest {
  permission: string;
  expectedOrigin: string | null;
  expectedWebContentsID: number | null;
  requestingWebContentsID: number | null;
  requestingOrigin: string;
  requestingURL?: string;
  isMainFrame: boolean;
  windowFocused: boolean;
}

function matchesOrigin(value: string, expectedOrigin: string): boolean {
  try {
    return new URL(value).origin === expectedOrigin;
  } catch {
    return false;
  }
}

export function isAllowedStudioPermission(request: StudioPermissionRequest): boolean {
  if (
    request.permission !== 'fullscreen' ||
    request.expectedOrigin === null ||
    request.expectedWebContentsID === null ||
    request.requestingWebContentsID !== request.expectedWebContentsID ||
    !request.isMainFrame ||
    !request.windowFocused ||
    !matchesOrigin(request.requestingOrigin, request.expectedOrigin)
  ) {
    return false;
  }

  return (
    request.requestingURL === undefined ||
    matchesOrigin(request.requestingURL, request.expectedOrigin)
  );
}
