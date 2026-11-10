'use client';

import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { LibraryApiError } from '@/lib/api/library';
import { extractDemoVoice, getDemoVoice } from '@/lib/api/voice';
import type { DemoVoice } from '@/lib/api/voice';
import { voiceErrorText } from '@/lib/replay/voice-errors';

const POLL_MS = 2000;

export type DemoVoiceControl = {
  /** null while it loads, or when this Studio cannot read voice at all. */
  demo: DemoVoice | null;
  busy: boolean;
  /** Whether the Replay plays the demo's own voice. */
  on: boolean;
  /** Turns it on or off; the first time it reads the voice from the demo, then plays it. */
  toggle: () => void;
};

/** The in-game voice a library match's demo carries, read once from the demo file on request. */
export function useDemoVoice(matchId: string | null): DemoVoiceControl {
  const [demo, setDemo] = useState<DemoVoice | null>(null);
  const [busy, setBusy] = useState(false);
  const [on, setOn] = useState(false);

  const load = useCallback(async (id: string): Promise<void> => {
    try {
      setDemo(await getDemoVoice(id));
    } catch {
      setDemo(null);
    }
  }, []);

  useEffect(() => {
    setDemo(null);
    setOn(false);
    if (matchId !== null) void load(matchId);
  }, [load, matchId]);

  const extracting = demo?.state === 'extracting';
  useEffect(() => {
    if (!extracting || matchId === null) return;
    const timer = window.setInterval(() => void load(matchId), POLL_MS);
    return () => window.clearInterval(timer);
  }, [extracting, load, matchId]);

  const failed = demo?.state === 'failed';
  useEffect(() => {
    if (failed) toast.error('No se pudo leer la voz de la demo; studio.log tiene el detalle.');
  }, [failed]);

  const toggle = useCallback(() => {
    if (matchId === null || demo === null) return;
    if (demo.state === 'ready') {
      setOn((current) => !current);
      return;
    }
    setBusy(true);
    setOn(true);
    extractDemoVoice(matchId)
      .then(setDemo)
      .catch((error: unknown) => {
        setOn(false);
        toast.error(voiceErrorText(error instanceof LibraryApiError ? error.code : undefined, 'No se pudo leer la voz de la demo.'));
      })
      .finally(() => setBusy(false));
  }, [demo, matchId]);

  return { demo, busy, on: on && demo?.state === 'ready' && demo.tracks.length > 0, toggle };
}
