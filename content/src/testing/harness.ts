import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { Client, InMemoryTransport } from "@modelcontextprotocol/client";
import { loadConfig, type Config } from "../config.ts";
import { createContentServer } from "../mcp/server.ts";
import { RenderJobManager } from "../render/client.ts";
import { openStateStore, type StateStore } from "../state/store.ts";
import { probeStudio } from "../studio/client.ts";

export const PACKAGE_DIR = path.resolve(import.meta.dirname, "..", "..");
export const LEGACY_MAPS = path.join(PACKAGE_DIR, "testdata", "legacy", "maps");
export const LEGACY_METRICS = path.join(PACKAGE_DIR, "testdata", "legacy", "metrics");
export const FAKE_STUDIO = path.join(PACKAGE_DIR, "src", "studio", "fake-studio-server.ts");

export function at(value: unknown, dotted: string): unknown {
  let current = value;
  for (const key of dotted.split(".")) {
    if (typeof current !== "object" || current === null) return undefined;
    current = Array.isArray(current) ? current[Number(key)] : Object.entries(current).find(([k]) => k === key)?.[1];
  }
  return current;
}

export type Harness = {
  dir: string;
  config: Config;
  store: StateStore;
  call(name: string, args?: Record<string, unknown>): Promise<{ isError: boolean; body: unknown; text: string }>;
  close(): Promise<void>;
};

export async function openHarness(file: Record<string, unknown> = {}): Promise<Harness> {
  const dir = mkdtempSync(path.join(tmpdir(), "cliphub-content-"));
  const configFile = path.join(dir, "config.local.json");
  writeFileSync(configFile, JSON.stringify({ statePath: path.join(dir, "state.db"), ...file }));
  const config = loadConfig({ env: { CLIPHUB_CONTENT_CONFIG: configFile } });
  const store = openStateStore({ path: config.statePath });
  const renderJobs = new RenderJobManager();
  const server = createContentServer({
    config,
    store,
    renderJobs,
    probeStudio: () => probeStudio({ config: config.studioMcp, timeoutMs: 20_000 }),
  });
  const client = new Client({ name: "cliphub-content-test", version: "0.0.0" });
  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  await server.connect(serverTransport);
  await client.connect(clientTransport);
  return {
    dir,
    config,
    store,
    async call(name, args = {}) {
      const result = await client.callTool({ name, arguments: args });
      const text = result.content.flatMap((block) => (block.type === "text" ? [block.text] : [])).join("\n");
      return { isError: result.isError === true, body: result.structuredContent, text };
    },
    async close() {
      renderJobs.closeAll();
      await client.close();
      await server.close();
      store.close();
      rmSync(dir, { recursive: true, force: true });
    },
  };
}
