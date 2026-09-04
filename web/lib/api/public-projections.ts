// Browser-facing projections of orchestrator entities. The Go structs carry
// local filesystem facts (`source_path`, `media_key`) that the worker needs
// and the Studio never does; the proxy routes stop them here so a TS type
// omitting a field is also a wire guarantee.

const STREAM_JOB_PUBLIC_KEYS = [
  'id',
  'status',
  'failure_reason',
  'failure_code',
  'source_url',
  'title',
  'probe',
  'edit_plan',
  'clip_count',
  'created_at',
  'updated_at',
] as const;

const EDITOR_ASSET_PUBLIC_KEYS = [
  'id',
  'sha256',
  'file_name',
  'origin',
  'origin_job_id',
  'origin_variant',
  'origin_name',
  'probe',
  'created_at',
] as const;

function pick(raw: unknown, keys: readonly string[]): Record<string, unknown> {
  if (!raw || typeof raw !== 'object') return {};
  const source = raw as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const key of keys) {
    if (source[key] !== undefined) out[key] = source[key];
  }
  return out;
}

/** `streamclips.Job` without `source_path` / `source_sha256`. */
export function publicStreamJob(raw: unknown): Record<string, unknown> {
  return pick(raw, STREAM_JOB_PUBLIC_KEYS);
}

/** `mediaassets.Asset` without `media_key`; media streams through `/assets/{id}/media`. */
export function publicEditorAsset(raw: unknown): Record<string, unknown> {
  return pick(raw, EDITOR_ASSET_PUBLIC_KEYS);
}
