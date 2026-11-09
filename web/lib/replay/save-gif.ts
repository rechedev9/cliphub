/**
 * Where the Replay's GIF goes. In Studio the desktop asks where to save it the
 * moment the user clicks, while the click still counts as theirs, then writes
 * the finished GIF there. A plain browser has no bridge and downloads it.
 */

export type GifTarget =
  | { readonly kind: 'file'; readonly token: string }
  | { readonly kind: 'download' }
  | { readonly kind: 'canceled' }
  | { readonly kind: 'failed'; readonly message: string };

export type GifWrite = { readonly kind: 'saved'; readonly name: string } | { readonly kind: 'failed'; readonly message: string };

const FAILED = 'No se pudo guardar el GIF.';

type Call = (...args: unknown[]) => Promise<unknown>;

function method(value: object, name: string): Call | null {
  const fn: unknown = Reflect.get(value, name);
  if (typeof fn !== 'function') return null;
  return (...args) => Promise.resolve(Reflect.apply(fn, value, args));
}

function replayBridge(scope: object): { pick: Call; write: Call } | null {
  const value: unknown = Reflect.get(scope, 'cliphubReplay');
  if (typeof value !== 'object' || value === null) return null;
  const pick = method(value, 'pickGif');
  const write = method(value, 'writeGif');
  return pick === null || write === null ? null : { pick, write };
}

function field(value: unknown, name: string): unknown {
  return typeof value === 'object' && value !== null ? Reflect.get(value, name) : undefined;
}

function failure(result: unknown): { kind: 'failed'; message: string } {
  const error = field(result, 'error');
  return { kind: 'failed', message: typeof error === 'string' ? error : FAILED };
}

/** Asks the desktop where to save the GIF; call it straight from the click. */
export async function pickGifTarget(fileName: string, scope: object = globalThis): Promise<GifTarget> {
  const bridge = replayBridge(scope);
  if (bridge === null) return { kind: 'download' };
  const result = await bridge.pick(fileName);
  const token = field(result, 'token');
  if (field(result, 'ok') === true && typeof token === 'string') return { kind: 'file', token };
  if (field(result, 'canceled') === true) return { kind: 'canceled' };
  return failure(result);
}

/** Writes the finished GIF where the user chose; `name` is the file's name, without its folder. */
export async function writeGifTarget(token: string, bytes: Uint8Array, scope: object = globalThis): Promise<GifWrite> {
  const bridge = replayBridge(scope);
  if (bridge === null) return { kind: 'failed', message: FAILED };
  const result = await bridge.write(token, bytes);
  const name = field(result, 'name');
  if (field(result, 'ok') === true) return { kind: 'saved', name: typeof name === 'string' ? name : '' };
  return failure(result);
}
