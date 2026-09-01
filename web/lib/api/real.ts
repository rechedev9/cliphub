import type { ApiClient, VideoReviewResolution } from './client';
import {
  applyMusicChoice,
  musicChoicesEqual,
  titleWithMusicSuffix,
  type MusicChoice,
} from './reel-music.ts';
import type { Match, Play, Song, Video, FeedItem, RenderMode, DemoPlayer, Preset, EditConfig, CaptureReadiness, CaptureTool, CaptureStatus, RosterMatch, CaptureProgress, SeriesDemo } from './types';
import { SERVICE_UNAVAILABLE_CODE, PLAN_READY_STATUSES } from './types';
import { MockApiClient } from './mock';
import { planToMatch, planToPlays, type KillPlan } from './map';
import { canHaveRenderState, decideReelReconcile, retryReelAction, shouldReconcileVideoStatus, unrecoverableJobGoneView, viewForJobGone, viewForRecordAdmission, type ReelAction, type ReelView, type RenderStatus } from './reel-reconcile';
import { loadReelIntents, saveReelIntents, DEFAULT_VARIANT, DEFAULT_EDIT_CONFIG, type ReelIntent } from './reel-store';
import { buildEditRequest, editConfigsEqual } from './edit-request';
import { reelIdentity, shouldReuseReelIntent } from './reel-identity';
import {
  applyEffectiveRenderMusic,
  clearVideoArtifactUrls,
  hydrateVideoFromIntent,
  parseEffectiveEditConfig,
  parseEffectiveRenderMusic,
  type EffectiveRenderMusic,
} from './render-hydration';
import { dataPlane, type DataPlane } from './dataplane';
import { parsePublishAssistant, type PublishAssistant } from './publish-assistant';
import {
  ROSTER_READY,
  listableJobs,
  planReadyJobs,
  summarizeSeries,
  jobToMatch,
  type IndexedJob,
  type SeriesSummary,
} from './jobs-index';
import { reconcileReels } from './reconcile-batch';
import { parseCaptureProgress } from '@/lib/capture-progress';
import { playsSelectionLabel } from '@/lib/format';
import { constrainEditConfig, isLandscapeRecap } from '@/lib/reel-brief';

/** Server roster row as returned by /api/demos/{jobId}/roster (steamid64). */
type RosterPlayer = {
  steamid64: string;
  name: string;
  team: 'CT' | 'T' | '';
  kills: number;
  deaths: number;
  assists: number;
  headshots?: number;
  mvps?: number;
  rounds?: number;
  adr?: number;
  hs_pct?: number;
  kast?: number;
  rating?: number;
  rounds_2k?: number;
  rounds_3k?: number;
  rounds_4k?: number;
  rounds_5k?: number;
};

/** Server match summary as returned by /api/demos/{jobId}/roster (snake_case). */
type RosterMatchResponse = {
  map: string;
  score_ct: number;
  score_t: number;
  rounds: number;
};

/** The full roster response: players plus optional match-level context. */
type RosterResponse = {
  players: RosterPlayer[];
  match?: RosterMatchResponse;
};

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Default vertical-reel preset/variant when an intent predates preset selection. */
const REEL_VARIANT = DEFAULT_VARIANT;

/** The render variant for a reel: the user's chosen preset, else the default. */
function variantOf(intent: Pick<ReelIntent, 'variant'>): string {
  return intent.variant ?? REEL_VARIANT;
}

/** Display-only labels for the known presets (server is the source of truth). */
const VARIANT_LABELS: Record<string, string> = {
  'viral-60-clean': 'Killfeed',
  'clean-pov-60': 'POV limpio',
  'full-hud-60': 'HUD completo',
  'gameplay-pov-60': 'POV nativo',
};

function variantLabel(variant: string): string {
  return VARIANT_LABELS[variant] ?? variant;
}

function isJobId(id: string): boolean {
  return UUID_RE.test(id);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function readJson<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const message = body && typeof body.error === 'string' ? body.error : `request failed (${res.status})`;
    // Carry the backend's stable `code` so callers do not sniff the message.
    const err = new Error(message) as Error & { code?: string };
    if (body && typeof body.code === 'string') err.code = body.code;
    throw err;
  }
  return (await res.json()) as T;
}



/** Bare song key at full default mix; object when music or game gain is set. */
function buildMusicRequest(
  intent: ReelIntent,
): string | { key: string; volume?: number; game_volume?: number } | undefined {
  if (intent.mode !== 'music' || !intent.songId) return undefined;
  const volume = intent.musicVolume !== undefined && intent.musicVolume < 1 ? intent.musicVolume : undefined;
  const gameVolume = intent.gameVolume;
  if (volume === undefined && gameVolume === undefined) return intent.songId;
  return {
    key: intent.songId,
    ...(volume !== undefined ? { volume } : {}),
    ...(gameVolume !== undefined ? { game_volume: gameVolume } : {}),
  };
}

function musicChoiceFromEffective(music: EffectiveRenderMusic | undefined): MusicChoice | undefined {
  if (!music) return undefined;
  if (music.mode === 'clean') return {};
  return { songId: music.songId, musicVolume: music.musicVolume, gameVolume: music.gameVolume };
}



/** A queued placeholder Video for an intent; its live status is filled by reconcile. */
function videoFromIntent(intent: ReelIntent): Video {
  return {
    id: intent.videoId,
    jobId: intent.jobId,
    title: intent.title,
    map: intent.map,
    score: intent.score,
    targetName: intent.targetName,
    mode: intent.mode,
    variant: intent.variant,
    songId: intent.songId,
    musicVolume: intent.musicVolume,
    gameVolume: intent.gameVolume,
    editConfig: intent.editConfig,
    status: 'queued',
    createdAt: intent.createdAt,
    availableForSec: 14 * 3600,
  };
}

/** Local orchestrator client: persist intents, reconcile live status via proxy. */
export class RealApiClient implements ApiClient {
  private readonly fallback = new MockApiClient();
  /** Live, derived view of each tracked reel (status/downloadUrl/failureReason). */
  private readonly reels = new Map<string, Video>();
  /** Durable facts the user asked for, mirrored to localStorage via reel-store. */
  private readonly intents = new Map<string, ReelIntent>();
  /** Reels with a record/render POST in flight, so a tick never double-drives. */
  private readonly driving = new Set<string>();

  /** Consecutive 404 status polls per reel, feeding the unrecoverable latch. */
  private readonly jobGoneTicks = new Map<string, number>();
  /** POST /record accepted; job may still read failed until the worker dequeues. */
  private readonly pendingCapture = new Set<string>();
  /** Server-reported artifact names for each reel (the file names the editor wrote). */
  private readonly artifactNames = new Map<string, { video: string; cover?: string; covers?: string[] }>();
  /** Cached per-job series match (map/score); immutable once a job has one. */
  private readonly seriesMatches = new Map<string, RosterMatch>();

  constructor() {
    // Rehydrate persisted intents so the Library survives a hard reload.
    for (const intent of loadReelIntents()) {
      this.intents.set(intent.videoId, intent);
      this.reels.set(intent.videoId, videoFromIntent(intent));
    }
  }

  /** The local same-origin proxy data plane (the only transport this app has). */
  private dp(): DataPlane {
    return dataPlane();
  }

  /** One data-plane request; merges transport headers onto the built init. */
  private async send(build: (dp: DataPlane) => { url: string; init?: RequestInit }): Promise<Response> {
    const dp = this.dp();
    const { url, init } = build(dp);
    const headers = { ...dp.headers, ...((init?.headers as Record<string, string> | undefined) ?? {}) };
    return fetch(url, { ...init, headers });
  }

  /** Reads a job's roster scan (players + optional match context) from the proxy. */
  private async fetchRoster(jobId: string): Promise<RosterResponse> {
    return readJson<RosterResponse>(await this.send((dp) => ({ url: dp.rosterUrl(jobId) })));
  }

  async scanDemo(file: File, opts?: { seriesId?: string }): Promise<{ jobId: string; players: DemoPlayer[]; match?: RosterMatch }> {
    const dp = this.dp();
    const form = new FormData();
    form.append(dp.scanField, file);
    // series_id rides the multipart body so the orchestrator groups the scan.
    if (opts?.seriesId) form.append(dp.scanSeriesField, opts.seriesId);
    const scanned = await readJson<unknown>(
      await this.send((d) => ({ url: d.scanUrl, init: { method: 'POST', body: form } })),
    );
    const jobId = dp.scanJobId(scanned);

    await this.waitForStatus(jobId, 'scanned');

    const roster = await readJson<RosterResponse>(await this.send((d) => ({ url: d.rosterUrl(jobId) })));
    return { jobId, players: roster.players.map(toDemoPlayer), match: toRosterMatch(roster.match) };
  }

  /** Series demos in upload order; a roster miss leaves that map unmatched. */
  async getSeries(seriesId: string): Promise<SeriesDemo[]> {
    type ProxyDemo = { jobId: string; status: string; failureReason?: string; fileName?: string };
    const body = await readJson<{ demos: ProxyDemo[] }>(await this.send((dp) => ({ url: dp.seriesUrl(seriesId) })));
    return Promise.all(
      body.demos.map(async (raw): Promise<SeriesDemo> => {
        const demo: SeriesDemo = { jobId: raw.jobId, status: raw.status };
        if (raw.fileName) demo.fileName = raw.fileName;
        if (raw.failureReason) demo.failureReason = raw.failureReason;
        const cached = this.seriesMatches.get(raw.jobId);
        if (cached) {
          demo.match = cached;
        } else if (ROSTER_READY.has(raw.status)) {
          try {
            const roster = await this.fetchRoster(raw.jobId);
            const match = toRosterMatch(roster.match);
            if (match) {
              this.seriesMatches.set(raw.jobId, match);
              demo.match = match;
            }
          } catch {
            // Roster not ready (409) or a transient failure: leave match unset.
          }
        }
        return demo;
      }),
    );
  }

  async parseDemo(input: { jobId: string; steamId: string }): Promise<Match> {
    await readJson<unknown>(
      await this.send((dp) => ({
        url: dp.parseUrl(input.jobId),
        init: {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(dp.parseBody(input.steamId)),
        },
      })),
    );

    await this.waitForStatus(input.jobId, 'parsed');

    const [plan, roster] = await Promise.all([
      readJson<KillPlan>(await this.send((dp) => ({ url: dp.planUrl(input.jobId) }))),
      readJson<RosterResponse>(await this.send((dp) => ({ url: dp.rosterUrl(input.jobId) }))),
    ]);

    const picked = roster.players.find((p) => p.steamid64 === input.steamId);
    if (!picked) throw new Error('chosen player not found in roster');
    return planToMatch(input.jobId, plan, toDemoPlayer(picked));
  }

  async getMatch(id: string): Promise<Match | null> {
    if (!isJobId(id)) return this.fallback.getMatch(id);

    const status = await this.fetchStatus(id);
    if (status === null) return null;
    if (!ROSTER_READY.has(status)) return null;

    // Parsing / scanned: listable in Partidas but no kill plan yet.
    if (!PLAN_READY_STATUSES.has(status)) {
      return this.jobToMatchEnriched({ jobId: id, status });
    }

    const plan = await readJson<KillPlan>(await this.send((dp) => ({ url: dp.planUrl(id) })));
    const match = planToMatch(id, plan, await this.summaryPlayer(id, plan));
    match.status = status;
    return match;
  }

  /** Plan target from roster when present; otherwise the plan's own target. */
  private async summaryPlayer(jobId: string, plan: KillPlan): Promise<DemoPlayer> {
    try {
      const { players } = await readJson<RosterResponse>(await this.send((dp) => ({ url: dp.rosterUrl(jobId) })));
      const row = players.find((p) => p.steamid64 === plan.target?.steamid64) ?? players[0];
      if (row) return toDemoPlayer(row);
    } catch {
      // No roster for this job; fall back to the plan's target below.
    }
    return {
      steamId: plan.target?.steamid64 ?? '',
      name: plan.target?.name_in_demo ?? '',
      team: normalizeTeam(plan.target?.team_at_start ?? ''),
      kills: plan.stats?.total_kills_target ?? 0,
      deaths: 0,
      assists: 0,
      headshots: 0,
      mvps: 0,
      rounds: 0,
      adr: 0,
      hsPct: 0,
      kast: 0,
      rating: 0,
    };
  }

  async findClips(matchId: string): Promise<Play[]> {
    if (!isJobId(matchId)) return this.fallback.findClips(matchId);

    const status = await this.fetchStatus(matchId);
    // No plan until parsing finishes; it persists through record/render.
    if (status === null || !PLAN_READY_STATUSES.has(status)) return [];

    const plan = await readJson<KillPlan>(await this.send((dp) => ({ url: dp.planUrl(matchId) })));
    return planToPlays(matchId, plan);
  }

  async findRecapClips(matchId: string): Promise<Play[]> {
    if (!isJobId(matchId)) return this.fallback.findRecapClips(matchId);

    const status = await this.fetchStatus(matchId);
    if (status === null || !PLAN_READY_STATUSES.has(status)) return [];

    const res = await this.send((dp) => ({ url: dp.recapPlanUrl(matchId) }));
    if (res.status === 409) return [];
    const plan = await readJson<KillPlan>(res);
    return planToPlays(matchId, plan);
  }

  /** Polls /status until it reaches `want`; throws on `failed` or timeout. */
  private async waitForStatus(jobId: string, want: string, maxAttempts = 240): Promise<void> {
    for (let attempt = 0; attempt < maxAttempts; attempt++) {
      const status = await this.fetchStatus(jobId);
      if (status === want) return;
      if (status === 'failed') throw new Error(`job ${jobId} failed`);
      await sleep(800);
    }
    throw new Error(`timed out waiting for ${want}`);
  }

  /** Register a durable reel intent; reconcile drives record→render. */
  async createVideo(input: { matchId: string; playIds: string[]; mode: RenderMode; songId?: string; musicVolume?: number; gameVolume?: number; variant?: string; editConfig?: EditConfig }): Promise<Video> {
    if (!isJobId(input.matchId)) return this.fallback.createVideo(input);

    const editConfig = constrainEditConfig(input.editConfig ?? DEFAULT_EDIT_CONFIG);
    const normalized = { ...input, editConfig };
    const videoId = reelIdentity(normalized);
    const existing = this.reels.get(videoId);
    const existingIntent = this.intents.get(videoId);
    if (existing && existingIntent && shouldReuseReelIntent(existing, existingIntent, { ...normalized, mode: input.mode })) {
      return { ...existing };
    }

    const recap = isLandscapeRecap(editConfig);
    const [plays, match] = await Promise.all([
      recap ? this.findRecapClips(input.matchId) : this.findClips(input.matchId),
      this.getMatch(input.matchId),
    ]);
    // Recap records every stored round. Shorts keep the caller's plan order.
    const pickedPlays = recap
      ? plays
      : input.playIds.map((pid) => plays.find((p) => p.id === pid)).filter((p): p is Play => Boolean(p));
    const variant = input.variant ?? REEL_VARIANT;
    const suffix = input.songId ? `${variantLabel(variant)} + Music` : variantLabel(variant);
    const selectionTitle = recap
      ? `${pickedPlays.length} ${pickedPlays.length === 1 ? 'ronda' : 'rondas'}`
      : (playsSelectionLabel(pickedPlays) ?? 'Highlight');
    const intent: ReelIntent = {
      videoId,
      jobId: input.matchId,
      segmentIds: recap ? [] : pickedPlays.map((p) => p.id),
      mode: input.mode,
      variant,
      editConfig,
      songId: input.songId,
      // Volume only rides along with a chosen song; without one it is meaningless.
      musicVolume: input.songId ? input.musicVolume : undefined,
      gameVolume: input.songId ? input.gameVolume : undefined,
      title: `${selectionTitle} - ${suffix}`,
      map: match?.map ?? 'Unknown',
      score: match?.score ?? '',
      targetName: match?.player,
      createdAt: Date.now(),
    };
    this.intents.set(videoId, intent);
    saveReelIntents(Array.from(this.intents.values()));
    this.reels.set(videoId, videoFromIntent(intent));
    void this.reconcile(); // kick now (idempotent); /videos polling continues it.
    return { ...videoFromIntent(intent) };
  }

  async listVideos(): Promise<Video[]> {
    await this.reconcile();
    // The Library shows only the user's own real reels, persisted on this PC.
    return Array.from(this.reels.values())
      .sort((a, b) => b.createdAt - a.createdAt)
      .map((v) => ({ ...v }));
  }

  async getVideo(id: string): Promise<Video | null> {
    const reel = this.reels.get(id);
    if (reel) return { ...reel };
    return this.fallback.getVideo(id);
  }

  async getPublishAssistant(id: string): Promise<PublishAssistant> {
    const intent = this.intents.get(id);
    if (!intent) return this.fallback.getPublishAssistant(id);
    const reel = this.reels.get(id);
    if (!reel || reel.status !== 'ready') throw new Error('video is not ready for publication');
    const variant = variantOf(intent);
    const name = await this.resolveArtifactName(intent, variant);
    if (!name) throw new Error('rendered video artifact is not available');
    const raw = await readJson<unknown>(
      await this.send((dp) => ({
        url: dp.publishAssistantUrl(intent.jobId, variant, name),
        init: { cache: 'no-store' },
      })),
    );
    return parsePublishAssistant(raw);
  }

  /** Re-drive a failed reel: re-record a failed job, else re-render. */
  async retryVideo(id: string): Promise<Video> {
    const intent = this.intents.get(id);
    if (!intent) return this.fallback.retryVideo(id);

    // Gone jobs cannot be re-driven; return the latch instead of re-failing.
    const current = this.reels.get(id);
    if (current?.unrecoverable) return { ...current };

    this.applyView(intent, { status: 'queued', action: 'none' });
    const [job, render] = await Promise.all([
      this.fetchStatusFull(intent.jobId),
      this.fetchRenderStatus(intent.jobId, variantOf(intent)),
    ]);
    const retryAction = retryReelAction({
      jobStatus: job?.status ?? '',
      renderStatus: render.status,
      renderFailureReason: render.failureReason,
    });
    if (retryAction !== 'none') {
      await this.drive(intent, retryAction);
    }
    await this.reconcile();
    return { ...(this.reels.get(id) ?? videoFromIntent(intent)) };
  }

  async resolveVideoReview(id: string, resolution: VideoReviewResolution): Promise<Video> {
    const intent = this.intents.get(id);
    if (!intent) return this.fallback.resolveVideoReview(id, resolution);
    const current = this.reels.get(id);
    if (!current || current.status !== 'review_required') {
      throw new Error('El reel ya no está pendiente de revisión.');
    }

    if (resolution.kind === 'rerender') {
      if (editConfigsEqual(intent.editConfig, resolution.editConfig)) {
        throw new Error('Cambia al menos una opción de edición antes de volver a renderizar.');
      }
      if (this.driving.has(intent.videoId)) {
        throw new Error('Ya hay una operación activa para este reel.');
      }
      this.driving.add(intent.videoId);
      try {
        // Persist the new edit only after the server admits this render.
        await readJson<unknown>(
          await this.send((dp) => ({
            url: dp.renderUrl(intent.jobId, variantOf(intent)),
            init: {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({
                music: buildMusicRequest(intent),
                edit: buildEditRequest(resolution.editConfig),
                expected_artifact_prefix: resolution.expectedArtifactPrefix,
                expected_warnings: resolution.expectedWarnings,
              }),
            },
          })),
        );
        this.artifactNames.delete(intent.videoId);
        const previousRevision = this.reels.get(intent.videoId);
        if (previousRevision) {
          this.reels.set(intent.videoId, clearVideoArtifactUrls(previousRevision));
        }
        intent.editConfig = resolution.editConfig;
        // New candidates will appear after re-render.
        delete intent.selectedCoverName;
        saveReelIntents(Array.from(this.intents.values()));
        this.applyView(intent, { status: 'queued', action: 'none' });
      } finally {
        this.driving.delete(intent.videoId);
      }
      await this.reconcileOne(intent);
      return { ...(this.reels.get(id) ?? videoFromIntent(intent)) };
    }

    const note = resolution.note.trim();
    if (!note) throw new Error('Documenta por qué los avisos son intencionales.');
    await readJson<unknown>(
      await this.send((dp) => ({
        url: dp.renderReviewUrl(intent.jobId, variantOf(intent)),
        init: {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            note,
            expected_artifact_prefix: resolution.expectedArtifactPrefix,
            expected_warnings: resolution.expectedWarnings,
          }),
        },
      })),
    );
    await this.reconcileOne(intent);
    return { ...(this.reels.get(id) ?? videoFromIntent(intent)) };
  }

  /** Re-render a ready reel with a new mix; persist only after POST accepts. */
  async rerenderVideoMusic(id: string, choice: MusicChoice): Promise<Video> {
    const intent = this.intents.get(id);
    if (!intent) return this.fallback.rerenderVideoMusic(id, choice);
    const current = this.reels.get(id);
    if (!current || current.status !== 'ready') {
      throw new Error('El reel tiene que estar listo para añadir o cambiar la música.');
    }
    if (current.unrecoverable) {
      throw new Error('Este reel ya no se puede volver a renderizar.');
    }
    const nextChoice: MusicChoice = choice.songId
      ? { songId: choice.songId, musicVolume: choice.musicVolume, gameVolume: choice.gameVolume }
      : {};
    if (musicChoicesEqual({ songId: intent.songId, musicVolume: intent.musicVolume, gameVolume: intent.gameVolume }, nextChoice)) {
      throw new Error('Elige una pista o un volumen distinto al actual.');
    }
    if (this.driving.has(intent.videoId)) {
      throw new Error('Ya hay una operación activa para este reel.');
    }

    this.driving.add(intent.videoId);
    try {
      const nextIntent: ReelIntent = { ...intent };
      applyMusicChoice(nextIntent, nextChoice);
      nextIntent.title = titleWithMusicSuffix(
        intent.title,
        variantLabel(variantOf(intent)),
        Boolean(nextIntent.songId),
      );
      const accepted = await readJson<{ accepted?: boolean; duplicate?: boolean }>(
        await this.send((dp) => ({
          url: dp.renderUrl(intent.jobId, variantOf(intent)),
          init: {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              music: buildMusicRequest(nextIntent),
              edit: buildEditRequest(intent.editConfig),
            }),
          },
        })),
      );
      if (accepted.duplicate && accepted.accepted !== true) {
        throw new Error('No se pudo encolar un render nuevo. Espera o cambia pista o volumen.');
      }
      this.artifactNames.delete(intent.videoId);
      applyMusicChoice(intent, nextChoice);
      intent.title = nextIntent.title;
      saveReelIntents(Array.from(this.intents.values()));
      const previousRevision = this.reels.get(intent.videoId);
      if (previousRevision) {
        this.reels.set(intent.videoId, {
          ...clearVideoArtifactUrls(previousRevision),
          title: intent.title,
        });
      }
      this.applyView(intent, { status: 'queued', action: 'none' });
    } finally {
      this.driving.delete(intent.videoId);
    }
    await this.reconcileOne(intent);
    return { ...(this.reels.get(id) ?? videoFromIntent(intent)) };
  }

  async selectVideoCover(id: string, coverName: string): Promise<Video> {
    const intent = this.intents.get(id);
    if (!intent) throw new Error('Reel desconocido.');
    const names = this.artifactNames.get(id);
    const candidates = names?.covers ?? (names?.cover ? [names.cover] : []);
    if (!candidates.includes(coverName)) {
      throw new Error('Esa portada ya no está entre las candidatas de este reel.');
    }
    intent.selectedCoverName = coverName;
    saveReelIntents(Array.from(this.intents.values()));
    const current = this.reels.get(id) ?? videoFromIntent(intent);
    const variant = variantOf(intent);
    const next: Video = {
      ...current,
      selectedCoverName: coverName,
      coverCandidates: candidates,
      thumbnailUrl: dataPlane().coverUrl(intent.jobId, variant, coverName),
    };
    this.reels.set(id, next);
    return { ...next };
  }

  async deleteVideo(id: string): Promise<void> {
    const intent = this.intents.get(id);
    if (!intent) return this.fallback.deleteVideo(id);
    try {
      const variant = variantOf(intent);
      const name = await this.resolveArtifactName(intent, variant);
      // Nothing rendered yet: drop the local intent below.
      if (name) {
        await this.send((dp) => ({ url: dp.videoUrl(intent.jobId, variant, name), init: { method: 'DELETE' } }));
      }
    } catch {
      // Offline: drop the library row; a later render overwrites leftovers.
    }
    this.artifactNames.delete(id);
    this.intents.delete(id);
    this.reels.delete(id);
    saveReelIntents(Array.from(this.intents.values()));
  }

  /** Delete the job and its artifacts; 404 is success, 409/503 throw. */
  async deleteMatch(jobId: string): Promise<void> {
    const res = await this.send((dp) => ({ url: dp.jobDeleteUrl(jobId), init: { method: 'DELETE' } }));
    // 404 is already-gone success; any other non-2xx throws with the body.
    if (res.status !== 404 && !res.ok) await readJson<unknown>(res);
    this.pruneJob(jobId);
  }

  /** Delete every demo in the series; a busy member surfaces as 409. */
  async deleteSeries(seriesId: string): Promise<void> {
    const demos = await this.getSeries(seriesId);
    for (const demo of demos) {
      await this.deleteMatch(demo.jobId);
    }
  }

  /** Drop local intents/views for a deleted job, then persist survivors. */
  private pruneJob(jobId: string): void {
    for (const [videoId, intent] of this.intents) {
      if (intent.jobId !== jobId) continue;
      this.intents.delete(videoId);
      this.reels.delete(videoId);
      this.artifactNames.delete(videoId);
      this.jobGoneTicks.delete(videoId);
    }
    this.seriesMatches.delete(jobId);
    saveReelIntents(Array.from(this.intents.values()));
  }

  /** Reconcile non-terminal reels against the orchestrator and drive the next step. */
  private async reconcile(): Promise<void> {
    const active = Array.from(this.intents.values()).filter((intent) => {
      const v = this.reels.get(intent.videoId);
      return shouldReconcileVideoStatus(v?.status) && !v?.unrecoverable;
    });
    await reconcileReels(active.map((intent) => this.reconcileOne(intent)));
  }

  private async reconcileOne(intent: ReelIntent): Promise<void> {
    const [job] = await Promise.all([
      this.fetchStatusFull(intent.jobId),
      this.hydrateIntentTarget(intent),
    ]);
    if (job === null) {
      // Consecutive 404s latch the reel; one tick can be a spurious miss.
      const strikes = (this.jobGoneTicks.get(intent.videoId) ?? 0) + 1;
      this.jobGoneTicks.set(intent.videoId, strikes);
      const gone = viewForJobGone(strikes);
      if (gone) this.applyView(intent, gone);
      return;
    }
    this.jobGoneTicks.delete(intent.videoId);
    // Skip render GET until recorded; earlier it is a guaranteed 404.
    const render: {
      status: RenderStatus;
      failureReason?: string;
      warnings?: string[];
      videoName?: string;
      coverName?: string;
      coverNames?: string[];
      artifactPrefix?: string;
      editConfig?: EditConfig;
      effectiveMusic?: EffectiveRenderMusic;
    } =
      canHaveRenderState(job.status)
        ? await this.fetchRenderStatus(intent.jobId, variantOf(intent))
        : { status: 'none' };
    // Use the editor's real artifact names instead of guessing from segment ids.
    if (render.videoName) {
      const names: { video: string; cover?: string; covers?: string[] } = { video: render.videoName };
      if (render.coverNames && render.coverNames.length > 0) {
        names.covers = render.coverNames;
        names.cover = render.coverName ?? render.coverNames[0];
      } else if (render.coverName) {
        names.cover = render.coverName;
        names.covers = [render.coverName];
      }
      this.artifactNames.set(intent.videoId, names);
    }
    if (job.status !== 'failed') {
      this.pendingCapture.delete(intent.videoId);
    }
    const decision = decideReelReconcile({
      jobStatus: job.status,
      jobFailureReason: job.failureReason,
      renderStatus: render.status,
      renderFailureReason: render.failureReason,
      renderWarnings: render.warnings,
      renderArtifactPrefix: render.artifactPrefix,
      captureProgress: job.captureProgress,
      recordAdmitted: this.pendingCapture.has(intent.videoId),
      intentEdit: intent.editConfig,
      renderEdit: render.editConfig,
      intentMusic: {
        songId: intent.songId,
        musicVolume: intent.musicVolume,
        gameVolume: intent.gameVolume,
      },
      renderMusic: musicChoiceFromEffective(render.effectiveMusic),
    });
    if (decision.adoptEffective && (render.status === 'ready' || render.status === 'review_required')) {
      let intentChanged = false;
      if (render.editConfig && !editConfigsEqual(intent.editConfig, render.editConfig)) {
        intent.editConfig = render.editConfig;
        intentChanged = true;
      }
      if (render.effectiveMusic) {
        intentChanged = applyEffectiveRenderMusic(intent, render.effectiveMusic) || intentChanged;
      }
      if (intentChanged) saveReelIntents(Array.from(this.intents.values()));
    }
    this.applyView(intent, decision.view);
    if (decision.view.action !== 'none') void this.drive(intent, decision.view.action);
  }

  /** Writes a reel's derived view onto its live Video, wiring URLs once ready. */
  private applyView(intent: ReelIntent, view: ReelView): void {
    const base = this.reels.get(intent.videoId) ?? videoFromIntent(intent);
    // captureProgress only belongs on a recording view.
    const next = hydrateVideoFromIntent({
      ...base,
      status: view.status,
      failureReason: view.failureReason,
      warnings: view.warnings,
      reviewArtifactPrefix: view.reviewArtifactPrefix,
      captureProgress: view.captureProgress,
    }, intent);
    if (intent.targetName) next.targetName = intent.targetName;
    // Latch stays set: a later plain-failed view must not clear it.
    delete next.unrecoverable;
    if (view.unrecoverable || base.unrecoverable) next.unrecoverable = true;
    // Ready without names yet: keep placeholder URLs until the next tick.
    const names = this.artifactNames.get(intent.videoId);
    if ((view.status === 'ready' || view.status === 'review_required') && names) {
      // Same-origin proxy URLs the browser can hand straight to <video>/<img>.
      const variant = variantOf(intent);
      const dp = dataPlane();
      next.downloadUrl = dp.videoUrl(intent.jobId, variant, names.video);
      if (names.covers && names.covers.length > 0) {
        next.coverCandidates = [...names.covers];
      }
      const approvedCover =
        intent.selectedCoverName && names.covers?.includes(intent.selectedCoverName)
          ? intent.selectedCoverName
          : undefined;
      if (approvedCover) {
        next.selectedCoverName = approvedCover;
        next.thumbnailUrl = dp.coverUrl(intent.jobId, variant, approvedCover);
      } else if (names.cover) {
        // Preview the first candidate without treating it as approved.
        next.thumbnailUrl = dp.coverUrl(intent.jobId, variant, names.cover);
      }
    }
    this.reels.set(intent.videoId, next);
  }

  /** Fill a legacy intent's target name from the kill plan; miss is retried. */
  private async hydrateIntentTarget(intent: ReelIntent): Promise<void> {
    if (intent.targetName) return;
    try {
      const plan = await readJson<KillPlan>(
        await this.send((dp) => ({ url: dp.planUrl(intent.jobId) })),
      );
      const targetName = plan.target?.name_in_demo?.trim();
      if (!targetName) return;
      intent.targetName = targetName;
      saveReelIntents(Array.from(this.intents.values()));
    } catch {
      // Plan can be temporarily unavailable while parsing or the service is offline.
    }
  }

  /** Issues the single pipeline POST for `action`, guarded so it fires at most once. */
  private async drive(intent: ReelIntent, action: ReelAction): Promise<void> {
    if (this.driving.has(intent.videoId)) return;
    this.driving.add(intent.videoId);
    const variant = variantOf(intent);
    try {
      const res =
        action === 'record'
          ? await this.send((dp) => ({
              url: dp.recordUrl(intent.jobId),
              init: {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  preset: variant,
                  segment_ids: intent.segmentIds,
                  edit: buildEditRequest(intent.editConfig),
                }),
              },
            }))
          : await this.send((dp) => ({
              url: dp.renderUrl(intent.jobId, variant),
              init: {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  music: buildMusicRequest(intent),
                  edit: buildEditRequest(intent.editConfig),
                }),
              },
            }));
      if (res.ok && action === 'record') {
        this.pendingCapture.add(intent.videoId);
        this.applyView(intent, { status: 'recording', action: 'none' });
        return;
      }
      if (!res.ok) {
        const body = (await res.json().catch(() => ({}))) as { error?: string; code?: string };
        if (action === 'record') {
          const view = viewForRecordAdmission(res.status, body);
          if (view === null) return;
          this.applyView(intent, view);
          return;
        }
        // 503 is transient; the next reconcile tick retries.
        if (body.code === SERVICE_UNAVAILABLE_CODE) return;
        // Job vanished mid-POST: latch unrecoverable so Retry cannot spin.
        if (res.status === 404) {
          this.applyView(intent, unrecoverableJobGoneView());
          return;
        }
        // Durable failure (e.g. capture unconfigured): surface it on the card.
        this.applyView(intent, {
          status: 'failed',
          action: 'none',
          failureReason: body.error || 'failed to start rendering',
        });
      }
    } catch {
      // network blip; the next reconcile tick re-evaluates from server truth.
    } finally {
      this.driving.delete(intent.videoId);
    }
  }

  /** Cached editor artifact name, or a fresh fetch after reload. */
  private async resolveArtifactName(intent: ReelIntent, variant: string): Promise<string | undefined> {
    const cached = this.artifactNames.get(intent.videoId);
    if (cached) return cached.video;
    const render = await this.fetchRenderStatus(intent.jobId, variant);
    return render.videoName;
  }

  /** Job status, failure, and capture progress; null on 404. */
  private async fetchStatusFull(
    jobId: string,
  ): Promise<{ status: string; failureReason?: string; captureProgress?: CaptureProgress } | null> {
    const res = await this.send((dp) => ({ url: dp.jobStatusUrl(jobId) }));
    if (res.status === 404) return null;
    const data = await readJson<{
      status: string;
      failure_reason?: string;
      progress?: { done?: number; total?: number; percent?: number };
    }>(res);
    const full: { status: string; failureReason?: string; captureProgress?: CaptureProgress } = {
      status: data.status,
      failureReason: data.failure_reason,
    };
    const parsed = parseCaptureProgress(data.progress);
    if (parsed) full.captureProgress = parsed;
    return full;
  }

  /** Reads the job status string; null when the job is unknown (404). */
  private async fetchStatus(jobId: string): Promise<string | null> {
    const full = await this.fetchStatusFull(jobId);
    return full ? full.status : null;
  }

  /** Render-variant state; 'none' until started. Artifact names arrive when ready. */
  private async fetchRenderStatus(
    jobId: string,
    variant: string,
  ): Promise<{
    status: RenderStatus;
    failureReason?: string;
    warnings?: string[];
    videoName?: string;
    coverName?: string;
    coverNames?: string[];
    artifactPrefix?: string;
    editConfig?: EditConfig;
    effectiveMusic?: EffectiveRenderMusic;
  }> {
    const res = await this.send((dp) => ({ url: dp.renderUrl(jobId, variant) }));
    if (res.status === 404) return { status: 'none' };
    if (!res.ok) await readJson<never>(res);
    const data = (await res.json()) as {
      status?: string;
      failure_reason?: string;
      warnings?: string[];
      videos?: string[];
      covers?: string[];
      artifact_prefix?: string;
      music?: unknown;
      edit?: {
        format?: EditConfig['format'];
        killEffect?: EditConfig['killEffect'];
        transition?: EditConfig['transition'];
        intro?: boolean;
        outro?: boolean;
        hook_text?: boolean;
        kill_counter?: boolean;
        cover_strategy?: EditConfig['coverStrategy'];
        intro_text?: string;
        outro_text?: string;
      };
    };
    const known = new Set<RenderStatus>([
      'queued',
      'rendering',
      'ready',
      'review_required',
      'failed',
    ]);
    const status: RenderStatus = data.status && known.has(data.status as RenderStatus) ? (data.status as RenderStatus) : 'none';
    const coverNames = Array.isArray(data.covers)
      ? data.covers.filter((name): name is string => typeof name === 'string' && name.length > 0)
      : undefined;
    return {
      status,
      failureReason: data.failure_reason,
      warnings: data.warnings,
      videoName: data.videos?.[0],
      coverName: coverNames?.[0],
      coverNames,
      artifactPrefix: data.artifact_prefix,
      editConfig: parseEffectiveEditConfig(data.edit),
      effectiveMusic: parseEffectiveRenderMusic(data.music),
    };
  }

  /** Capture readiness from /api/capabilities; 503 is offline, not unconfigured. */
  async getCaptureReadiness(): Promise<CaptureReadiness> {
    try {
      const res = await this.send((dp) => ({ url: dp.capabilitiesUrl, init: { cache: 'no-store' } }));
      if (!res.ok) {
        // Non-ok is transport; unconfigured arrives as 200 with record.enabled=false.
        return { recordEnabled: false, status: 'offline', tools: [], reason: 'local analysis service offline' };
      }
      const data = (await res.json()) as { record?: { enabled?: boolean; tools?: CaptureTool[] } };
      const tools = data.record?.tools ?? [];
      const enabled = Boolean(data.record?.enabled);
      const anyMissing = tools.some((t) => t.configured && !t.accessible);
      let status: CaptureStatus;
      if (!enabled) {
        status = 'unconfigured';
      } else if (anyMissing) {
        status = 'warning';
      } else {
        status = 'ready';
      }
      return { recordEnabled: enabled, status, tools };
    } catch {
      return { recordEnabled: false, status: 'offline', tools: [] };
    }
  }
  /** Persisted jobs as Partidas rows; roster enrichment is best-effort. */
  async listMatches(): Promise<Match[]> {
    const jobs = await this.fetchJobs();
    return Promise.all(listableJobs(jobs).map((job) => this.jobToMatchEnriched(job)));
  }

  async listPlanReadyMatches(): Promise<Match[]> {
    const jobs = await this.fetchJobs();
    return Promise.all(planReadyJobs(jobs).map((job) => this.jobToMatchEnriched(job)));
  }

  /** One series row per bulk upload, including maps still scanning. */
  async listSeriesSummaries(): Promise<SeriesSummary[]> {
    return summarizeSeries(await this.fetchJobs());
  }

  /** The recent demo jobs the orchestrator persists (the Partidas index feed). */
  private async fetchJobs(): Promise<IndexedJob[]> {
    const body = await readJson<{ jobs: IndexedJob[] }>(await this.send((dp) => ({ url: dp.jobsUrl })));
    return body.jobs;
  }

  /** Job to Match; a missing roster still lists a zeroed filename row. */
  private async jobToMatchEnriched(job: IndexedJob): Promise<Match> {
    try {
      const roster = await this.fetchRoster(job.jobId);
      const enrichment: { map?: string; player?: DemoPlayer } = {};
      if (roster.match) enrichment.map = roster.match.map;
      const row = job.targetSteamId ? roster.players.find((p) => p.steamid64 === job.targetSteamId) : undefined;
      if (row) enrichment.player = toDemoPlayer(row);
      return jobToMatch(job, enrichment);
    } catch {
      // Roster not ready (409) or a transient failure: still list the demo.
      return jobToMatch(job);
    }
  }
  /** @deprecated Superseded by scanDemo + parseDemo. */
  uploadDemo(input: { fileName: string }): Promise<Match> {
    return this.fallback.uploadDemo(input);
  }
  /** Real music catalog from the orchestrator; falls back to the mock offline. */
  async listSongs(): Promise<Song[]> {
    try {
      const res = await fetch('/api/songs', { cache: 'no-store' });
      const data = await readJson<{
        songs: Array<{ id: string; title: string; artist?: string; genre?: string; durationSec?: number; license?: string; audioUrl: string }>;
      }>(res);
      return data.songs.map((s) => ({
        id: s.id,
        title: s.title,
        artist: s.artist ?? '',
        genre: s.genre ?? '',
        previewUrl: s.audioUrl,
        durationSec: s.durationSec ?? 0,
        license: s.license,
      }));
    } catch {
      return this.fallback.listSongs();
    }
  }

  /** Real preset registry from the orchestrator; falls back to the mock offline. */
  async listPresets(): Promise<Preset[]> {
    try {
      const res = await fetch('/api/presets', { cache: 'no-store' });
      const data = await readJson<{
        default?: string;
        presets: Array<{
          name: string;
          label?: string;
          description?: string;
          hud_mode?: string;
          default?: boolean;
          width?: number;
          height?: number;
        }>;
      }>(res);
      return data.presets.map((p) => ({
        name: p.name,
        label: p.label ?? p.name,
        description: p.description ?? '',
        hudMode: p.hud_mode,
        default: p.default,
        width: typeof p.width === 'number' && p.width > 0 ? p.width : undefined,
        height: typeof p.height === 'number' && p.height > 0 ? p.height : undefined,
      }));
    } catch {
      return this.fallback.listPresets();
    }
  }

  listFeed(): Promise<FeedItem[]> {
    // The community feed was a cloud surface; the desktop app shows no feed.
    return Promise.resolve([]);
  }
}

/** Server roster row (steamid64) → the UI's DemoPlayer (steamId). */
function toDemoPlayer(p: RosterPlayer): DemoPlayer {
  return {
    steamId: p.steamid64,
    name: p.name,
    team: normalizeTeam(p.team),
    kills: p.kills,
    deaths: p.deaths,
    assists: p.assists,
    headshots: p.headshots ?? 0,
    mvps: p.mvps ?? 0,
    rounds: p.rounds ?? 0,
    adr: p.adr ?? 0,
    hsPct: p.hs_pct ?? 0,
    kast: p.kast ?? 0,
    rating: p.rating ?? 0,
    rounds2k: p.rounds_2k,
    rounds3k: p.rounds_3k,
    rounds4k: p.rounds_4k,
    rounds5k: p.rounds_5k,
  };
}

/** Keeps only the known sides; anything else collapses to '' (spectator/unknown). */
function normalizeTeam(team: string): DemoPlayer['team'] {
  return team === 'CT' || team === 'T' ? team : '';
}

/** Server match summary (snake_case) → the UI's RosterMatch; undefined when the scan has none. */
function toRosterMatch(m: RosterMatchResponse | undefined): RosterMatch | undefined {
  if (!m) return undefined;
  return { map: m.map, scoreCt: m.score_ct, scoreT: m.score_t, rounds: m.rounds };
}
