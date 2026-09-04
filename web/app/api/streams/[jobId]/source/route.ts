import { NextResponse } from 'next/server';
import { streamJobUrl, proxyStream } from '../../_lib';
import { IMMUTABLE_CACHE_CONTROL } from '../../../demos/_lib';

export const runtime = 'nodejs';

/**
 * GET /api/streams/{jobId}/source — stream the job's source MP4 (the
 * acquired/uploaded video, before rendering) so the facecam picker can paint
 * a frame from it in a <video> element. Range-aware via proxyStream, which
 * mirrors the demos renders/videos proxy.
 *
 * The body is cacheable forever: the source lives at streamclips.SourceKey
 * under the job's own acquisition UUID, is written once, and the acquire
 * worker never overwrites it, so this URL's bytes cannot change. That matters
 * because the stream editor aims three media pipelines at this one URL (a
 * hidden shared decoder, the preview audio element, and per-row thumbnail
 * videos) and re-seeks it on every scrub and every playback tick; without a
 * cache policy each of those ranges is a fresh proxy round-trip. proxyStream
 * applies the header on the 2xx branch only, so a 404 served while the job is
 * still acquiring stays uncached.
 */
export async function GET(request: Request, { params }: { params: Promise<{ jobId: string }> }): Promise<Response> {
  const { jobId } = await params;
  const url = streamJobUrl(jobId, '/source');
  if (!url) return NextResponse.json({ error: 'invalid job id' }, { status: 400 });
  return proxyStream(url, 'video/mp4', request, IMMUTABLE_CACHE_CONTROL);
}
