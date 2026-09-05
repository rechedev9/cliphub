'use client';
import { useEffect, useState } from 'react';
import { Download, FolderOpen } from 'lucide-react';
import { streamsApi, type StreamVariant } from '@/lib/api/streams';
import { streamVideoFilename } from '@/lib/streams/download';
import { Button } from '@/components/ui/button';
type DownloadsBridge = { isSaved: (url: string) => Promise<boolean>; reveal: (url: string) => Promise<boolean> };
function bridge(): DownloadsBridge | undefined {
  return (window as unknown as { cliphubDownloads?: DownloadsBridge }).cliphubDownloads;
}
export function StreamSaveButton({
  jobId,
  variant,
  clipId,
  title,
  revision,
  prominent = false,
  disabled = false,
}: {
  jobId: string;
  variant: StreamVariant;
  clipId: string;
  title: string;
  revision?: string;
  prominent?: boolean;
  disabled?: boolean;
}) {
  const url = streamsApi.videoUrl(jobId, variant, clipId, revision);
  const [saved, setSaved] = useState(false);
  useEffect(() => {
    let active = true;
    setSaved(false);
    const api = bridge();
    if (!api || disabled) return;
    const refresh = () => {
      void api
        .isSaved(new URL(url, window.location.origin).href)
        .then((value) => {
          if (active) setSaved(value);
        })
        .catch(() => {});
    };
    refresh();
    const timer = setInterval(refresh, 1500);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [url, disabled]);
  return (
    <div className="flex flex-wrap gap-2">
      {disabled ? (
        <Button disabled>Guardar vídeo</Button>
      ) : (
        <Button asChild variant={prominent ? 'stream' : 'outline'}>
          <a href={url} download={streamVideoFilename(title)} aria-label={`Guardar vídeo: ${title}`}>
            <Download aria-hidden />
            Guardar vídeo
          </a>
        </Button>
      )}
      {saved && !disabled ? (
        <Button
          variant="outline"
          onClick={() => {
            void bridge()?.reveal(new URL(url, window.location.origin).href);
          }}
        >
          <FolderOpen aria-hidden />
          Abrir carpeta
        </Button>
      ) : null}
    </div>
  );
}
