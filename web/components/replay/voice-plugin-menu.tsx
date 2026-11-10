'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Radio } from 'lucide-react';
import { listVoiceRecordings, type VoiceRecording } from '@/lib/api/voice-plugin';
import { recordingTitle } from '@/lib/voice-plugin-view';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import type { ReplayVoice } from './use-replay-voice';

type Recordings = { kind: 'idle' } | { kind: 'loading' } | { kind: 'ready'; list: readonly VoiceRecording[] } | { kind: 'failed' };

/**
 * The TeamSpeak plugin from the Replay: the recordings it left without a match, newest first, to give
 * one to this match; with none, the way to Ajustes where the plugin is installed.
 */
export function VoicePluginMenu({ voice }: { voice: ReplayVoice }): ReactNode {
  const [recordings, setRecordings] = useState<Recordings>({ kind: 'idle' });
  const load = (): void => {
    setRecordings({ kind: 'loading' });
    listVoiceRecordings()
      .then(({ recordings: list }) =>
        setRecordings({ kind: 'ready', list: list.filter((entry) => entry.status === 'pending').sort((a, b) => b.started_at.localeCompare(a.started_at)) }),
      )
      .catch(() => setRecordings({ kind: 'failed' }));
  };
  return (
    <Popover onOpenChange={(open) => (open ? load() : undefined)}>
      <PopoverTrigger asChild>
        <Button type="button" size="xs" variant="outline">
          <Radio aria-hidden />
          Plugin de TeamSpeak
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" side="top" className="flex w-96 max-w-[92vw] flex-col gap-2 p-3">
        <h3 className="text-body-sm font-semibold text-fg-1">Grabaciones del plugin sin partida</h3>
        {recordings.kind === 'loading' || recordings.kind === 'idle' ? <p className="text-meta text-fg-3">Leyendo las grabaciones…</p> : null}
        {recordings.kind === 'failed' ? <p className="text-meta text-destructive">No se pudieron leer las grabaciones del plugin.</p> : null}
        {recordings.kind === 'ready' && recordings.list.length === 0 ? (
          <p className="text-meta text-fg-2">Ninguna grabación pendiente. Con el plugin instalado, TeamSpeak graba solo cada partida de CS2.</p>
        ) : null}
        {recordings.kind === 'ready' && recordings.list.length > 0 ? (
          <ul className="flex max-h-64 flex-col gap-1 overflow-y-auto">
            {recordings.list.map((recording) => (
              <li key={recording.id} className="flex items-center gap-2 rounded-sm px-1 py-1 hover:bg-surface-3">
                <span className="min-w-0 flex-1 truncate text-meta text-fg-1 tabular-nums" title={recording.channel}>
                  {recordingTitle(recording)}
                </span>
                <Button type="button" size="xs" variant="outline" loading={voice.busy} onClick={() => voice.attachRecording(recording.id)}>
                  Usar aquí
                </Button>
              </li>
            ))}
          </ul>
        ) : null}
        <Button asChild variant="ghost" size="xs" className="self-start">
          <Link href="/settings#teamspeak">Instalar o revisar el plugin</Link>
        </Button>
      </PopoverContent>
    </Popover>
  );
}
