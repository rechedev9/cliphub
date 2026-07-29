import assert from 'node:assert/strict';
import test from 'node:test';
import {
  E2E_BOOT_DEADLINE_MS,
  E2E_HARNESS_MARGIN_MS,
  RUNTIME_TOOL_PROVISIONING_BUDGET_MS,
  SERVICE_STARTUP_BUDGET_MS,
} from './e2e-boot-budget.mjs';

test('cold-boot harness outlives every supported product boot phase', () => {
  assert.equal(E2E_BOOT_DEADLINE_MS, 420_000);
  assert.ok(E2E_HARNESS_MARGIN_MS > 0);
  assert.ok(
    E2E_BOOT_DEADLINE_MS >
      RUNTIME_TOOL_PROVISIONING_BUDGET_MS + SERVICE_STARTUP_BUDGET_MS,
  );
});
