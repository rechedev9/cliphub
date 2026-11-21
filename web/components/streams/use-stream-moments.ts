import { useRef, useState } from 'react';
import { toast } from 'sonner';
import type { StreamClipRange, StreamEditPlan, StreamJob, StreamPlay } from '@/lib/api/streams';
import { streamRangeIssue, streamRangeOverlapIssue } from '@/lib/clip-edit';
import { pruneStreamGroups } from '@/lib/streams/groups';
import { moveMomentEdge, nudgeMomentEdge, suggestedCut, withMomentRange, type MomentEdge } from '@/lib/streams/moment-edges';
import { removeMoments, undoRemoval, withoutMoments } from '@/lib/streams/moments';
import { formatStreamClock, insertClipSorted, nextClipId } from '@/lib/streams/plan';
import { clipsWithPlays } from '@/lib/streams/plays';
import { streamTitle } from '@/lib/streams/title';

type MomentRangeValue = { start_seconds: number; end_seconds: number };

export function useStreamMoments({
  job,
  plan,
  sourceDuration,
  onPlanChange,
  focusClip,
  clearSelection,
  showDraft,
  showEdge,
  showWholeVideo,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  sourceDuration: number;
  onPlanChange: (plan: StreamEditPlan) => void;
  focusClip: (clip: StreamClipRange) => void;
  clearSelection: () => void;
  showDraft: () => void;
  showEdge: (seconds: number) => void;
  showWholeVideo: () => void;
}) {
  const [draftRange, setDraftRange] = useState<MomentRangeValue>({
    start_seconds: 0,
    end_seconds: Math.min(sourceDuration, 30),
  });
  const [creatingMoment, setCreatingMoment] = useState(plan.clips.length === 0);
  const latestPlanRef = useRef(plan);
  latestPlanRef.current = plan;

  const setClips = (clips: StreamClipRange[]) => onPlanChange(pruneStreamGroups({ ...plan, clips }));
  const selectClip = (clip: StreamClipRange) => {
    setCreatingMoment(false);
    focusClip(clip);
  };
  const draftClip: StreamClipRange = { id: 'draft', ...draftRange };
  const draftCandidates = insertClipSorted(plan.clips, draftClip);
  const draftIndex = draftCandidates.findIndex((c) => c.id === 'draft');
  const draftIssue =
    streamRangeIssue(draftClip, sourceDuration, draftIndex) ?? streamRangeOverlapIssue(draftCandidates, draftIndex);
  const addMoment = (range = draftRange) => {
    const sourceTitle = streamTitle(job);
    const title = plan.clips.length ? `${sourceTitle} ${plan.clips.length + 1}` : sourceTitle;
    const clip: StreamClipRange = { id: nextClipId(), ...range, title };
    const next = insertClipSorted(plan.clips, clip);
    const index = next.findIndex((c) => c.id === clip.id);
    const issue = streamRangeIssue(clip, sourceDuration, index) ?? streamRangeOverlapIssue(next, index);
    if (issue) {
      toast.error(issue);
      return;
    }
    setClips(next);
    selectClip(clip);
  };
  const addPlays = (plays: StreamPlay[]) => {
    const next = clipsWithPlays(plan.clips, plays, nextClipId);
    const added = next.filter((clip) => !plan.clips.some((c) => c.id === clip.id));
    const [first] = added;
    if (!first) return;
    setClips(next);
    const addedIds = added.map((clip) => clip.id);
    toast(added.length === 1 ? `Añadida: ${first.title ?? 'jugada'} (${formatStreamClock(first.start_seconds)})` : `${added.length} jugadas añadidas a tus momentos`, {
      position: 'top-right',
      action: {
        label: 'Deshacer',
        onClick: () => {
          const current = latestPlanRef.current;
          onPlanChange(pruneStreamGroups({ ...current, clips: withoutMoments(current.clips, addedIds) }));
        },
      },
    });
  };
  const removeClips = (removed: readonly StreamClipRange[]) => {
    const [clip] = removed;
    if (!clip) return;
    const removal = removeMoments(plan, removed.map((c) => c.id));
    onPlanChange(removal.plan);
    const [next] = removal.plan.clips;
    if (next) selectClip(next);
    else {
      setCreatingMoment(true);
      clearSelection();
    }
    showWholeVideo();
    toast(removed.length === 1 ? `Quitado: ${clip.title?.trim() || 'momento'}` : `${removed.length} momentos quitados`, {
      position: 'top-right',
      action: {
        label: 'Deshacer',
        onClick: () => {
          const restored = undoRemoval(latestPlanRef.current, removal);
          onPlanChange(restored);
          if (restored.clips.some((c) => c.id === clip.id)) selectClip(clip);
        },
      },
    });
  };
  const applyEdge = ({ clips, clipId, edge }: { clips: StreamClipRange[]; clipId: string; edge: MomentEdge }) => {
    const next = { ...latestPlanRef.current, clips };
    latestPlanRef.current = next;
    onPlanChange(next);
    const clip = clips.find((c) => c.id === clipId);
    if (clip) showEdge(edge === 'start' ? clip.start_seconds : clip.end_seconds);
  };
  const moveEdge = ({ clipId, edge, seconds }: { clipId: string; edge: MomentEdge; seconds: number }) =>
    applyEdge({ clips: moveMomentEdge({ clips: latestPlanRef.current.clips, clipId, edge, seconds, sourceDuration }), clipId, edge });
  const nudgeEdge = ({ clipId, edge, delta }: { clipId: string; edge: MomentEdge; delta: number }) =>
    applyEdge({ clips: nudgeMomentEdge({ clips: latestPlanRef.current.clips, clipId, edge, delta, sourceDuration }), clipId, edge });
  const restoreSuggestedCut = (clip: StreamClipRange, plays: readonly StreamPlay[]) => {
    const cut = suggestedCut({ clips: plan.clips, clipId: clip.id, plays, sourceDuration });
    const next = cut && withMomentRange({ clips: plan.clips, clipId: clip.id, range: cut });
    if (!cut || !next) return;
    const previous = { start_seconds: clip.start_seconds, end_seconds: clip.end_seconds };
    onPlanChange({ ...plan, clips: next });
    showEdge(cut.start_seconds);
    toast('Corte sugerido recuperado', {
      position: 'top-right',
      action: {
        label: 'Deshacer',
        onClick: () => {
          const current = latestPlanRef.current;
          const undone = withMomentRange({ clips: current.clips, clipId: clip.id, range: previous });
          if (undone) onPlanChange({ ...current, clips: undone });
        },
      },
    });
  };
  const removeClip = (clip: StreamClipRange) => removeClips([clip]);
  const startNewMoment = (playheadSeconds: number) => {
    setDraftRange({
      start_seconds: Math.min(playheadSeconds, Math.max(0, sourceDuration - 1)),
      end_seconds: Math.min(sourceDuration, playheadSeconds + 10),
    });
    setCreatingMoment(true);
    showDraft();
  };
  const patchDraft = (patch: Partial<MomentRangeValue>) => setDraftRange((current) => ({ ...current, ...patch }));
  const nudgeDraft = ({ edge, delta }: { edge: MomentEdge; delta: number }) =>
    setDraftRange((current) => {
      const [moved] = nudgeMomentEdge({ clips: [{ id: 'draft', ...current }], clipId: 'draft', edge, delta, sourceDuration });
      return moved ? { start_seconds: moved.start_seconds, end_seconds: moved.end_seconds } : current;
    });
  const cancelDraft = () => setCreatingMoment(false);

  return {
    creatingMoment,
    draftRange,
    draftIssue,
    patchDraft,
    cancelDraft,
    setClips,
    selectClip,
    addMoment,
    addPlays,
    removeClip,
    moveEdge,
    nudgeEdge,
    restoreSuggestedCut,
    startNewMoment,
    nudgeDraft,
  };
}

export type StreamMoments = ReturnType<typeof useStreamMoments>;
