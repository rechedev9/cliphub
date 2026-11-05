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

export const STORAGE_FILE_KINDS = ['demo', 'studio_demo', 'short', 'full_demo', 'stream', 'editor', 'cloud'] as const;
export type StorageFileKind = (typeof STORAGE_FILE_KINDS)[number];

/** One file on the Almacenamiento screen; `path` is absolute. */
export type StorageFile = {
  path: string;
  sizeBytes: number;
  modifiedAt: string;
  kind: StorageFileKind;
  jobId?: string;
};

export type StorageFiles = {
  dataDir: string;
  /** True while Studio still searches the PC for demos. */
  scanning: boolean;
  scannedAt: string | null;
  demos: StorageFile[];
  studioDemos: StorageFile[];
  videos: StorageFile[];
};

function isStorageFileKind(value: unknown): value is StorageFileKind {
  return STORAGE_FILE_KINDS.some((kind) => kind === value);
}

function projectStorageFileList(body: unknown, key: string): StorageFile[] {
  const raw: unknown = typeof body === 'object' && body !== null ? Reflect.get(body, key) : undefined;
  const files: StorageFile[] = [];
  for (const row of Array.isArray(raw) ? raw : []) {
    if (typeof row !== 'object' || row === null) continue;
    const path: unknown = Reflect.get(row, 'path');
    const kind: unknown = Reflect.get(row, 'kind');
    if (typeof path !== 'string' || path === '' || !isStorageFileKind(kind)) continue;
    const modifiedAt: unknown = Reflect.get(row, 'modified_at');
    const jobId: unknown = Reflect.get(row, 'job_id');
    files.push({
      path,
      sizeBytes: byteCount(row, 'size_bytes'),
      modifiedAt: typeof modifiedAt === 'string' ? modifiedAt : '',
      kind,
      ...(typeof jobId === 'string' && jobId !== '' ? { jobId } : {}),
    });
  }
  return files;
}

/** Orchestrator `GET /api/storage/files` body to the proxy's camelCase whitelist. */
export function projectStorageFiles(body: unknown): StorageFiles {
  const field = (key: string): unknown => (typeof body === 'object' && body !== null ? Reflect.get(body, key) : undefined);
  const dataDir = field('data_dir');
  const scannedAt = field('scanned_at');
  return {
    dataDir: typeof dataDir === 'string' ? dataDir : '',
    scanning: field('scanning') === true,
    scannedAt: typeof scannedAt === 'string' ? scannedAt : null,
    demos: projectStorageFileList(body, 'demos'),
    studioDemos: projectStorageFileList(body, 'studio_demos'),
    videos: projectStorageFileList(body, 'videos'),
  };
}
