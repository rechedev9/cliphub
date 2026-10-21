/** Disk use of one job folder; `freeable` when a ready long video render covers its `captureBytes`. */
export type JobStorage = {
  jobId: string;
  captureBytes: number;
  renderBytes: number;
  freeable: boolean;
};

export type StorageUsage = {
  jobs: JobStorage[];
  totalBytes: number;
};

function byteCount(body: unknown, key: string): number {
  if (typeof body !== 'object' || body === null) return 0;
  const value: unknown = Reflect.get(body, key);
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : 0;
}

/** Orchestrator `GET /api/storage` body to the proxy's camelCase whitelist; malformed rows drop. */
export function projectStorageUsage(body: unknown): StorageUsage {
  const raw: unknown = typeof body === 'object' && body !== null ? Reflect.get(body, 'jobs') : undefined;
  const jobs: JobStorage[] = [];
  for (const job of Array.isArray(raw) ? raw : []) {
    const id: unknown = typeof job === 'object' && job !== null ? Reflect.get(job, 'id') : undefined;
    if (typeof id !== 'string' || id === '') continue;
    jobs.push({
      jobId: id,
      captureBytes: byteCount(job, 'capture_bytes'),
      renderBytes: byteCount(job, 'render_bytes'),
      freeable: Reflect.get(job, 'freeable') === true,
    });
  }
  return { jobs, totalBytes: byteCount(body, 'total_bytes') };
}

/** Orchestrator `DELETE /api/jobs/{id}/captures` body to the bytes it freed. */
export function projectFreedBytes(body: unknown): { freedBytes: number } {
  return { freedBytes: byteCount(body, 'freed_bytes') };
}
