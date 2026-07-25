import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

/**
 * CheaterDetect: screen an already uploaded demo for cheat-suspicion signals
 * and score every player against a measured reference distribution.
 *
 * Like `streams.ts`, this is kept out of the demo→reel `ApiClient` because it
 * drives its own orchestrator surface (`/api/demos/{jobId}/anticheat`) and has
 * nothing to do with the clip pipeline. The screening never changes a demo
 * job's status, so a demo can be screened and clipped independently.
 *
 * Nothing in this module submits a report anywhere. The dossier is material
 * the user carries into the official reporting flow themselves.
 */

/** Lifecycle of a stored analysis, mirroring anticheat.DocumentStatus. */
export const ANTICHEAT_STATUS = {
  running: 'running',
  ready: 'ready',
  failed: 'failed',
} as const;
export type AnticheatStatus = (typeof ANTICHEAT_STATUS)[keyof typeof ANTICHEAT_STATUS];

/** Conservative suspicion bands, mirroring anticheat.Verdict. */
export const ANTICHEAT_VERDICT = {
  insufficient: 'insufficient_data',
  clean: 'clean',
  inconclusive: 'inconclusive',
  anomalous: 'anomalous',
  highlyAnomalous: 'highly_anomalous',
} as const;
export type AnticheatVerdict = (typeof ANTICHEAT_VERDICT)[keyof typeof ANTICHEAT_VERDICT];

/** Spanish labels for each band; the Go side renders the same words. */
export const VERDICT_LABEL: Record<AnticheatVerdict, string> = {
  [ANTICHEAT_VERDICT.highlyAnomalous]: 'muy anómalo',
  [ANTICHEAT_VERDICT.anomalous]: 'anómalo',
  [ANTICHEAT_VERDICT.inconclusive]: 'no concluyente',
  [ANTICHEAT_VERDICT.clean]: 'sin anomalías',
  [ANTICHEAT_VERDICT.insufficient]: 'datos insuficientes',
};

/**
 * Verdicts that warrant human review. Only these unlock the dossier, so the
 * reporting material is never one click away from a clean scoreboard line.
 */
const REVIEWABLE_VERDICTS: ReadonlySet<AnticheatVerdict> = new Set([
  ANTICHEAT_VERDICT.anomalous,
  ANTICHEAT_VERDICT.highlyAnomalous,
]);

export function isReviewable(verdict: AnticheatVerdict): boolean {
  return REVIEWABLE_VERDICTS.has(verdict);
}

export type MetricDirection = 'high' | 'low';

export type MetricBaseline = { mean: number; stddev: number; samples: number };

export type AnticheatMetric = {
  id: string;
  label: string;
  unit: string;
  description: string;
  direction: MetricDirection;
  value: number;
  samples: number;
  baseline: MetricBaseline;
  z: number;
  suspicion: number;
  weight: number;
  applied: boolean;
};

export type AnticheatEvidence = {
  kind: string;
  round: number;
  tick: number;
  victim?: string;
  weapon?: string;
  detail: string;
};

export type AnticheatPlayer = {
  steamid64: string;
  name: string;
  team: 'CT' | 'T' | '';
  rounds: number;
  gun_kills: number;
  score: number;
  confidence: number;
  verdict: AnticheatVerdict;
  metrics: AnticheatMetric[];
  evidence: AnticheatEvidence[];
};

export type AnticheatBaselineHeader = {
  id: string;
  source: string;
  description: string;
  measured: boolean;
};

export type AnticheatReport = {
  schema_version: number;
  source: { demo_path?: string; sha256?: string };
  baseline: AnticheatBaselineHeader;
  match: { map: string; rounds: number; tick_rate: number; sampled_ticks: number };
  players: AnticheatPlayer[];
  limitations: string[];
};

export type AnticheatDocument = {
  schema_version: number;
  job_id: string;
  status: AnticheatStatus;
  started_at: string;
  completed_at?: string;
  failure_reason?: string;
  report?: AnticheatReport;
};

export type ReportChannel = {
  id: string;
  label: string;
  url?: string;
  instructions: string;
  effective: boolean;
};

export type AnticheatDossier = {
  steamid64: string;
  name: string;
  verdict: AnticheatVerdict;
  score: number;
  confidence: number;
  profile_url?: string;
  markdown: string;
  channels: ReportChannel[];
  policy: { summary: string; rules: string[]; rejected: string };
};

/** Raised when the local analysis service cannot be reached. */
export class AnticheatServiceError extends Error {
  readonly code: string;

  constructor(message: string, code: string) {
    super(message);
    this.name = 'AnticheatServiceError';
    this.code = code;
  }
}

/** HTTP 409 from the analysis endpoints: "no screening exists yet". */
export const ANTICHEAT_NOT_STARTED = 'anticheat_not_started' as const;

async function readError(res: Response): Promise<string> {
  try {
    const body = (await res.json()) as { error?: string; code?: string };
    return body.error ?? `HTTP ${res.status}`;
  } catch {
    return `HTTP ${res.status}`;
  }
}

async function request(url: string, init?: RequestInit): Promise<Response> {
  let res: Response;
  try {
    res = await fetch(url, init);
  } catch (err) {
    throw new AnticheatServiceError(`servicio local inaccesible: ${String(err)}`, SERVICE_UNAVAILABLE_CODE);
  }
  if (res.status === 503) {
    throw new AnticheatServiceError('el servicio de análisis local no responde', SERVICE_UNAVAILABLE_CODE);
  }
  return res;
}

/** Queues the CheaterDetect pass for a demo job. Safe to call twice. */
export async function startAnticheat(jobId: string): Promise<AnticheatStatus> {
  const res = await request(`/api/demos/${jobId}/anticheat`, { method: 'POST' });
  if (!res.ok) throw new AnticheatServiceError(await readError(res), `http_${res.status}`);
  const body = (await res.json()) as { status: AnticheatStatus };
  return body.status;
}

/**
 * Fetches the stored analysis, or null when this demo has never been screened
 * (the orchestrator answers 409 for that, which is a state and not an error).
 */
export async function fetchAnticheat(jobId: string): Promise<AnticheatDocument | null> {
  const res = await request(`/api/demos/${jobId}/anticheat`);
  if (res.status === 409) return null;
  if (!res.ok) throw new AnticheatServiceError(await readError(res), `http_${res.status}`);
  return (await res.json()) as AnticheatDocument;
}

/** Fetches one player's evidence pack from a finished analysis. */
export async function fetchDossier(jobId: string, steamId: string): Promise<AnticheatDossier> {
  const res = await request(`/api/demos/${jobId}/anticheat/dossier/${steamId}`);
  if (!res.ok) throw new AnticheatServiceError(await readError(res), `http_${res.status}`);
  return (await res.json()) as AnticheatDossier;
}

/* -------------------------------------------------------------------------- */
/* Proxy-route response whitelists                                            */
/* -------------------------------------------------------------------------- */

/**
 * Top-level keys the analysis-document proxy forwards, mirroring
 * anticheat.Document. `report` is the one nested value that travels whole,
 * because it is the analysis the UI renders and the orchestrator is its only
 * producer.
 */
export const ANTICHEAT_DOCUMENT_KEYS: readonly string[] = [
  'schema_version',
  'job_id',
  'status',
  'started_at',
  'completed_at',
  'failure_reason',
  'report',
];
