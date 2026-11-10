'use client';

import { useState, type ReactNode } from 'react';
import { Mic, RefreshCw } from 'lucide-react';
import { LibraryApiError } from '@/lib/api/library';
import { autoSyncVoice, type VoiceSummary } from '@/lib/api/voice';
import { resyncOutcome, voiceSyncTitle } from '@/lib/library/voice-summary';
import { voiceErrorText } from '@/lib/replay/voice-errors';
import { cn } from '@/lib/utils';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

type Resync =
  | { kind: 'idle' }
  | { kind: 'running' }
  | { kind: 'done'; tone: 'ok' | 'warning'; text: string }
  | { kind: 'failed'; text: string };

/** The voice icon of a row with its sync state, the reason an unsynced recording gave, and Volver a sincronizar voz. */
export function PartidaVoice({ summary, onResynced }: { summary: VoiceSummary; onResynced: () => void }): ReactNode {
  const [resync, setResync] = useState<Resync>({ kind: 'idle' });
  const { sync } = summary;
  const title = voiceSyncTitle(sync);

  async function run(): Promise<void> {
    setResync({ kind: 'running' });
    try {
      const voice = await autoSyncVoice(summary.match_id);
      setResync({ kind: 'done', ...resyncOutcome(voice.sync) });
      onResynced();
    } catch (error) {
      const code = error instanceof LibraryApiError ? error.code : undefined;
      setResync({ kind: 'failed', text: voiceErrorText(code, 'No se pudo sincronizar la voz. Inténtalo de nuevo.') });
    }
  }

  const unsynced = sync.state === 'unsynced';
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
      <Tooltip>
        <TooltipTrigger asChild>
          {/* Focusable so the sync state also opens from the keyboard. */}
          <span tabIndex={0} role="img" aria-label={title} className={cn('inline-flex rounded-sm', FOCUS_RING, unsynced ? 'text-warning' : 'text-fg-2')}>
            <Mic aria-hidden className="size-4" />
          </span>
        </TooltipTrigger>
        <TooltipContent side="bottom">{title}</TooltipContent>
      </Tooltip>
      <Button type="button" variant="ghost" size="xs" loading={resync.kind === 'running'} onClick={() => void run()}>
        <RefreshCw aria-hidden />
        Volver a sincronizar voz
      </Button>
      {unsynced && sync.note && resync.kind !== 'done' ? (
        <span className="max-w-72 truncate text-meta text-warning" title={sync.note}>{sync.note}</span>
      ) : null}
      {resync.kind === 'done' ? (
        <span role="status" className={cn('max-w-72 truncate text-meta', resync.tone === 'warning' ? 'text-warning' : 'text-fg-2')} title={resync.text}>
          {resync.text}
        </span>
      ) : null}
      {resync.kind === 'failed' ? (
        <span role="alert" className="max-w-72 truncate text-meta text-destructive" title={resync.text}>{resync.text}</span>
      ) : null}
    </div>
  );
}
