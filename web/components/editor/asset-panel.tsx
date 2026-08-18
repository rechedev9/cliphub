'use client';

import { useEffect, useState, type ReactElement } from 'react';
import { Film, Library } from 'lucide-react';
import { StatusTag } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { editorApi, type EditorAsset } from '@/lib/api/editor';
import { IMPORT_SOURCE, listImportableRenders, type ImportableRender } from '@/lib/api/editor-library';
import { editorUserMessage } from '@/lib/editor/errors';
import { cn } from '@/lib/utils';

const ASSET_SECTION = {
  upload: 'upload',
  library: 'library',
} as const;

type AssetSection = (typeof ASSET_SECTION)[keyof typeof ASSET_SECTION];

export function EditorAssetPanel({
  assets,
  locked,
  onUpload,
  onAddToTimeline,
  onImported,
}: {
  assets: EditorAsset[];
  locked: boolean;
  onUpload: (file: File) => Promise<void>;
  onAddToTimeline: (asset: EditorAsset) => void;
  onImported: (asset: EditorAsset) => void;
}): ReactElement {
  const [section, setSection] = useState<AssetSection>(ASSET_SECTION.upload);
  const [library, setLibrary] = useState<ImportableRender[]>([]);
  const [libraryError, setLibraryError] = useState<string | null>(null);
  const [importingKey, setImportingKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listImportableRenders()
      .then((list) => {
        if (!cancelled) setLibrary(list);
      })
      .catch((err: unknown) => {
        if (!cancelled) setLibraryError(editorUserMessage(err));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function importEntry(entry: ImportableRender): Promise<void> {
    const key = importKey(entry);
    setImportingKey(key);
    setError(null);
    try {
      const asset = await editorApi.importAsset({
        source: entry.source,
        job_id: entry.job_id,
        variant: entry.variant,
        name: entry.name,
      });
      onImported(asset);
    } catch (err: unknown) {
      setError(editorUserMessage(err));
    } finally {
      setImportingKey(null);
    }
  }

  return (
    <aside className="studio-panel flex min-h-0 min-w-0 flex-col gap-3 overflow-auto p-3">
      <div className="grid grid-cols-2 gap-1">
        <Button
          type="button"
          size="sm"
          variant={section === ASSET_SECTION.upload ? 'outline-primary' : 'ghost'}
          aria-pressed={section === ASSET_SECTION.upload}
          onClick={() => setSection(ASSET_SECTION.upload)}
        >
          Subir MP4
        </Button>
        <Button
          type="button"
          size="sm"
          variant={section === ASSET_SECTION.library ? 'outline-primary' : 'ghost'}
          aria-pressed={section === ASSET_SECTION.library}
          onClick={() => setSection(ASSET_SECTION.library)}
        >
          Biblioteca
        </Button>
      </div>
      {error !== null ? (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
      {section === ASSET_SECTION.upload ? (
        <div className="flex flex-col gap-3">
          <label className="text-meta text-fg-3">
            Subir MP4
            <input
              type="file"
              accept="video/mp4"
              disabled={locked}
              className="mt-1 block w-full text-meta"
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) void onUpload(file);
                event.target.value = '';
              }}
            />
          </label>
          {assets.length === 0 ? (
            <p className="flex items-start gap-2 text-body-sm text-fg-3">
              <Film className="mt-0.5 size-4 shrink-0" aria-hidden />
              No hay MP4s. Sube uno o impórtalo de la biblioteca.
            </p>
          ) : (
            <ul className="grid gap-2">
              {assets.map((asset) => (
                <li key={asset.id}>
                  <button
                    type="button"
                    disabled={locked}
                    className={cn(
                      'w-full rounded-md border border-border-strong bg-surface-3 px-2 py-2 text-left text-meta text-fg-1 hover:bg-surface-4 disabled:opacity-50',
                      FOCUS_RING,
                    )}
                    onClick={() => onAddToTimeline(asset)}
                  >
                    <span className="block truncate">{asset.file_name}</span>
                    {asset.probe.duration_seconds !== undefined ? (
                      <span className="font-mono text-fg-3">{asset.probe.duration_seconds.toFixed(1)}s</span>
                    ) : null}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {libraryError !== null ? (
            <p role="alert" className="text-body-sm text-destructive">
              {libraryError}
            </p>
          ) : null}
          {library.length === 0 && libraryError === null ? (
            <p className="flex items-start gap-2 text-body-sm text-fg-3">
              <Library className="mt-0.5 size-4 shrink-0" aria-hidden />
              No hay renders importables.
            </p>
          ) : (
            <ul className="grid gap-2">
              {library.map((entry) => {
                const key = importKey(entry);
                return (
                  <li key={key} className="flex flex-col gap-2 rounded-md border border-border bg-surface-3 p-2">
                    <div className="flex items-start justify-between gap-2">
                      <span className="min-w-0 truncate text-meta text-fg-1">{entry.title}</span>
                      <StatusTag tone={entry.source === IMPORT_SOURCE.stream ? 'stream' : 'primary'} size="sm">
                        {entry.source === IMPORT_SOURCE.stream ? 'Stream' : 'Demo'}
                      </StatusTag>
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      disabled={locked || importingKey === key}
                      loading={importingKey === key}
                      onClick={() => void importEntry(entry)}
                    >
                      Importar
                    </Button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}
    </aside>
  );
}

function importKey(entry: ImportableRender): string {
  return `${entry.source}:${entry.job_id}:${entry.variant ?? ''}:${entry.name ?? ''}`;
}
