/** Hub first-run guide dismissal; the guide also hides itself once every step is done. */
const GUIDE_DISMISSED_KEY = 'cliphub.hub.guide.v1';

export function isFirstRunGuideDismissed(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(GUIDE_DISMISSED_KEY) !== null;
  } catch {
    return false;
  }
}

export function dismissFirstRunGuide(): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(GUIDE_DISMISSED_KEY, '1');
  } catch {
    // quota / privacy mode: the guide simply returns next visit.
  }
}
