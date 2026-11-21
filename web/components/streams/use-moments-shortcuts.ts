import { useEffect, useRef } from 'react';
import { momentsKeyAction, type MomentsKeyAction, type MomentsKeyFocus, type MomentsKeyState } from '@/lib/streams/moments-keys';

const PAGE_FOCUS: MomentsKeyFocus = { tag: 'BODY', role: null, editable: false, playRowId: null, inDialog: false };

function keyFocus(target: EventTarget | null): MomentsKeyFocus {
  if (!(target instanceof HTMLElement)) return PAGE_FOCUS;
  return {
    tag: target.tagName,
    role: target.getAttribute('role'),
    editable: target.isContentEditable,
    playRowId: target.dataset.playRow ?? null,
    inDialog: target.closest('dialog, [role="dialog"], [role="alertdialog"]') !== null,
  };
}

export function focusPlayRow(playId: string): void {
  const row = document.querySelector(`[data-play-row="${CSS.escape(playId)}"]`);
  if (row instanceof HTMLElement) row.focus({ preventScroll: true });
}

export function useMomentsShortcuts({
  state,
  onAction,
}: {
  state: Omit<MomentsKeyState, 'lastPlayId'>;
  onAction: (action: Exclude<MomentsKeyAction, { kind: 'none' }>) => void;
}): void {
  const lastPlayRef = useRef<string | null>(null);
  if (state.selectedPlayId) lastPlayRef.current = state.selectedPlayId;
  const latestRef = useRef({ state, onAction });
  latestRef.current = { state, onAction };

  useEffect(() => {
    let spaceHandledOnRow = false;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) return;
      const focus = keyFocus(event.target);
      const action = momentsKeyAction({
        key: event.key,
        repeat: event.repeat,
        modified: event.ctrlKey || event.metaKey || event.altKey,
        focus,
        state: { ...latestRef.current.state, lastPlayId: lastPlayRef.current },
      });
      if (action.kind === 'none') return;
      event.preventDefault();
      if (event.key === ' ') spaceHandledOnRow = focus.playRowId !== null;
      latestRef.current.onAction(action);
    };
    const onKeyUp = (event: KeyboardEvent) => {
      if (event.key !== ' ' || !spaceHandledOnRow) return;
      spaceHandledOnRow = false;
      event.preventDefault();
    };
    window.addEventListener('keydown', onKeyDown);
    window.addEventListener('keyup', onKeyUp);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('keyup', onKeyUp);
    };
  }, []);
}
