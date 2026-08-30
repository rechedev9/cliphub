'use client';

import { useCallback, useEffect, useRef, useState, type ReactElement } from 'react';
import { EditorAssetPanel } from '@/components/editor/asset-panel';
import { EditorInspector } from '@/components/editor/inspector';
import { EditorOverlaysPanel } from '@/components/editor/overlays-panel';
import { EditorPreviewPlayer } from '@/components/editor/preview-player';
import { EditorTimeline } from '@/components/editor/timeline';
import { CoverImage } from '@/components/studio/cover-image';
import { LongOperation } from '@/components/studio/long-operation';
import { MediaFrame } from '@/components/studio/media-frame';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { api } from '@/lib/api';
import { editorApi, EDITOR_STATUS, type EditorAsset, type EditorRenderState } from '@/lib/api/editor';
import { addItem } from '@/lib/editor/document';
import { editorUserMessage } from '@/lib/editor/errors';
import {
  defaultEditorDocument,
  documentDuration,
  EDITOR_TRACK_KINDS,
  normalizeDocument,
  type EditorDocument,
} from '@/lib/editor/evaluate';
import { createPlanStore, type PlanStore, type PlanStoreState } from '@/lib/editor/plan-store';
import { validateForRender } from '@/lib/editor/validate';

type Phase =
  | { kind: 'loading' }
  | { kind: 'ready' }
  | { kind: 'rendering' }
  | { kind: 'rendered' }
  | { kind: 'failed'; error: string };

type EditorWorkspaceProps = { projectId: string };

const EMPTY_SONGS: ReadonlyArray<{ id: string; title: string }> = [];
const RENDER_POLL_MS = 1500;
const STORE_POLL_MS = 200;

export function EditorWorkspace({ projectId }: EditorWorkspaceProps): ReactElement {
  const [phase, setPhase] = useState<Phase>({ kind: 'loading' });
  const [doc, setDoc] = useState<EditorDocument>(defaultEditorDocument);
  const [assets, setAssets] = useState<EditorAsset[]>([]);
  const [time, setTime] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [title, setTitle] = useState('Editor');
  const [songs, setSongs] = useState(EMPTY_SONGS);
  const [render, setRender] = useState<EditorRenderState | null>(null);
  const store = usePlanStore(projectId);
  const [save, setSave] = useState<PlanStoreState>(() => store.getState());
  const duration = documentDuration(doc);
  const locked = phase.kind === 'rendering';
  const renderIssues = validateForRender(doc);
  const renderDisabled = renderIssues.length > 0 || phase.kind === 'rendering';
  const renderHint =
    renderIssues[0] !== undefined ? editorUserMessage(new Error(renderIssues[0])) : 'Añade un clip al timeline';

  const persist = useCallback(
    (next: EditorDocument): void => {
      if (store.getState().locked) return;
      setDoc(next);
      store.update(next);
      setSave(store.getState());
    },
    [store],
  );

  const handleEnded = useCallback((): void => {
    setPlaying(false);
    setTime(duration);
  }, [duration]);

  useEffect(() => {
    const id = window.setInterval(() => {
      const next = store.getState();
      setSave((prev) =>
        prev.dirty === next.dirty && prev.saving === next.saving && prev.lastError === next.lastError && prev.locked === next.locked
          ? prev
          : next,
      );
    }, STORE_POLL_MS);
    return () => window.clearInterval(id);
  }, [store]);

  useEffect(() => {
    let cancelled = false;
    Promise.all([
      editorApi.getProject(projectId),
      editorApi.listAssets(),
      editorApi.getRender(projectId),
      api.listSongs().catch(() => []),
    ])
      .then(([project, list, state, catalog]) => {
        if (cancelled) return;
        setTitle(project.title);
        if (project.plan !== undefined) setDoc(normalizeDocument(project.plan));
        setAssets(list);
        setRender(state);
        setSongs(catalog.map((song) => ({ id: song.id, title: song.title })));
        setPhase(phaseFromRender(state));
      })
      .catch((err: unknown) => {
        if (!cancelled) setPhase({ kind: 'failed', error: editorUserMessage(err) });
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

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

  useEffect(() => {
    if (phase.kind !== 'rendering') return;
    let cancelled = false;
    const poll = async (): Promise<void> => {
      try {
        const state = await editorApi.getRender(projectId);
        if (cancelled) return;
        setRender(state);
        if (state.status === EDITOR_STATUS.rendered) {
          store.unlock();
          setPhase({ kind: 'rendered' });
          setSave(store.getState());
        } else if (state.status === EDITOR_STATUS.failed) {
          store.unlock();
          setPhase({ kind: 'failed', error: renderErrorMessage(state) });
          setSave(store.getState());
        }
      } catch (err: unknown) {
        if (!cancelled) setError(editorUserMessage(err));
      }
    };
    void poll();
    const timer = window.setInterval(() => {
      void poll();
    }, RENDER_POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [phase.kind, projectId, store]);

  async function onUpload(file: File): Promise<void> {
    try {
      const asset = await editorApi.uploadAsset(file);
      setAssets((prev) => [asset, ...prev]);
    } catch (err: unknown) {
      setError(editorUserMessage(err));
    }
  }

  function addAssetToTimeline(asset: EditorAsset): void {
    const trackId = doc.tracks.find((track) => track.kind === EDITOR_TRACK_KINDS.video)?.id;
    if (trackId === undefined) return;
    const next = addItem(doc, trackId, asset);
    persist(next);
    const items = next.tracks.find((track) => track.id === trackId)?.items ?? [];
    const added = items[items.length - 1];
    if (added !== undefined) setSelected(added.id);
  }

  async function retrySave(): Promise<void> {
    try {
      await store.flush();
      setSave(store.getState());
    } catch (err: unknown) {
      setError(editorUserMessage(err));
    }
  }

  async function onRender(): Promise<void> {
    if (renderDisabled) return;
    setError(null);
    try {
      await store.flush();
      setSave(store.getState());
      const lastError = store.getState().lastError;
      if (lastError !== null) {
        setError(editorUserMessage(new Error(lastError)));
        return;
      }
      store.lock();
      setPlaying(false);
      setPhase({ kind: 'rendering' });
      setSave(store.getState());
      const state = await editorApi.startRender(projectId);
      setRender(state);
      if (state.status === EDITOR_STATUS.rendered) {
        store.unlock();
        setPhase({ kind: 'rendered' });
        setSave(store.getState());
      } else if (state.status === EDITOR_STATUS.failed) {
        store.unlock();
        setPhase({ kind: 'failed', error: renderErrorMessage(state) });
        setSave(store.getState());
      }
    } catch (err: unknown) {
      store.unlock();
      setPhase({ kind: 'failed', error: editorUserMessage(err) });
      setSave(store.getState());
    }
  }

  const aspect = doc.canvas.width > doc.canvas.height ? '16:9' : '9:16';

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <StudioPageHeader
        title={title}
        description={<SaveStatus save={save} onRetry={() => void retrySave()} />}
        actions={
          <div className="flex gap-2">
            <Button type="button" variant="outline" onClick={() => setPlaying((value) => !value)} disabled={phase.kind === 'loading'}>
              {playing ? 'Pausa' : 'Play'}
            </Button>
            <Button
              type="button"
              onClick={() => void onRender()}
              disabled={renderDisabled || phase.kind === 'loading'}
              title={renderDisabled ? renderHint : undefined}
              loading={phase.kind === 'rendering'}
            >
              Renderizar
            </Button>
          </div>
        }
      />
      {error !== null ? (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
      {phase.kind === 'loading' ? (
        <p className="text-body text-fg-2">Cargando proyecto…</p>
      ) : (
        <div className="grid min-h-0 flex-1 grid-cols-[220px_minmax(0,1fr)_260px] gap-3">
          <EditorAssetPanel
            assets={assets}
            locked={locked}
            onUpload={onUpload}
            onAddToTimeline={addAssetToTimeline}
            onImported={(asset) => setAssets((prev) => [asset, ...prev.filter((entry) => entry.id !== asset.id)])}
          />
          <section className="flex min-h-0 flex-col gap-3">
            {phase.kind === 'rendering' ? <LongOperation stage="RENDERIZANDO" progress={render?.progress} /> : null}
            <EditorPreviewPlayer doc={doc} time={time} playing={playing} onTime={setTime} onEnded={handleEnded} />
            {phase.kind === 'rendered' ? (
              <div className="grid gap-2">
                <MediaFrame
                  aspect={aspect}
                  capHeight
                  className="bg-black"
                  media={<video src={editorApi.videoUrl(projectId)} controls playsInline />}
                />
                <CoverImage src={editorApi.coverUrl(projectId)} className="max-h-32 w-full object-cover" />
                {render?.warnings?.map((warning) => (
                  <StatusTag key={warning} tone="warning">
                    {warning}
                  </StatusTag>
                ))}
              </div>
            ) : null}
            {phase.kind === 'failed' ? (
              <p role="alert" className="text-body-sm text-destructive">
                {phase.error}
              </p>
            ) : null}
            <EditorTimeline
              doc={doc}
              time={time}
              selectedId={selected}
              locked={locked}
              onSeek={(next) => {
                setTime(next);
                if (next >= duration) setPlaying(false);
              }}
              onSelect={setSelected}
              onChange={persist}
            />
          </section>
          <div className="flex min-h-0 flex-col gap-3 overflow-auto">
            <EditorInspector doc={doc} selectedId={selected} locked={locked} songs={songs} onChange={persist} />
            <EditorOverlaysPanel doc={doc} locked={locked} onChange={persist} />
          </div>
        </div>
      )}
    </div>
  );
}

function usePlanStore(projectId: string): PlanStore {
  const ref = useRef<{ id: string; store: PlanStore } | null>(null);
  if (ref.current === null || ref.current.id !== projectId) {
    ref.current = {
      id: projectId,
      store: createPlanStore({
        putPlan: (plan) => editorApi.putPlan(projectId, plan),
      }),
    };
  }
  return ref.current.store;
}

function phaseFromRender(state: EditorRenderState): Phase {
  if (state.status === EDITOR_STATUS.rendering) return { kind: 'rendering' };
  if (state.status === EDITOR_STATUS.rendered) return { kind: 'rendered' };
  if (state.status === EDITOR_STATUS.failed) return { kind: 'failed', error: renderErrorMessage(state) };
  return { kind: 'ready' };
}

function renderErrorMessage(state: EditorRenderState): string {
  if (state.error !== undefined && state.error !== '') return editorUserMessage(new Error(state.error));
  return 'El render falló.';
}

function SaveStatus({ save, onRetry }: { save: PlanStoreState; onRetry: () => void }): ReactElement {
  if (save.lastError !== null) {
    return (
      <span className="flex items-center gap-2">
        <StatusTag tone="danger">Error al guardar</StatusTag>
        <Button type="button" size="sm" variant="outline" onClick={onRetry}>
          Reintentar
        </Button>
      </span>
    );
  }
  if (save.dirty || save.saving) {
    return <StatusTag tone="primary">Guardando…</StatusTag>;
  }
  return <StatusTag tone="success">Guardado</StatusTag>;
}
