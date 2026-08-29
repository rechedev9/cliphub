#!/usr/bin/env bash
# VM evidence for Full Demo 16:9. Does not launch HLAE or CS2.
# Usage: scripts/full-demo-vm-evidence.sh [evidence-dir]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-/opt/cursor/artifacts/full-demo-live-rounds}"
mkdir -p "$OUT"

echo "=== environment gap ==="
echo "HLAE/CS2/Windows Studio capture/render: NOT AVAILABLE on this host"
echo "This script is not a production capture/render certification."

echo "=== overlay compositor + fixture capture + shorts portrait ==="
export FULL_DEMO_OVERLAY_OUT="$OUT"
(
  cd "$ROOT"
  go test ./internal/parser -run 'TestSegmentRecapIsOneContinuousLiveRoundNotAJumpCut|TestCollectorRecapPistolRoundStaysOneContinuousLiveWindow|TestWithIntroFreeze|TestWithOutroHold' -count=1 -v
  go test ./internal/recording -run 'TestGenerateHLAEJavaScriptLocksFirstPersonOnOneRecapWindow|TestNewPlanFromKillPlanDropsRecapEventsOutsideLiveWindow' -count=1 -v
  go test ./internal/editor -run 'TestFullDemoOverlayCompositesOntoFixtureCapture|TestShortsParsePlanPortraitSeam|TestBuildManifestShortsPathIgnoresFullDemoOverlay|TestBuildManifestFullDemoAttachesIntroAndOutroOverlays' -count=1 -timeout 3m -v
  go test ./internal/workers -run 'TestRenderWorkerNativePOVDropsMusicBed|TestRenderWorkerPassesFullDemoOverlay' -count=1 -v
  go test ./internal/demooverlay -run 'TestRenderPNGsWritesIntroAndOutroStills|TestOverlayWindowsStartsAfterFadeAndLeavesBeforeLive|TestDefaultLayoutKeepsNativeHUDChannelAndFullFrameOutro' -count=1 -v
) | tee "$OUT/go-invariants.log"

echo "=== web invariants ==="
(
  cd "$ROOT"
  node --experimental-strip-types --test --test-reporter=tap \
    web/lib/full-demo.test.ts web/lib/api/reel-store.test.ts
) | tee "$OUT/web-invariants.log"

if [[ -d "$ROOT/web/node_modules/@playwright/test" ]]; then
  echo "=== studio full-demo e2e ==="
  (
    cd "$ROOT/web"
    E2E_SKIP_BUILD=1 pnpm exec playwright test e2e/full-demo.spec.ts
  ) | tee "$OUT/playwright-full-demo.log"
else
  echo "playwright not installed; studio e2e skipped" | tee "$OUT/playwright-full-demo.log"
fi

echo "=== cli (no HLAE) ==="
(
  cd "$ROOT"
  go run ./cmd/zv presets --format json
  echo
  go run ./cmd/zv flows show demo --format json
) | tee "$OUT/cli-no-hlae.log"

echo "=== evidence written to $OUT ==="
ls -la "$OUT"
