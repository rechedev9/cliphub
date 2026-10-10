import { isCloudKind, type CloudKind } from "./job-types.ts";

export interface CloudConfig {
  // Kinds the portal accepts today; others answer kind_unavailable.
  kinds: CloudKind[];
  maxQueuedJobs: number;
  maxDemoBytes: number;
  // Bytes of distinct demos one user may have stored at a time.
  maxDemoBytesPerUser: number;
  maxArtifactBytes: Record<CloudKind, number>;
  artifactPartBytes: number;
  // Free space below which no new job or demo upload is accepted.
  minFreeBytes: number;
  // The lower floor for claims: finishing jobs is what frees demos.
  minFreeBytesClaim: number;
  // Jobs one worker may hold in uploading before it is refused new claims.
  maxUploadBacklog: number;
  artifactRetentionDays: number;
  failedDemoRetentionHours: number;
  leaseSeconds: number;
  uploadDir: string;
}

const MIB = 1024 * 1024;
const GIB = 1024 * MIB;

function positive(raw: string | undefined, fallback: number): number {
  const value = Number(raw);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function parseKinds(raw: string | undefined): CloudKind[] {
  const kinds = (raw ?? "short")
    .split(",")
    .map((kind) => kind.trim())
    .filter(isCloudKind);
  return [...new Set(kinds)];
}

export function loadCloudConfig(
  env: Record<string, string | undefined> = process.env,
): CloudConfig {
  return {
    kinds: parseKinds(env.CLOUD_KINDS),
    maxQueuedJobs: positive(env.MAX_QUEUED_JOBS, 80),
    maxDemoBytes: positive(env.MAX_DEMO_BYTES, 700 * MIB),
    maxDemoBytesPerUser: positive(env.MAX_DEMO_BYTES_PER_USER, 3 * GIB),
    maxArtifactBytes: {
      short: positive(env.MAX_ARTIFACT_BYTES, GIB),
      full_demo: positive(env.MAX_FULL_DEMO_ARTIFACT_BYTES, 8 * GIB),
    },
    artifactPartBytes: positive(env.ARTIFACT_PART_BYTES, 32 * MIB),
    minFreeBytes: positive(env.MIN_FREE_BYTES, 10 * GIB),
    minFreeBytesClaim: positive(env.MIN_FREE_BYTES_CLAIM, 2 * GIB),
    maxUploadBacklog: positive(env.MAX_UPLOAD_BACKLOG, 3),
    artifactRetentionDays: positive(env.ARTIFACT_RETENTION_DAYS, 7),
    failedDemoRetentionHours: positive(env.FAILED_DEMO_RETENTION_HOURS, 48),
    leaseSeconds: positive(env.LEASE_SECONDS, 90),
    uploadDir: env.UPLOAD_DIR ?? "./data/uploads",
  };
}
