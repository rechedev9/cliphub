import { useRef } from 'react';
import { toast } from 'sonner';
import type { StreamClipRange, StreamEditPlan, StreamJob, StreamPlay } from '@/lib/api/streams';
import { pruneStreamGroups } from '@/lib/streams/groups';
import { fullVideoMoment, markMomentAt } from '@/lib/streams/mark-moment';
import { moveMomentEdge, nudgeMomentEdge, suggestedCut, withMomentRange, type MomentEdge } from '@/lib/streams/moment-edges';
import { removeMoments, undoRemoval, withoutMoments } from '@/lib/streams/moments';
import { formatStreamClock, insertClipSorted, nextClipId } from '@/lib/streams/plan';
import { clipsWithPlays } from '@/lib/streams/plays';
import { streamTitle } from '@/lib/streams/title';

export function useStreamMoments({
  job,
  plan,
  sourceDuration,
  onPlanChange,
  focusClip,
  clearSelection,
  showEdge,
  showWholeVideo,
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  sourceDuration: number;
  onPlanChange: (plan: StreamEditPlan) => void;
  focusClip: (clip: StreamClipRange) => void;
  clearSelection: () => void;
  showEdge: (seconds: number) => void;
  showWholeVideo: () => void;
}) {
  const latestPlanRef = useRef(plan);
  latestPlanRef.current = plan;

  const setClips = (clips: StreamClipRange[]) => onPlanChange(pruneStreamGroups({ ...plan, clips }));
  const selectClip = focusClip;
  const createMoment = ({ range, message }: { range: { start_seconds: number; end_seconds: number }; message: string }) => {
    const { clips } = latestPlanRef.current;
    const sourceTitle = streamTitle(job);
    const title = clips.length ? `${sourceTitle} ${clips.length + 1}` : sourceTitle;
    const clip: StreamClipRange = { id: nextClipId(), ...range, title };
    const next = pruneStreamGroups({ ...latestPlanRef.current, clips: insertClipSorted(clips, clip) });
    latestPlanRef.current = next;
    onPlanChange(next);
    selectClip(clip);
    toast(message, {
      position: 'top-right',
      action: {
        label: 'Deshacer',
        onClick: () => {
          const current = latestPlanRef.current;
          onPlanChange(pruneStreamGroups({ ...current, clips: withoutMoments(current.clips, [clip.id]) }));
          showWholeVideo();
        },
      },
    });
  };
  const markHere = (seconds: number) => {
    const outcome = markMomentAt({ clips: latestPlanRef.current.clips, seconds, sourceDuration });
    if (outcome.kind === 'inside') {
      selectClip(outcome.clip);
      toast('Aquí ya hay un momento. Ajusta sus bordes.', { position: 'top-right' });
    } else if (outcome.kind === 'no-room') {
      toast('No queda hueco aquí para otro momento. Muévete a una parte libre del vídeo.', { position: 'top-right' });
    } else {
      createMoment({ range: outcome.range, message: 'Momento creado. Arrastra los bordes para ajustarlo.' });
    }
  };
  const addFullVideo = () => {
    const range = fullVideoMoment({ clips: latestPlanRef.current.clips, sourceDuration });
    if (range) createMoment({ range, message: 'Vas a usar el vídeo completo' });
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
    else clearSelection();
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
    toast('El momento vuelve a los bordes de la jugada', {
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
  return {
    setClips,
    selectClip,
    markHere,
    addFullVideo,
    addPlays,
    removeClip,
    moveEdge,
    nudgeEdge,
    restoreSuggestedCut,
  };
}

export type StreamMoments = ReturnType<typeof useStreamMoments>;
