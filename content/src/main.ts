import { serveStdio } from "@modelcontextprotocol/server/stdio";
import { loadConfig } from "./config.ts";
import { createContentServer } from "./mcp/server.ts";
import { openStateStore } from "./state/store.ts";
import { probeStudio } from "./studio/client.ts";

const STUDIO_TIMEOUT_MS = 15_000;

const config = loadConfig({ env: process.env });
const store = openStateStore({ path: config.statePath });

serveStdio(
  () => createContentServer({ config, store, probeStudio: () => probeStudio({ config: config.studioMcp, timeoutMs: STUDIO_TIMEOUT_MS }) }),
  { onerror: (error) => process.stderr.write(`cliphub-content: ${error.message}\n`) },
);
