'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactElement } from 'react';
import { Button } from '@/components/ui/button';
import { editorApi, EDITOR_STATUS, type EditorAsset, type EditorRenderState } from '@/lib/api/editor';
import {
  defaultEditorDocument,
  documentDuration,
  evaluateTimeline,
  itemOutputDuration,
  itemTimelineEnd,
  normalizeDocument,
  type EditorDocument,
  type EditorItem,
} from '@/lib/editor/evaluate';

type EditorWorkspaceProps = { projectId: string };

export function EditorWorkspace({ projectId }: EditorWorkspaceProps): ReactElement {
  const [doc, setDoc] = useState<EditorDocument>(defaultEditorDocument());
  const [assets, setAssets] = useState<EditorAsset[]>([]);
  const [time, setTime] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [title, setTitle] = useState('Editor');
  const [error, setError] = useState<string | null>(null);
  const [render, setRender] = useState<EditorRenderState | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const saveTimer = useRef<number | null>(null);

  const sample = useMemo(() => evaluateTimeline(doc, time), [doc, time]);
  const duration = sample.duration;

  useEffect(() => {
    let cancelled = false;
    Promise.all([editorApi.getProject(projectId), editorApi.listAssets(), editorApi.getRender(projectId)])
      .then(([project, list, state]) => {
        if (cancelled) return;
        setTitle(project.title);
        if (project.plan !== undefined) setDoc(normalizeDocument(project.plan));
        setAssets(list);
        setRender(state);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'No se pudo abrir el proyecto.');
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  const persist = useCallback(
    (next: EditorDocument) => {
      setDoc(next);
      if (saveTimer.current !== null) window.clearTimeout(saveTimer.current);
      saveTimer.current = window.setTimeout(() => {
        void editorApi.putPlan(projectId, next).catch((err: unknown) => {
          setError(err instanceof Error ? err.message : 'No se pudo guardar el plan.');
        });
      }, 400);
    },
    [projectId],
  );

  useEffect(() => {
    if (!playing) return;
    let frame = 0;
    let last = performance.now();
    const tick = (now: number): void => {
      const dt = (now - last) / 1000;
      last = now;
      setTime((prev) => {
        const next = prev + dt;
        if (next >= duration) {
          setPlaying(false);
          return duration;
        }
        return next;
      });
      frame = window.requestAnimationFrame(tick);
    };
    frame = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frame);
  }, [playing, duration]);

  function addAssetToTimeline(asset: EditorAsset): void {
    const start = documentDuration(doc);
    const item: EditorItem = {
      id: `clip-${Date.now().toString(36)}`,
      asset_id: asset.id,
      timeline_start: start,
      source_in: 0,
      source_out: Math.max(0.2, asset.probe.duration_seconds ?? 2),
    };
    const tracks = doc.tracks.map((track, index) =>
      index === 0 ? { ...track, items: [...track.items, item] } : track,
    );
    persist({ ...doc, tracks });
    setSelected(item.id);
  }

  function updateSelected(mutate: (item: EditorItem) => EditorItem): void {
    if (selected === null) return;
    persist({
      ...doc,
      tracks: doc.tracks.map((track) => ({
        ...track,
        items: track.items.map((item) => (item.id === selected ? mutate(item) : item)),
      })),
    });
  }

  async function onUpload(file: File): Promise<void> {
    try {
      const asset = await editorApi.uploadAsset(file);
      setAssets((prev) => [asset, ...prev]);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'No se pudo subir el MP4.');
    }
  }

  async function onRender(): Promise<void> {
    try {
      const state = await editorApi.startRender(projectId);
      setRender(state);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'No se pudo lanzar el render.');
    }
  }

  useEffect(() => {
    if (render?.status !== EDITOR_STATUS.rendering) return;
    const timer = window.setInterval(() => {
      void editorApi.getRender(projectId).then(setRender);
    }, 1500);
    return () => window.clearInterval(timer);
  }, [projectId, render?.status]);

  const selectedItem = doc.tracks.flatMap((track) => track.items).find((item) => item.id === selected);

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <h1 className="font-display text-display-sm font-bold text-fg-1">{title}</h1>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => setPlaying((value) => !value)}>
            {playing ? 'Pausa' : 'Play'}
          </Button>
          <Button onClick={() => void onRender()} disabled={render?.status === EDITOR_STATUS.rendering}>
            Renderizar
          </Button>
        </div>
      </div>
      {error !== null ? <p className="text-body-sm text-destructive">{error}</p> : null}
      <div className="grid min-h-0 flex-1 grid-cols-[220px_minmax(0,1fr)_240px] gap-3">
        <aside className="flex flex-col gap-3 overflow-auto rounded-lg border border-border bg-surface-2 p-3">
          <label className="text-meta text-fg-3">
            Subir MP4
            <input
              type="file"
              accept="video/mp4"
              className="mt-1 block w-full text-meta"
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) void onUpload(file);
              }}
            />
          </label>
          <ul className="grid gap-2">
            {assets.map((asset) => (
              <li key={asset.id}>
                <button
                  type="button"
                  className="w-full rounded-md border border-border px-2 py-2 text-left text-meta hover:bg-surface-2"
                  onClick={() => addAssetToTimeline(asset)}
                >
                  {asset.file_name}
                </button>
              </li>
            ))}
          </ul>
        </aside>
        <section className="flex min-h-0 flex-col gap-3">
          <div className="relative mx-auto aspect-[9/16] max-h-[52vh] overflow-hidden rounded-lg border border-border bg-black">
            {sample.layers.map((layer) => (
              <video
                key={layer.item_id}
                src={editorApi.assetMediaUrl(layer.asset_id)}
                className="absolute"
                style={{
                  left: `${layer.transform.x * 100}%`,
                  top: `${layer.transform.y * 100}%`,
                  width: `${layer.transform.width * 100}%`,
                  height: `${layer.transform.height * 100}%`,
                  opacity: layer.opacity,
                  objectFit: 'cover',
                }}
                muted
                playsInline
                ref={(node) => {
                  if (node && Math.abs(node.currentTime - layer.source_time) > 0.08) {
                    node.currentTime = layer.source_time;
                  }
                }}
              />
            ))}
            {sample.texts.map((text) => (
              <div
                key={text.id}
                className="pointer-events-none absolute left-0 right-0 text-center font-bold text-white"
                style={{ top: `${text.position_y * 100}%`, fontSize: text.font_size }}
              >
                {text.text}
              </div>
            ))}
          </div>
          <div className="rounded-lg border border-border bg-surface-2 p-3">
            <input
              type="range"
              min={0}
              max={Math.max(duration, 0.1)}
              step={0.01}
              value={time}
              className="w-full"
              onChange={(event) => setTime(Number(event.target.value))}
            />
            <div className="mt-3 grid gap-2">
              {doc.tracks.map((track) => (
                <div key={track.id} className="relative h-10 rounded bg-surface-2">
                  {track.items.map((item) => {
                    const left = duration === 0 ? 0 : (item.timeline_start / Math.max(duration, 0.1)) * 100;
                    const width = duration === 0 ? 8 : (itemOutputDuration(item) / Math.max(duration, 0.1)) * 100;
                    return (
                      <button
                        key={item.id}
                        type="button"
                        className={`absolute top-1 h-8 rounded px-2 text-left text-meta ${
                          selected === item.id ? 'bg-primary text-primary-foreground' : 'bg-fg-3/40 text-fg-1'
                        }`}
                        style={{ left: `${left}%`, width: `${Math.max(width, 4)}%` }}
                        onClick={() => setSelected(item.id)}
                      >
                        {item.id}
                      </button>
                    );
                  })}
                </div>
              ))}
            </div>
          </div>
        </section>
        <aside className="overflow-auto rounded-lg border border-border bg-surface-2 p-3 text-meta text-fg-2">
          {selectedItem === undefined ? (
            <p>Selecciona un clip del timeline.</p>
          ) : (
            <div className="grid gap-2">
              <label>
                Inicio
                <input
                  type="number"
                  className="mt-1 w-full rounded border border-border bg-background px-2 py-1"
                  value={selectedItem.timeline_start}
                  step={0.05}
                  onChange={(event) => updateSelected((item) => ({ ...item, timeline_start: Number(event.target.value) }))}
                />
              </label>
              <label>
                Source in
                <input
                  type="number"
                  className="mt-1 w-full rounded border border-border bg-background px-2 py-1"
                  value={selectedItem.source_in}
                  step={0.05}
                  onChange={(event) => updateSelected((item) => ({ ...item, source_in: Number(event.target.value) }))}
                />
              </label>
              <label>
                Source out
                <input
                  type="number"
                  className="mt-1 w-full rounded border border-border bg-background px-2 py-1"
                  value={selectedItem.source_out}
                  step={0.05}
                  onChange={(event) => updateSelected((item) => ({ ...item, source_out: Number(event.target.value) }))}
                />
              </label>
              <label>
                Velocidad
                <input
                  type="number"
                  className="mt-1 w-full rounded border border-border bg-background px-2 py-1"
                  value={selectedItem.speed ?? 1}
                  min={0.25}
                  max={3}
                  step={0.05}
                  onChange={(event) => updateSelected((item) => ({ ...item, speed: Number(event.target.value) }))}
                />
              </label>
              <p className="text-fg-3">Fin en {itemTimelineEnd(selectedItem).toFixed(2)}s</p>
            </div>
          )}
          {render !== null ? (
            <div className="mt-4 border-t border-border pt-3">
              <p>Render: {render.status}</p>
              {render.status === EDITOR_STATUS.rendered ? (
                <video className="mt-2 w-full" src={editorApi.videoUrl(projectId)} controls />
              ) : null}
              {render.warnings?.map((warning) => (
                <p key={warning} className="mt-1 text-warning">
                  {warning}
                </p>
              ))}
              {render.error !== undefined && render.error !== '' ? <p className="text-destructive">{render.error}</p> : null}
            </div>
          ) : null}
        </aside>
      </div>
    </div>
  );
}
