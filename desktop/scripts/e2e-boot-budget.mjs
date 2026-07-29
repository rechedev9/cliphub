// Runtime tools provision concurrently, so the slowest supported download
// dominates (FFmpeg: 300s). Service health then has a 60s budget. Keep a
// separate harness margin for extraction, hashing, Electron launch, and loaded
// host contention instead of racing the product's own deadlines.
export const RUNTIME_TOOL_PROVISIONING_BUDGET_MS = 300_000;
export const SERVICE_STARTUP_BUDGET_MS = 60_000;
export const E2E_HARNESS_MARGIN_MS = 60_000;
export const E2E_BOOT_DEADLINE_MS =
  RUNTIME_TOOL_PROVISIONING_BUDGET_MS +
  SERVICE_STARTUP_BUDGET_MS +
  E2E_HARNESS_MARGIN_MS;
