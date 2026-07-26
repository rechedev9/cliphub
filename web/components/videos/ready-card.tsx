'use client';

import { useState } from 'react';
import { Clock, Download, Eye, Share2, Youtube } from 'lucide-react';
import { toast } from 'sonner';
import type { Video } from '@/lib/api/types';
import { formatCountdown } from '@/lib/format';
import { downloadPublishMP4 } from '@/lib/publish-actions';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { StatusTag } from '@/components/studio/status-tag';
import { DeleteVideoButton } from '@/components/videos/delete-video-button';
import { PublishAssistantDialog } from '@/components/videos/publish-assistant-dialog';
import { ReelCard, reelFormatLabel } from '@/components/videos/reel-card';

/**
 * A finished, downloadable reel. The card is the shared `ReelCard` in its payoff
 * state: raised off the grid, cyan-edged, its cover finally rendered at the
 * format it was actually made in, and its stage track filled to LISTO.
 *
 * Hovering (or focusing, or touching) the frame surfaces Ver/Compartir. There is
 * no thumbnail-duration data, so the corner tag shows the render format and
 * nothing else — the shape of the frame now carries most of that information
 * anyway. Ver plays the reel inline in a dialog.
 */
export function ReadyCard({
  video,
  onDeleted,
}: {
  video: Video;
  onDeleted?: () => void;
}) {
  const [publishOpen, setPublishOpen] = useState(false);
  const [playerOpen, setPlayerOpen] = useState(false);

  const handleDownload = () => {
    if (!video.downloadUrl) return;
    downloadPublishMP4(video.downloadUrl, video.title);
  };

  // In cloud mode the reel's media is a DOM object URL (blob:) fetched through the
  // Bearer-gated loopback: it lives and dies with this tab, so there is no
  // persistent URL to share. Hide Share entirely there rather than copy a link
  // that dies with the tab. Download and inline playback still work with blob:.
  const canShare = video.downloadUrl != null && !video.downloadUrl.startsWith('blob:');

  const handleShare = async () => {
    if (!video.downloadUrl) return;
    const url = new URL(video.downloadUrl, window.location.origin).href;
    try {
      if (typeof navigator !== 'undefined' && navigator.share) {
        await navigator.share({ title: video.title, url });
        return;
      }
    } catch {
      // user dismissed the share sheet, or it failed — fall through to copy.
    }
    try {
      await navigator.clipboard.writeText(url);
      toast('Enlace copiado al portapapeles.');
    } catch {
      toast('No se pudo copiar el enlace.');
    }
  };

  const matchMeta = video.score ? `${video.map} · ${video.score}` : video.map;
  const meta = video.targetName ? `POV ${video.targetName} · ${matchMeta}` : matchMeta;
  const formatBadge = reelFormatLabel(video.editConfig);

  return (
    <>
      <ReelCard
        video={video}
        tone="primary"
        raised
        scrim
        badge={formatBadge ? <StatusTag tone="primary">{formatBadge}</StatusTag> : undefined}
        frameActions={
          <>
            <Button
              type="button"
              variant="outline-primary"
              size="sm"
              onClick={() => video.downloadUrl && setPlayerOpen(true)}
              disabled={!video.downloadUrl}
            >
              <Eye className="size-4" aria-hidden /> Ver
            </Button>
            {canShare ? (
              <Button type="button" variant="outline" size="sm" onClick={handleShare}>
                <Share2 className="size-4" aria-hidden /> Compartir
              </Button>
            ) : null}
          </>
        }
        footer={
          <div className="flex flex-col gap-2 p-4">
            {/* Full width, and allowed to wrap rather than being starved into a
                112px column by a three-track footer grid. */}
            <Button
              type="button"
              variant="hero"
              className="h-auto min-h-11 w-full whitespace-normal px-3 py-2.5 text-center leading-tight"
              onClick={() => setPublishOpen(true)}
            >
              <Youtube className="size-4" aria-hidden /> PREPARAR PUBLICACIÓN
            </Button>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline-primary"
                size="sm"
                className="flex-1"
                onClick={handleDownload}
                disabled={!video.downloadUrl}
              >
                <Download className="size-4" aria-hidden /> MP4
              </Button>
              <DeleteVideoButton video={video} onDeleted={() => onDeleted?.()} />
            </div>
          </div>
        }
      >
        <div className="flex flex-wrap items-center gap-2">
          <StatusTag tone="success" dot>
            Listo
          </StatusTag>
          {video.availableForSec !== undefined ? (
            <StatusTag tone="warning" icon={Clock}>
              caduca en <span className="tabular-nums">{formatCountdown(video.availableForSec)}</span>
            </StatusTag>
          ) : null}
        </div>
      </ReelCard>

      <Dialog open={playerOpen} onOpenChange={setPlayerOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="truncate">{video.title}</DialogTitle>
            <DialogDescription className="font-mono tabular-nums">{meta}</DialogDescription>
          </DialogHeader>
          {video.downloadUrl ? (
            <video
              src={video.downloadUrl}
              controls
              autoPlay
              playsInline
              preload="metadata"
              className="mx-auto max-h-[72vh] w-auto rounded-lg bg-surface-0"
            />
          ) : null}
        </DialogContent>
      </Dialog>

      <PublishAssistantDialog open={publishOpen} video={video} onOpenChange={setPublishOpen} />
    </>
  );
}
