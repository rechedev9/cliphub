/** Hands each page the desktop asks for (an assistant's open_in_studio) to `navigate`; returns the unsubscribe. */
type OnOpen = (navigate: (path: string) => void) => () => void;

/** The desktop's in-app navigation bridge (desktop/src/preload.ts), or null in a browser. */
export function desktopNavigation(scope: object = globalThis): OnOpen | null {
  const bridge: unknown = Reflect.get(scope, 'cliphubNavigation');
  if (typeof bridge !== 'object' || bridge === null || !('onOpen' in bridge)) return null;
  const onOpen: unknown = bridge.onOpen;
  if (typeof onOpen !== 'function') return null;
  return (navigate) => {
    const stop: unknown = Reflect.apply(onOpen, bridge, [
      (path: unknown) => {
        if (isStudioPagePath(path)) navigate(path);
      },
    ]);
    return typeof stop === 'function' ? () => void Reflect.apply(stop, undefined, []) : () => {};
  };
}

/** A page of the Studio UI: not an API route, a Next asset or a file. The desktop checks the same. */
export function isStudioPagePath(path: unknown): path is string {
  if (typeof path !== 'string' || !path.startsWith('/') || path.startsWith('//') || /[\\\r\n]/.test(path)) return false;
  const pathname = path.split(/[?#]/, 1)[0] ?? '';
  if (pathname === '/api' || pathname.startsWith('/api/') || pathname.startsWith('/_next/')) return false;
  return !(pathname.split('/').pop() ?? '').includes('.');
}
