/**
 * The shell's chrome-state cookie: written on the client, read on the server.
 *
 * They live in a module with no `'use client'` directive on purpose. A value
 * exported from a client module and imported by a server component does not
 * arrive as that value — React hands the server a client *reference*, so
 * `cookies().get(SIDEBAR_COOKIE_NAME)` silently looked up a proxy instead of
 * `"sidebar_state"`, returned undefined, and the sidebar came back expanded on
 * every reload with no error anywhere. Measured in the browser before this file
 * existed. Keep these constants out of `components/ui/sidebar.tsx`.
 */

export const SIDEBAR_COOKIE_NAME = 'sidebar_state';
export const SIDEBAR_COOKIE_MAX_AGE = 60 * 60 * 24 * 7;
