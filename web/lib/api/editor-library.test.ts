import assert from 'node:assert/strict';
import test from 'node:test';
import {
  IMPORT_SOURCE,
  mapImportableRenders,
  parseDemoVideoRef,
  type DemoLibraryVideo,
  type StreamLibraryJob,
} from './editor-library.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const STREAM_JOB = '22222222-2222-4222-8222-222222222222';
const VARIANT = 'viral-60-clean';
const STREAM_VARIANT = 'streamer-vertical-stack-40-60';

function video(overrides: Partial<DemoLibraryVideo> & Pick<DemoLibraryVideo, 'title' | 'status'>): DemoLibraryVideo {
  return {
    jobId: overrides.jobId,
    title: overrides.title,
    variant: overrides.variant,
    status: overrides.status,
    downloadUrl: overrides.downloadUrl,
  };
}

function streamJob(overrides: Partial<StreamLibraryJob> & Pick<StreamLibraryJob, 'id' | 'status'>): StreamLibraryJob {
  return {
    id: overrides.id,
    status: overrides.status,
    title: overrides.title,
    edit_plan: overrides.edit_plan,
  };
}

test('parseDemoVideoRef reads job, variant, and artifact name from the library URL', () => {
  const cases: { url: string; want: { job_id: string; variant: string; name: string } | null }[] = [
    {
      url: `/api/demos/${JOB}/renders/${VARIANT}/videos/ace`,
      want: { job_id: JOB, variant: VARIANT, name: 'ace' },
    },
    {
      url: `/api/demos/${JOB}/renders/${VARIANT}/videos/demo-compilation`,
      want: { job_id: JOB, variant: VARIANT, name: 'demo-compilation' },
    },
    { url: '/reel-sample.mp4', want: null },
    { url: `/api/demos/${JOB}/renders/${VARIANT}/covers/ace`, want: null },
    { url: `/api/streams/${STREAM_JOB}/renders/${STREAM_VARIANT}/videos/clip-1`, want: null },
  ];
  for (const tc of cases) {
    assert.deepEqual(parseDemoVideoRef(tc.url), tc.want, tc.url);
  }
});

test('mapImportableRenders keeps finished demo and stream videos only', () => {
  const cases: {
    name: string;
    videos?: DemoLibraryVideo[];
    streamJobs?: StreamLibraryJob[];
    want: ReturnType<typeof mapImportableRenders>;
  }[] = [
    {
      name: 'ready demo with proxy video',
      videos: [
        video({
          title: 'Ace reel',
          status: 'ready',
          jobId: JOB,
          variant: VARIANT,
          downloadUrl: `/api/demos/${JOB}/renders/${VARIANT}/videos/ace`,
        }),
      ],
      want: [{ source: IMPORT_SOURCE.demo, job_id: JOB, variant: VARIANT, name: 'ace', title: 'Ace reel' }],
    },
    {
      name: 'review_required demo still has a video',
      videos: [
        video({
          title: 'Needs review',
          status: 'review_required',
          downloadUrl: `/api/demos/${JOB}/renders/${VARIANT}/videos/hs`,
        }),
      ],
      want: [{ source: IMPORT_SOURCE.demo, job_id: JOB, variant: VARIANT, name: 'hs', title: 'Needs review' }],
    },
    {
      name: 'queued demo is skipped',
      videos: [video({ title: 'Queued', status: 'queued', jobId: JOB, downloadUrl: `/api/demos/${JOB}/renders/${VARIANT}/videos/ace` })],
      want: [],
    },
    {
      name: 'ready demo without a parseable video is skipped',
      videos: [video({ title: 'Seed', status: 'ready', jobId: JOB, downloadUrl: '/reel-sample.mp4' })],
      want: [],
    },
    {
      name: 'rendered stream emits one row per clip',
      streamJobs: [
        streamJob({
          id: STREAM_JOB,
          status: 'rendered',
          title: 'VOD',
          edit_plan: {
            schema_version: '1',
            variant: STREAM_VARIANT,
            clips: [
              { id: 'clip-1', start_seconds: 0, end_seconds: 8, title: 'Entry' },
              { id: 'clip-2', start_seconds: 10, end_seconds: 18 },
            ],
          },
        }),
      ],
      want: [
        { source: IMPORT_SOURCE.stream, job_id: STREAM_JOB, variant: STREAM_VARIANT, name: 'clip-1', title: 'Entry' },
        { source: IMPORT_SOURCE.stream, job_id: STREAM_JOB, variant: STREAM_VARIANT, name: 'clip-2', title: 'VOD' },
      ],
    },
    {
      name: 'unrendered stream is skipped',
      streamJobs: [
        streamJob({
          id: STREAM_JOB,
          status: 'ready',
          edit_plan: {
            schema_version: '1',
            variant: STREAM_VARIANT,
            clips: [{ id: 'clip-1', start_seconds: 0, end_seconds: 8 }],
          },
        }),
      ],
      want: [],
    },
    {
      name: 'rendered stream without clips is skipped',
      streamJobs: [streamJob({ id: STREAM_JOB, status: 'rendered', title: 'Empty', edit_plan: { schema_version: '1', variant: STREAM_VARIANT, clips: [] } })],
      want: [],
    },
  ];

  for (const tc of cases) {
    assert.deepEqual(mapImportableRenders({ videos: tc.videos, streamJobs: tc.streamJobs }), tc.want, tc.name);
  }
});
