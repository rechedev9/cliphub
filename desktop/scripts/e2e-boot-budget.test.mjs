import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import {
  E2E_BOOT_DEADLINE_MS,
  RUNTIME_TOOL_PROVISIONING_BUDGET_MS,
  SERVICE_STARTUP_BUDGET_MS,
} from './e2e-boot-budget.mjs';

function productSource(relativePath) {
  return readFileSync(new URL(`../src/${relativePath}`, import.meta.url), 'utf8');
}

function numericLiterals(source, pattern) {
  return [...source.matchAll(pattern)].map((match) => Number(match[1].replaceAll('_', '')));
}

test('cold-boot budget copies the product runtime-tool and service-health deadlines', () => {
  // Runtime tools provision concurrently, so the slowest pinned tool bounds
  // that phase. Main then waits BOOT_HEALTH_TIMEOUT_MS for service health.
  const toolTimeouts = numericLiterals(
    productSource('runtime-tools.ts') + productSource('hlae-tool.ts'),
    /\btimeoutMs:\s*([\d_]+)\s*,/g,
  );
  const [bootHealthTimeout] = numericLiterals(
    productSource('main.ts'),
    /\bconst BOOT_HEALTH_TIMEOUT_MS\s*=\s*([\d_]+)\s*;/g,
  );

  assert.ok(toolTimeouts.length >= 3, 'every pinned runtime tool declares a timeout');
  assert.equal(RUNTIME_TOOL_PROVISIONING_BUDGET_MS, Math.max(...toolTimeouts));
  assert.equal(SERVICE_STARTUP_BUDGET_MS, bootHealthTimeout);
  assert.ok(E2E_BOOT_DEADLINE_MS > Math.max(...toolTimeouts) + bootHealthTimeout);
});
