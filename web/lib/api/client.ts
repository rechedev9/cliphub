import type { Match, Play, Song, Video, FeedItem, RenderMode, DemoPlayer, Preset, EditConfig, CaptureReadiness, RosterMatch, ScannedDemo, SeriesDemo } from './types';
import type { SeriesSummary } from './jobs-index';
import type { PublishAssistant } from './publish-assistant';
import type { MusicChoice } from './reel-music.ts';

export type VideoReviewResolution =
  | {
      kind: 'rerender';
      editConfig: EditConfig;
      expectedArtifactPrefix: string;
      expectedWarnings: string[];
    }
  | {
      kind: 'accept';
      note: string;
      expectedArtifactPrefix: string;
      expectedWarnings: string[];
    };

export interface ApiClient {
  getCaptureReadiness(): Promise<CaptureReadiness>;
  listMatches(): Promise<Match[]>;
  listPlanReadyMatches(): Promise<Match[]>;
  listSeriesSummaries(): Promise<SeriesSummary[]>;
  getMatch(id: string): Promise<Match | null>;
  /** Pass `opts.seriesId` to tag a bo3/bo5 part. */
  scanDemo(file: File, opts?: { seriesId?: string }): Promise<{ jobId: string; players: DemoPlayer[]; match?: RosterMatch }>;
  getSeries(seriesId: string): Promise<SeriesDemo[]>;
  /** Resumes an existing job's POV pick: its status and roster; null when the job is gone (404). */
  getScan(jobId: string): Promise<ScannedDemo | null>;
  parseDemo(input: { jobId: string; steamId: string }): Promise<Match>;
  findClips(matchId: string): Promise<Play[]>;
  findRecapClips(matchId: string): Promise<Play[]>;
  listSongs(): Promise<Song[]>;
  listPresets(): Promise<Preset[]>;
  /** playIds must be in plan order, not click order. */
  createVideo(input: { matchId: string; playIds: string[]; mode: RenderMode; songId?: string; musicVolume?: number; gameVolume?: number; variant?: string; editConfig?: EditConfig }): Promise<Video>;
  listVideos(): Promise<Video[]>;
  getVideo(id: string): Promise<Video | null>;
  getPublishAssistant(id: string): Promise<PublishAssistant>;
  retryVideo(id: string): Promise<Video>;
  resolveVideoReview(id: string, resolution: VideoReviewResolution): Promise<Video>;
  rerenderVideoMusic(id: string, choice: MusicChoice): Promise<Video>;
  selectVideoCover(id: string, coverName: string): Promise<Video>;
  deleteVideo(id: string): Promise<void>;
  /** 404 is success; 409/503 throw. */
  deleteMatch(jobId: string): Promise<void>;
  deleteSeries(seriesId: string): Promise<void>;
  listFeed(): Promise<FeedItem[]>;
}
