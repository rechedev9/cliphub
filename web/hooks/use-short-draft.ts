'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { Play } from '@/lib/api/types';
import { clearShortDraft, defaultShortSettings, loadShortDraft, saveShortDraft, type ShortSettings } from '@/lib/produce/short-draft';

/** The producer is keyed by matchId. Polls must not reinitialize the user's choices. */
export function useShortDraft(matchId: string, plays: Play[]) {
  const [initial] = useState(() => ({ settings: defaultShortSettings(plays), validIds: new Set(plays.map((play) => play.id)) }));
  const [settings, setSettings] = useState(initial.settings);
  const current = useRef(initial.settings);
  const initialized = useRef(false);
  const edited = useRef(false);
  const [loaded, setLoaded] = useState(false);
  const [restored, setRestored] = useState(false);

  useEffect(() => {
    const saved = loadShortDraft(matchId, initial.validIds);
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
