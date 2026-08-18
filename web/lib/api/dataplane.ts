/** Same-origin /api/demos/* addressing. No token travels to the browser. */

/** Local proxy transport; headers stay empty so no auth crosses the boundary. */
export type DataPlane = {
  headers: Record<string, string>;
  scanUrl: string;
  /** Multipart field name for the .dem upload. */
  scanField: 'demo';
  /** Multipart field name for the optional bulk-series id carried on a scan. */
  scanSeriesField: 'series_id';
  /** Reads the job id out of a scan response. */
  scanJobId(body: unknown): string;
  jobStatusUrl(jobId: string): string;
  /** DELETE target for removing a demo job (match) and its server-side artifacts. */
  jobDeleteUrl(jobId: string): string;
  rosterUrl(jobId: string): string;
  /** The recent-jobs listing that Partidas rediscovers uploads/series from. */
  jobsUrl: string;
  seriesUrl(seriesId: string): string;
  parseUrl(jobId: string): string;
  /** Parse request body. */
  parseBody(steamId: string): Record<string, string>;
  planUrl(jobId: string): string;
  recapPlanUrl(jobId: string): string;
  recordUrl(jobId: string): string;
  renderUrl(jobId: string, variant: string): string;
  renderReviewUrl(jobId: string, variant: string): string;
  videoUrl(jobId: string, variant: string, name: string): string;
  publishAssistantUrl(jobId: string, variant: string, name: string, days?: number): string;
  coverUrl(jobId: string, variant: string, name: string): string;
  capabilitiesUrl: string;
};

function str(body: unknown, key: string): string {
  const v = (body as Record<string, unknown> | null)?.[key];
  return typeof v === 'string' ? v : '';
}

/** Same-origin /api/demos/* paths used by RealApiClient. */
export function dataPlane(): DataPlane {
  return {
    headers: {},
    scanUrl: '/api/demos/scan',
    scanField: 'demo',
    scanSeriesField: 'series_id',
    scanJobId: (body) => str(body, 'jobId'),
    jobStatusUrl: (jobId) => `/api/demos/${jobId}/status`,
    jobDeleteUrl: (jobId) => `/api/demos/${jobId}`,
    rosterUrl: (jobId) => `/api/demos/${jobId}/roster`,
    jobsUrl: '/api/demos/jobs',
    seriesUrl: (seriesId) => `/api/demos/series/${seriesId}`,
    parseUrl: (jobId) => `/api/demos/${jobId}/parse`,
    parseBody: (steamId) => ({ steamId }),
    planUrl: (jobId) => `/api/demos/${jobId}/plan`,
    recapPlanUrl: (jobId) => `/api/demos/${jobId}/recap-plan`,
    recordUrl: (jobId) => `/api/demos/${jobId}/record`,
    renderUrl: (jobId, variant) => `/api/demos/${jobId}/renders/${variant}`,
    renderReviewUrl: (jobId, variant) => `/api/demos/${jobId}/renders/${variant}/review`,
    videoUrl: (jobId, variant, name) => `/api/demos/${jobId}/renders/${variant}/videos/${name}`,
    publishAssistantUrl: (jobId, variant, name, days = 7) =>
      `/api/demos/${jobId}/renders/${variant}/videos/${name}/publish-assistant?days=${days}`,
    coverUrl: (jobId, variant, name) => `/api/demos/${jobId}/renders/${variant}/covers/${name}`,
    capabilitiesUrl: '/api/capabilities',
  };
}
