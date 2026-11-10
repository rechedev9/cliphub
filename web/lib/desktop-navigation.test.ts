import test from 'node:test';
import assert from 'node:assert/strict';
import { desktopNavigation, isStudioPagePath } from './desktop-navigation.ts';

test('only Studio pages pass', () => {
  for (const ok of ['/replay?r=a:3&t=x', '/informes/1b2c', '/equipos/x?tab=preparar#mapa']) assert.equal(isStudioPagePath(ok), true, ok);
  for (const bad of ['/api/library/reports', '/api', '/_next/static/x.js', '/hud/theme.png', '//evil.example', 'replay', '/a\\b', 3]) {
    assert.equal(isStudioPagePath(bad), false, String(bad));
  }
});

test('the bridge forwards only page paths and unsubscribes', () => {
  assert.equal(desktopNavigation({}), null);
  const hold: { listener?: (path: unknown) => void } = {};
  let stopped = false;
  const onOpen = desktopNavigation({
    cliphubNavigation: {
      onOpen: (next: (path: unknown) => void) => {
        hold.listener = next;
        return () => {
          stopped = true;
        };
      },
    },
  });
  assert.ok(onOpen);
  const seen: string[] = [];
  const stop = onOpen((path) => seen.push(path));
  hold.listener?.('/api/library/demos');
  hold.listener?.('/replay?r=a:1');
  stop();
  assert.deepEqual(seen, ['/replay?r=a:1']);
  assert.equal(stopped, true);
});
