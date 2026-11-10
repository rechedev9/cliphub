/**
 * Where the Replay's GIF or MP4 goes. In Studio the desktop asks where to save it the
 * moment the user clicks, while the click still counts as theirs, then writes
 * the finished file there. A plain browser has no bridge and downloads it.
 */

export type GifTarget =
  | { readonly kind: 'file'; readonly token: string }
  | { readonly kind: 'download' }
  | { readonly kind: 'canceled' }
  | { readonly kind: 'failed'; readonly message: string };

export type GifWrite = { readonly kind: 'saved'; readonly name: string } | { readonly kind: 'failed'; readonly message: string };

type Format = { pick: string; write: string; failed: string };

const GIF: Format = { pick: 'pickGif', write: 'writeGif', failed: 'No se pudo guardar el GIF.' };
const VIDEO: Format = { pick: 'pickVideo', write: 'writeVideo', failed: 'No se pudo guardar el MP4.' };

type Call = (...args: unknown[]) => Promise<unknown>;

function method(value: object, name: string): Call | null {
  const fn: unknown = Reflect.get(value, name);
  if (typeof fn !== 'function') return null;
  return (...args) => Promise.resolve(Reflect.apply(fn, value, args));
}

function replayBridge(scope: object, format: Format): { pick: Call; write: Call } | null {
  const value: unknown = Reflect.get(scope, 'cliphubReplay');
  if (typeof value !== 'object' || value === null) return null;
  const pick = method(value, format.pick);
  const write = method(value, format.write);
  return pick === null || write === null ? null : { pick, write };
}

function field(value: unknown, name: string): unknown {
  return typeof value === 'object' && value !== null ? Reflect.get(value, name) : undefined;
}

function failure(result: unknown, format: Format): { kind: 'failed'; message: string } {
  const error = field(result, 'error');
  return { kind: 'failed', message: typeof error === 'string' ? error : format.failed };
}

async function pickTarget(options: { fileName: string; scope: object; format: Format }): Promise<GifTarget> {
  const bridge = replayBridge(options.scope, options.format);
  if (bridge === null) return { kind: 'download' };
  const result = await bridge.pick(options.fileName);
  const token = field(result, 'token');
  if (field(result, 'ok') === true && typeof token === 'string') return { kind: 'file', token };
  if (field(result, 'canceled') === true) return { kind: 'canceled' };
  return failure(result, options.format);
}

async function writeTarget(options: { token: string; bytes: Uint8Array; scope: object; format: Format }): Promise<GifWrite> {
  const bridge = replayBridge(options.scope, options.format);
  if (bridge === null) return { kind: 'failed', message: options.format.failed };
  const result = await bridge.write(options.token, options.bytes);
  const name = field(result, 'name');
  if (field(result, 'ok') === true) return { kind: 'saved', name: typeof name === 'string' ? name : '' };
  return failure(result, options.format);
}

/** Asks the desktop where to save the GIF; call it straight from the click. */
export function pickGifTarget(fileName: string, scope: object = globalThis): Promise<GifTarget> {
  return pickTarget({ fileName, scope, format: GIF });
}

/** Writes the finished GIF where the user chose; `name` is the file's name, without its folder. */
export function writeGifTarget(token: string, bytes: Uint8Array, scope: object = globalThis): Promise<GifWrite> {
  return writeTarget({ token, bytes, scope, format: GIF });
}

/** Asks the desktop where to save the MP4; call it straight from the click. */
export function pickVideoTarget(fileName: string, scope: object = globalThis): Promise<GifTarget> {
  return pickTarget({ fileName, scope, format: VIDEO });
}

/** Writes the finished MP4 where the user chose. */
export function writeVideoTarget(options: { token: string; bytes: Uint8Array; scope?: object }): Promise<GifWrite> {
  return writeTarget({ token: options.token, bytes: options.bytes, scope: options.scope ?? globalThis, format: VIDEO });
}
