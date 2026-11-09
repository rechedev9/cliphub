export type ReplayShortcut =
  | 'help'
  | 'fullscreen'
  | 'escape'
  | 'multi'
  | 'follow'
  | 'setpos'
  | 'playlist'
  | 'gif'
  | 'tool-move'
  | 'tool-draw'
  | 'tool-arrow'
  | 'tool-erase'
  | 'tool-timed'
  | 'undo-stroke'
  | 'fold-left'
  | 'fold-right'
  | 'play'
  | 'round-prev'
  | 'round-next'
  | 'seek-back'
  | 'seek-forward'
  | 'event-back'
  | 'event-forward';

/** The parts of a keydown the replay reads; `altGraph` is `getModifierState('AltGraph')`. */
export type ReplayKey = { key: string; shiftKey: boolean; ctrlKey: boolean; altKey: boolean; metaKey: boolean; altGraph: boolean };

/** The focused element as far as shortcuts care. `keyboardFocus`: it was not focused by a mouse press. */
export type KeyTarget = { tag: string; inputType: string | null; editable: boolean; keyboardFocus: boolean };

function arrow(key: string, shift: boolean): ReplayShortcut | null {
  if (key === 'ArrowLeft') return shift ? 'event-back' : 'seek-back';
  if (key === 'ArrowRight') return shift ? 'event-forward' : 'seek-forward';
  return null;
}

const CHARACTER_KEYS: Readonly<Record<string, ReplayShortcut>> = {
  '?': 'help',
  p: 'fullscreen',
  P: 'fullscreen',
  m: 'multi',
  M: 'multi',
  f: 'follow',
  F: 'follow',
  s: 'setpos',
  S: 'setpos',
  b: 'playlist',
  B: 'playlist',
  g: 'gif',
  G: 'gif',
  h: 'tool-move',
  H: 'tool-move',
  d: 'tool-draw',
  D: 'tool-draw',
  a: 'tool-arrow',
  A: 'tool-arrow',
  e: 'tool-erase',
  E: 'tool-erase',
  t: 'tool-timed',
  T: 'tool-timed',
  z: 'undo-stroke',
  Z: 'undo-stroke',
  ',': 'fold-left',
  '.': 'fold-right',
  ' ': 'play',
  '[': 'round-prev',
  ']': 'round-next',
  // J and K as in Skybox EDGE, for teams used to it.
  j: 'round-prev',
  J: 'round-prev',
  k: 'round-next',
  K: 'round-next',
};

/** Which replay shortcut a key press asks for, or null when it is not one. */
export function replayShortcut(press: ReplayKey): ReplayShortcut | null {
  // Windows reports AltGr as Ctrl+Alt; on Spanish and German layouts it types [ and ].
  const altGraph = press.altGraph || (press.ctrlKey && press.altKey);
  const character = press.key.length === 1;
  if (press.metaKey) return null;
  if ((press.ctrlKey || press.altKey) && !(altGraph && character)) return null;
  if (press.key === 'Escape') return 'escape';
  // Supr picks the eraser too, as in the owner's key table; E stays.
  if (press.key === 'Delete') return 'tool-erase';
  if (press.key === '/' && press.shiftKey) return 'help';
  return CHARACTER_KEYS[press.key] ?? arrow(press.key, press.shiftKey);
}

/** Whether the focused element leaves this shortcut to the replay. */
export function targetTakesKey(target: KeyTarget, shortcut: ReplayShortcut): boolean {
  if (target.editable || target.tag === 'TEXTAREA' || target.tag === 'SELECT') return false;
  if (target.tag === 'INPUT' && target.inputType !== 'range' && target.inputType !== 'checkbox') return false;
  // A control reached with Tab keeps Space to press it; after a mouse click Space plays or pauses.
  const pressable = target.tag === 'BUTTON' || target.inputType === 'checkbox';
  return !(shortcut === 'play' && pressable && target.keyboardFocus);
}
