// The type scale lives in globals.css and tailwind-merge has to be told about
// it by hand (see TYPE_SCALE_STEPS in utils.ts); this test is what keeps the
// two from drifting. Without it, adding a `--text-*` step silently
// reintroduces the bug the list exists to fix: tailwind-merge files an unknown
// `text-*` under text-colour, so inside cn() a size step and a colour step
// delete each other.
//
// It asserts the behaviour rather than the list, so nothing has to be exported
// only for a test to read it.
// Run: node --test "lib/**/*.test.ts"
import { strict as assert } from 'node:assert';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { cn } from './utils.ts';

/** `--text-body: …` but not the `--text-body--line-height: …` modifiers. */
const TYPE_SCALE_DECLARATION = /^\s*--text-([a-z0-9-]+):/gm;

function declaredSteps(): string[] {
  const css = readFileSync(fileURLToPath(new URL('../app/globals.css', import.meta.url)), 'utf8');
  const steps = new Set<string>();
  for (const [, step] of css.matchAll(TYPE_SCALE_DECLARATION)) {
    if (step !== undefined && !step.includes('--')) steps.add(step);
  }
  return [...steps];
}

test('globals.css actually declares a type scale', () => {
  // A regex that silently matches nothing would make the test below vacuous.
  assert.ok(declaredSteps().length >= 8, `only found ${declaredSteps().length} steps`);
});

test('every --text-* step survives cn() next to a colour class', () => {
  const dropped = declaredSteps().filter((step) => !cn(`text-${step}`, 'text-fg-2').includes(`text-${step}`));
  assert.deepEqual(dropped, [], `add these to TYPE_SCALE_STEPS in lib/utils.ts: ${dropped.join(', ')}`);
});

test('the colour class survives too', () => {
  assert.ok(cn('text-meta', 'text-fg-2').includes('text-fg-2'));
});

test('two size steps still collapse to the last one', () => {
  assert.equal(cn('text-meta', 'text-display'), 'text-display');
});
