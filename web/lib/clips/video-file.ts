import type { StorageFile } from '../api/storage.ts';

export type RenderFileRef = { jobId: string; variant: string; artifactName: string; revision: string };

/** The listed MP4 of a demo render, under jobs/<job>/renders/<variant>/…/videos/<name>; its own revision first. */
export function renderVideoFile(files: readonly StorageFile[], ref: RenderFileRef): StorageFile | undefined {
  const matches = files.filter((file) => {
    if (file.jobId !== ref.jobId) return false;
    const parts = file.path.split(/[\\/]/);
    const renders = parts.lastIndexOf('renders');
    return renders >= 0 && parts[renders + 1] === ref.variant && parts.at(-2) === 'videos' && parts.at(-1) === ref.artifactName;
  });
  return matches.find((file) => file.path.split(/[\\/]/).includes(ref.revision)) ?? matches[0];
}
