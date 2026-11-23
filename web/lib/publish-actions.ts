import {
  YOUTUBE_STUDIO_URL,
  type PublishAssistant,
  type PublishRecommendation,
} from './api/publish-assistant.ts';
import type { VideoStatus } from './api/types.ts';
import { writeClipboardText } from './clipboard-write.ts';

export type PublishAssistantAvailability = 'ready' | 'waiting' | 'failed';

/** ClipHub does not upload: the action prepares the texts and the user uploads the MP4 in YouTube Studio. */
export const PUBLISH_PREP_LABEL = 'Preparar publicación';
export const PUBLISH_PREP_HINT = 'Prepara título, descripción y etiquetas; el MP4 lo subes tú en YouTube Studio.';

export const PUBLISH_ASSISTANT_WAITING_COPY =
  'La preparación para YouTube estará disponible cuando el vídeo esté listo.';
export const PUBLISH_ASSISTANT_FAILED_COPY =
  'No se pudo preparar la publicación porque el vídeo falló. Vuelve a montarlo desde Mis vídeos.';

/** Ready videos get the assistant. Failed videos stay failed. Everything else waits. */
export function publishAssistantAvailability(status: VideoStatus): PublishAssistantAvailability {
  if (status === 'ready') return 'ready';
  if (status === 'failed') return 'failed';
  return 'waiting';
}

export type PublishDraft = {
  title: string;
  description: string;
  tags: string[];
};

export function initialPublishDraft(assistant: PublishAssistant): PublishDraft {
  return {
    title: assistant.metadata.title,
    description: assistant.metadata.description,
    tags: [...assistant.metadata.tags],
  };
}

export function recommendedPublishDraft(recommendation: PublishRecommendation): PublishDraft {
  return {
    title: recommendation.title,
    description: recommendation.description,
    tags: [...recommendation.tags],
  };
}

export async function copyPublishText(value: string): Promise<void> {
  await writeClipboardText(value);
}

export function publishTagsText(tags: string[]): string {
  return tags.join(', ');
}

export function publishMP4FileName(title: string): string {
  const safeTitle = title.trim().replace(/[<>:"/\\|?*\u0000-\u001f]/g, '-') || 'cliphub-reel';
  return `${safeTitle}.mp4`;
}

export function downloadPublishMP4(url: string, title: string): void {
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = publishMP4FileName(title);
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}

export function openYouTubeStudio(): void {
  window.open(YOUTUBE_STUDIO_URL, '_blank', 'noopener,noreferrer');
}
