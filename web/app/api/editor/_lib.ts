import {
  orchestratorUrl,
  callOrchestrator,
  mutationHeaders,
  forwardError,
  serviceUnavailable,
  proxyStream,
  callOrchestratorStreamingUpload,
  UPLOAD_BODY_LIMIT_EXCEEDED,
} from '../demos/_lib';

export {
  orchestratorUrl,
  callOrchestrator,
  mutationHeaders,
  forwardError,
  serviceUnavailable,
  proxyStream,
  callOrchestratorStreamingUpload,
  UPLOAD_BODY_LIMIT_EXCEEDED,
};

const UUID_RE = /^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i;

export function editorAssetUrl(assetId: string, suffix = ''): string | null {
  return UUID_RE.test(assetId) ? `${orchestratorUrl()}/api/editor/assets/${assetId}${suffix}` : null;
}

export function editorProjectUrl(projectId: string, suffix = ''): string | null {
  return UUID_RE.test(projectId) ? `${orchestratorUrl()}/api/editor/projects/${projectId}${suffix}` : null;
}

