'use client';

import { useEffect, useRef, useState } from 'react';
import { Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { deleteErrorMessage } from '@/lib/delete-error';

/** How long the armed "¿BORRAR?" state waits before reverting on its own. */
const REVERT_MS = 8000;

/**
 * Two-step inline delete for a Partidas row (match or series). The first click
 * arms a destructive "¿BORRAR?" button; the second confirms. Blur or a short
 * timeout disarms it, so there is no native confirm() and no modal. While
 * deleting the button shows a spinner and is disabled; on success `onDeleted`
 * lets the page re-fetch, and on failure the row shows an inline Spanish message
 * (offline hint or the orchestrator's 409 explanation) instead of crashing.
 *
 * Both states are `Button` now: the hand-rolled versions re-typed the geometry
 * and the whole `focus-visible:ring-*` recipe that `buttonVariants` already
 * ships — the single most duplicated string in the domain layer.
 */
export function DeleteMatchButton({
  label,
  onConfirm,
  onDeleted,
}: {
  label: string;
  onConfirm: () => Promise<void>;
  onDeleted: () => void;
}) {
  const [armed, setArmed] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  function clearTimer() {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
  }

  useEffect(() => clearTimer, []);

  function arm() {
    setError(null);
    setArmed(true);
    clearTimer();
    timer.current = setTimeout(() => setArmed(false), REVERT_MS);
  }

  function disarm() {
    clearTimer();
    setArmed(false);
  }

  async function confirm() {
    if (deleting) return;
    clearTimer();
    setDeleting(true);
    setError(null);
    try {
      await onConfirm();
      setArmed(false);
      onDeleted();
    } catch (err) {
      setError(deleteErrorMessage(err));
      setArmed(false);
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex shrink-0 flex-col items-end gap-1">
      {armed || deleting ? (
        <Button
          type="button"
          variant="destructive"
          autoFocus
          loading={deleting}
          onClick={() => void confirm()}
          onBlur={disarm}
          aria-label={`Confirmar borrar ${label}`}
          className="font-mono text-meta uppercase tracking-wider"
        >
          {deleting ? null : <Trash2 aria-hidden />}
          {deleting ? 'BORRANDO…' : '¿BORRAR?'}
        </Button>
      ) : (
        <Button
          type="button"
          variant="outline"
          size="icon"
          onClick={arm}
          aria-label={`Borrar ${label}`}
          className="text-fg-3 hover:border-destructive/60 hover:bg-destructive/10 hover:text-destructive"
        >
          <Trash2 aria-hidden />
        </Button>
      )}
      {error ? (
        <p role="status" className="max-w-[13rem] text-right font-mono text-meta leading-tight text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  );
}
