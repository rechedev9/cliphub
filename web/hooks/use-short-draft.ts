'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { Play } from '@/lib/api/types';
import { clearShortDraft, defaultShortSettings, loadShortDraft, saveShortDraft, type ShortSettings } from '@/lib/produce/short-draft';

/**
 * The producer is keyed by matchId. Polls must not reinitialize the user's choices.
 * `focusIds` comes from the analysis area: those plays open selected, even over a saved draft.
 */
export function useShortDraft(matchId: string, plays: Play[], focusIds: readonly string[] = []) {
  const [initial] = useState(() => {
    const base = defaultShortSettings(plays);
    const focus = focusIds.length > 0 ? [...focusIds] : null;
    return { settings: focus ? { ...base, selectedIds: focus } : base, validIds: new Set(plays.map((play) => play.id)), focus };
  });
  const [settings, setSettings] = useState(initial.settings);
  const current = useRef(initial.settings);
  const initialized = useRef(false);
  const edited = useRef(false);
  const [loaded, setLoaded] = useState(false);
  const [restored, setRestored] = useState(false);

  useEffect(() => {
    const loaded = loadShortDraft(matchId, initial.validIds);
    const saved = loaded && initial.focus ? { ...loaded, selectedIds: initial.focus } : loaded;
    current.current = saved ?? initial.settings;
    edited.current = saved !== null;
    initialized.current = true;
    setSettings(current.current);
    setRestored(saved !== null);
    setLoaded(true);
  }, [matchId, initial]);

  const updateSettings = useCallback((change: Partial<ShortSettings> | ((value: ShortSettings) => Partial<ShortSettings>), userEdit = true) => {
    if (!initialized.current) return;
    const patch = typeof change === 'function' ? change(current.current) : change;
    const next = { ...current.current, ...patch };
    current.current = next;
    if (userEdit) edited.current = true;
    // Save in the event, not in a delayed effect that navigation could cancel.
    if (edited.current) saveShortDraft(matchId, next);
    setSettings(next);
  }, [matchId]);

  const discardDraft = useCallback(() => {
    edited.current = false;
    clearShortDraft(matchId);
    setRestored(false);
  }, [matchId]);

  const resetSettings = useCallback((next: ShortSettings) => {
    discardDraft();
    current.current = next;
    setSettings(next);
  }, [discardDraft]);

  return { settings, updateSettings, resetSettings, discardDraft, loaded, restored };
}
