interface ClipboardBridge {
  writeText(value: string): Promise<void>;
}

interface ClipboardScope {
  tickcutClipboard?: unknown;
  navigator?: {
    clipboard?: {
      writeText?: unknown;
    };
  };
}

/** Uses Electron's user-activation-gated bridge, with the browser API fallback. */
export async function writeClipboardText(value: string, scope: ClipboardScope = globalThis): Promise<void> {
  const bridge = clipboardBridge(scope.tickcutClipboard);
  if (bridge !== null) {
    await bridge.writeText(value);
    return;
  }
  const browserWrite = scope.navigator?.clipboard?.writeText;
  if (typeof browserWrite !== 'function') throw new Error('clipboard write unavailable');
  await browserWrite.call(scope.navigator?.clipboard, value);
}

function clipboardBridge(value: unknown): ClipboardBridge | null {
  if (typeof value !== 'object' || value === null) return null;
  const writeText = (value as { writeText?: unknown }).writeText;
  return typeof writeText === 'function'
    ? { writeText: writeText.bind(value) as ClipboardBridge['writeText'] }
    : null;
}
