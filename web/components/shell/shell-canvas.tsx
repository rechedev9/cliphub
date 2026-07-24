import type { ReactElement } from 'react';

/**
 * The room's static geometry: horizon wash, two-level lattice, a CSS-3D floor
 * receding to the horizon, scanlines and a vignette — one fixed, aria-hidden,
 * never-animated plane. It replaces the two `.neon-grid` applications that used
 * to sit on <body> and on SidebarInset.
 *
 * Both of those were document-height elements, so `at 42% -12%` resolved
 * against a 3000px page and the ambient wash never reached the viewport, the
 * 48px lattice was painted twice from two different origins, and the copy
 * inside SidebarInset restarted at x = 240px — collapsing the sidebar jumped
 * the whole texture sideways. Fixed positioning fixes all three at once.
 *
 * Server component on purpose: there is no state, no effect and no browser API
 * here, and it must be in the SSR paint.
 */
export function ShellCanvas(): ReactElement {
  return <div aria-hidden className="shell-canvas" />;
}
