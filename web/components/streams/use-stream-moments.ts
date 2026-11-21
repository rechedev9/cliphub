import { useRef, useState } from 'react';
import { toast } from 'sonner';
import type { StreamClipRange, StreamEditPlan, StreamJob, StreamPlay } from '@/lib/api/streams';
import { streamRangeIssue, streamRangeOverlapIssue } from '@/lib/clip-edit';
import { pruneStreamGroups, restoreStreamGroup } from '@/lib/streams/groups';
import { withoutMoments } from '@/lib/streams/moments';
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
}: {
  job: StreamJob;
  plan: StreamEditPlan;
  sourceDuration: number;
  onPlanChange: (plan: StreamEditPlan) => void;
  focusClip: (clip: StreamClipRange) => void;
  clearSelection: () => void;
  showDraft: () => void;
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
    const ids = new Set(removed.map((c) => c.id));
    const groups = plan.groups?.filter((g) => g.clip_ids.some((id) => ids.has(id))) ?? [];
    const next = plan.clips.filter((c) => !ids.has(c.id));
    setClips(next);
    if (next.length) selectClip(next[0]);
    else {
      setCreatingMoment(true);
      clearSelection();
    }
    toast(removed.length === 1 ? 'Momento quitado' : `${removed.length} momentos quitados`, {
      position: 'top-right',
      action: {
        label: 'Deshacer',
        onClick: () => {
          const current = latestPlanRef.current;
          const missing = removed.filter((r) => !current.clips.some((c) => c.id === r.id));
          if (missing.length) {
            const restored: StreamEditPlan = { ...current, clips: missing.reduce(insertClipSorted, current.clips) };
            onPlanChange(groups.reduce(restoreStreamGroup, restored));
          }
          selectClip(clip);
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
    startNewMoment,
  };
}

export type StreamMoments = ReturnType<typeof useStreamMoments>;
