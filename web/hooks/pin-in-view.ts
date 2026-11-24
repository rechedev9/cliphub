const PIN_MS = 2000;
const USER_SCROLL_EVENTS = ['wheel', 'touchstart', 'keydown', 'pointerdown'] as const;

export function pinInView(element: HTMLElement): () => void {
  const content = element.closest('main') ?? document.body;
  const observer = new ResizeObserver(() => element.scrollIntoView({ block: 'start' }));
  const stop = (): void => {
    observer.disconnect();
    window.clearTimeout(timer);
    for (const type of USER_SCROLL_EVENTS) window.removeEventListener(type, stop, true);
  };
  const timer = window.setTimeout(stop, PIN_MS);
  observer.observe(content);
  for (const type of USER_SCROLL_EVENTS) window.addEventListener(type, stop, { capture: true, passive: true });
  return stop;
}
