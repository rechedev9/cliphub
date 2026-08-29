import type { TelemetryReleaseConfig } from './telemetry-client.ts';

// The ingest key is intentionally public, like an error-tracker DSN: it is an
// abuse filter, not an administrator credential. The private agent token is
// never compiled into ClipHub Studio.
export const PACKAGED_TELEMETRY_CONFIG: TelemetryReleaseConfig = {
  endpoint: 'https://hetzner-openclaw.taila10698.ts.net:8443/v1/ingest',
  ingestKey: '66fc3bcc68f39de6d8c5b759360034a64c496f6f663266dd',
};
