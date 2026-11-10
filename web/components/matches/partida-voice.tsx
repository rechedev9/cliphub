'use client';

import { useState, type ReactNode } from 'react';
import { Mic, RefreshCw } from 'lucide-react';
import { toast } from 'sonner';
import { LibraryApiError } from '@/lib/api/library';
import { autoSyncVoice, type VoiceSummary } from '@/lib/api/voice';
import { resyncOutcome, voiceSyncTitle } from '@/lib/library/voice-summary';
import { voiceErrorText } from '@/lib/replay/voice-errors';
import { cn } from '@/lib/utils';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';

type Resync =
  | { kind: 'idle' }
  | { kind: 'running' }
  | { kind: 'done'; tone: 'ok' | 'warning'; text: string }
  | { kind: 'failed'; text: string };

type PartidaVoiceProps = {
  summary: VoiceSummary;
  onResynced: () => void;
  /** The table cell: the icon and a resync icon button, with the result as a toast instead of a line. */
  compact?: boolean;
};

/** The line under the icon: the last resync's result, else the reason an unsynced recording gave. */
function lineOf(resync: Resync, note: string | undefined): string | null {
  if (resync.kind === 'done' || resync.kind === 'failed') return resync.text;
  return note ? note : null;
}

function toneOf(resync: Resync): string {
  if (resync.kind === 'failed') return 'text-destructive';
  return resync.kind === 'done' && resync.tone === 'ok' ? 'text-fg-2' : 'text-warning';
}

/** The voice icon of a row with its sync state, the reason an unsynced recording gave, and Volver a sincronizar voz. */
export function PartidaVoice({ summary, onResynced, compact = false }: PartidaVoiceProps): ReactNode {
  const [resync, setResync] = useState<Resync>({ kind: 'idle' });
  const [confirming, setConfirming] = useState(false);
  const { sync } = summary;
  const title = voiceSyncTitle(sync);

  async function run(): Promise<void> {
    setConfirming(false);
    setResync({ kind: 'running' });
    try {
      const voice = await autoSyncVoice(summary.match_id);
      const outcome = resyncOutcome(voice.sync);
      setResync({ kind: 'done', ...outcome });
      if (compact) (outcome.tone === 'ok' ? toast.success : toast.warning)(outcome.text);
      onResynced();
    } catch (error) {
      const code = error instanceof LibraryApiError ? error.code : undefined;
      const text = voiceErrorText(code, 'No se pudo sincronizar la voz. Inténtalo de nuevo.');
      setResync({ kind: 'failed', text });
      if (compact) toast.error(text);
    }
  }

  // A hand-made sync is the user's work: one click must not overwrite it.
  function ask(): void {
    if (sync.state === 'manual') setConfirming(true);
    else void run();
  }

  const unsynced = sync.state === 'unsynced';
  const message = lineOf(resync, unsynced ? sync.note : undefined);
  const tone = toneOf(resync);
  const hover = message === null ? title : `${title}. ${message}`;
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
      <Tooltip>
        <TooltipTrigger asChild>
          {/* Focusable so the sync state also opens from the keyboard. */}
          <span tabIndex={0} role="img" aria-label={hover} className={cn('inline-flex rounded-sm', FOCUS_RING, unsynced ? 'text-warning' : 'text-fg-2')}>
            <Mic aria-hidden className="size-4" />
          </span>
        </TooltipTrigger>
        <TooltipContent side="bottom" className="max-w-sm">{hover}</TooltipContent>
      </Tooltip>
      {compact ? (
        <Button type="button" variant="ghost" size="icon-xs" aria-label="Volver a sincronizar voz" title="Volver a sincronizar voz" loading={resync.kind === 'running'} onClick={ask}>
          <RefreshCw aria-hidden />
        </Button>
      ) : (
        <Button type="button" variant="ghost" size="xs" loading={resync.kind === 'running'} onClick={ask}>
          <RefreshCw aria-hidden />
          Volver a sincronizar voz
        </Button>
      )}
      {!compact && message !== null ? (
        <span role={resync.kind === 'failed' ? 'alert' : 'status'} className={cn('max-w-72 truncate text-meta', tone)} title={message}>
          {message}
        </span>
      ) : null}
      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Volver a sincronizar la voz</DialogTitle>
            <DialogDescription>La voz está sincronizada a mano. ¿Volver a sincronizarla sola? Se perderá el ajuste manual.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirming(false)}>Cancelar</Button>
            <Button type="button" onClick={() => void run()}>Volver a sincronizar</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
