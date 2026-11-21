import type { StreamClipRange, StreamPlay } from '../api/streams.ts';
import { playRowAction } from './moments.ts';

export type MomentsKeyFocus = {
  tag: string;
  role: string | null;
  editable: boolean;
  playRowId: string | null;
  inDialog: boolean;
};

export type MomentsKeyState = {
  plays: readonly StreamPlay[];
  clips: readonly StreamClipRange[];
  selectedPlayId: string | null;
  lastPlayId: string | null;
  selectedClipId: string | null;
  canMark: boolean;
  busy: boolean;
};

export type MomentsKeyAction =
  | { kind: 'none' }
  | { kind: 'toggle-play' }
  | { kind: 'watch-play'; play: StreamPlay }
  | { kind: 'add-play'; play: StreamPlay }
  | { kind: 'jump'; clip: StreamClipRange }
  | { kind: 'remove'; clip: StreamClipRange }
  | { kind: 'mark' };

export const MOMENTS_KEY = {
  play: 'Espacio',
  add: 'Intro',
  remove: 'Supr',
  mark: 'M',
} as const;

export type ShortcutHint = { keys: readonly string[]; label: string };

export function momentsShortcutLegend({ vod }: { vod: boolean }): ShortcutHint[] {
  const jugadas: ShortcutHint[] = [
    { keys: ['↑', '↓'], label: 'pasar de jugada' },
    { keys: [MOMENTS_KEY.add], label: 'añadir' },
  ];
  return [
    ...(vod ? jugadas : []),
    { keys: [MOMENTS_KEY.play], label: 'reproducir' },
    { keys: [MOMENTS_KEY.remove], label: 'quitar momento' },
    { keys: [MOMENTS_KEY.mark], label: 'marcar momento' },
  ];
}

const NONE: MomentsKeyAction = { kind: 'none' };
const TEXT_TAGS = new Set(['INPUT', 'TEXTAREA', 'SELECT']);
const CONTROL_TAGS = new Set(['BUTTON', 'A', 'SUMMARY']);
const CONTROL_ROLES = new Set(['button', 'link', 'switch', 'checkbox', 'radio', 'tab', 'option', 'menuitem', 'menuitemradio', 'menuitemcheckbox']);
const ARROW_ROLES = new Set(['slider', 'spinbutton', 'scrollbar', 'radio', 'radiogroup', 'tab', 'tablist', 'listbox', 'option', 'menu', 'menuitem', 'menuitemradio', 'menuitemcheckbox', 'combobox', 'tree', 'treeitem', 'grid', 'gridcell']);

function typing(focus: MomentsKeyFocus): boolean {
  return focus.editable || TEXT_TAGS.has(focus.tag);
}

function ownsActivation(focus: MomentsKeyFocus): boolean {
  if (focus.playRowId !== null) return false;
  return CONTROL_TAGS.has(focus.tag) || (focus.role !== null && CONTROL_ROLES.has(focus.role));
}

function ownsArrows(focus: MomentsKeyFocus): boolean {
  return focus.role !== null && ARROW_ROLES.has(focus.role);
}

function steppedPlay(state: MomentsKeyState, step: 1 | -1): MomentsKeyAction {
  const { plays, selectedPlayId, lastPlayId } = state;
  const from = plays.findIndex((p) => p.id === (selectedPlayId ?? lastPlayId));
  const target = plays[from < 0 ? 0 : from + step];
  return target ? { kind: 'watch-play', play: target } : NONE;
}

function keptPlay(state: MomentsKeyState, playId: string | null): MomentsKeyAction {
  const play = state.plays.find((p) => p.id === playId);
  if (!play) return NONE;
  const action = playRowAction(state.clips, play);
  return action.kind === 'jump' ? { kind: 'jump', clip: action.clip } : { kind: 'add-play', play };
}

function removedMoment(state: MomentsKeyState): MomentsKeyAction {
  const clip = state.clips.find((c) => c.id === state.selectedClipId);
  return clip ? { kind: 'remove', clip } : NONE;
}

export function momentsKeyAction({
  key,
  repeat,
  modified,
  focus,
  state,
}: {
  key: string;
  repeat: boolean;
  modified: boolean;
  focus: MomentsKeyFocus;
  state: MomentsKeyState;
}): MomentsKeyAction {
  if (modified || state.busy || focus.inDialog || typing(focus)) return NONE;
  if (key === 'ArrowDown' || key === 'ArrowUp') {
    return ownsArrows(focus) ? NONE : steppedPlay(state, key === 'ArrowDown' ? 1 : -1);
  }
  if (repeat) return NONE;
  if (key === ' ') return ownsActivation(focus) ? NONE : { kind: 'toggle-play' };
  if (key === 'Enter') return ownsActivation(focus) ? NONE : keptPlay(state, focus.playRowId ?? state.selectedPlayId);
  if (key === 'Delete' || key === 'Backspace') return removedMoment(state);
  if (key === 'm' || key === 'M') return state.canMark ? { kind: 'mark' } : NONE;
  return NONE;
}
