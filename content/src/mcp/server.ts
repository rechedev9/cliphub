import { McpServer } from "@modelcontextprotocol/server";
import { z } from "zod";
import type { Config } from "../config.ts";
import { importLegacyState, LegacyImportError } from "../legacy/import.ts";
import { applyMapEvent, mapStage, povStatus, shortStatus, type MapEvent, type MapRecord } from "../state/model.ts";
import type { StateStore } from "../state/store.ts";
import type { StudioProbe } from "../studio/client.ts";

export type ContentServerDeps = {
  config: Config;
  store: StateStore;
  probeStudio: () => Promise<StudioProbe>;
};

export const SERVER_INFO = { name: "cliphub-content", version: "0.1.0" };

const INSTRUCTIONS = `cliphub-content keeps the state of the ClipHub YouTube channel's content: one record per map POV
(source event, faceit or clip), its Shorts, the YouTube upload receipts and raw metric snapshots, in a local SQLite file.
A map's stage (open, ready, done, cleaned, skipped) and each Short's status are derived from its fields; change them only
with record_map_event, which refuses an illegal step (for example cleaning a map that is not done). content_status says
whether the config was found, where the state lives, its schema version and whether the Studio MCP (zv-mcp) answers.
import_legacy_state reads the old map.json folders and metrics files; it is a dry run unless write is true.`;

const stageKinds = z.enum(["open", "ready", "done", "cleaned", "skipped"]);

const mapEvent = z.discriminatedUnion("type", [
  z.object({ type: z.literal("pov_published"), youtubeId: z.string().min(1), publishedAt: z.iso.datetime({ offset: true }) }),
  z.object({ type: z.literal("pov_skipped"), reason: z.string().min(1) }),
  z.object({ type: z.literal("short_published"), position: z.int().positive(), youtubeId: z.string().min(1), publishedAt: z.iso.datetime({ offset: true }) }),
  z.object({ type: z.literal("short_related_linked"), position: z.int().positive() }),
  z.object({ type: z.literal("short_thumbnail_checked"), position: z.int().positive() }),
  z.object({ type: z.literal("short_youtube_skipped"), position: z.int().positive(), reason: z.string().min(1) }),
  z.object({ type: z.literal("map_skipped"), at: z.iso.datetime({ offset: true }), reason: z.string().min(1) }),
  z.object({ type: z.literal("map_unskipped") }),
  z.object({ type: z.literal("map_cleaned"), at: z.iso.datetime({ offset: true }) }),
]);

type ToolResult = { content: { type: "text"; text: string }[]; structuredContent: Record<string, unknown>; isError?: boolean };

function ok(body: Record<string, unknown>): ToolResult {
  return { content: [{ type: "text", text: JSON.stringify(body, null, 2) }], structuredContent: body };
}

function failure(message: string): ToolResult {
  return { content: [{ type: "text", text: message }], structuredContent: { error: message }, isError: true };
}

function summary(map: MapRecord): Record<string, unknown> {
  return {
    id: map.id,
    source: map.source,
    player: map.player,
    map: map.mapName,
    stage: mapStage(map),
    pov: povStatus(map.pov),
    shorts: map.shorts.map((short) => ({ position: short.position, round: short.round, status: shortStatus(short) })),
  };
}

function detail(map: MapRecord): Record<string, unknown> {
  return { ...summary(map), record: map };
}

async function status(deps: ContentServerDeps): Promise<ToolResult> {
  const studio = await deps.probeStudio();
  return ok({
    config: { found: deps.config.configFile !== null, file: deps.config.configFile },
    state: { path: deps.store.path, schemaVersion: deps.store.schemaVersion(), counts: deps.store.counts() },
    legacy: { mapsDir: deps.config.legacyMapsDir, metricsDir: deps.config.legacyMetricsDir },
    studio: {
      configured: studio.kind !== "not_configured",
      answers: studio.kind === "answered",
      ...(studio.kind === "answered" ? { status: studio.status } : {}),
      ...(studio.kind === "failed" ? { error: studio.error } : {}),
    },
  });
}

function recordEvent(store: StateStore, request: { mapId: string; event: MapEvent }): ToolResult {
  const { mapId, event } = request;
  return store.transaction({ commit: true }, () => {
    const map = store.getMap(mapId);
    if (map === null) return failure(`no map ${mapId} in the state`);
    const result = applyMapEvent(map, event);
    if (!result.ok) return failure(result.error);
    store.saveMap(result.map);
    return ok(summary(result.map));
  });
}

export function createContentServer(deps: ContentServerDeps): McpServer {
  const server = new McpServer(SERVER_INFO, { capabilities: { tools: {} }, instructions: INSTRUCTIONS });
  const { store, config } = deps;

  server.registerTool(
    "content_status",
    { description: "Config found, state DB path and schema version, row counts, and whether the Studio MCP answers studio_status.", annotations: { readOnlyHint: true } },
    () => status(deps),
  );

  server.registerTool(
    "list_maps",
    {
      description: "Every map POV in the state with its derived stage, POV status and Short statuses. Filter by stage or source.",
      inputSchema: z.object({ stage: stageKinds.optional(), source: z.enum(["event", "faceit", "clip"]).optional() }),
      annotations: { readOnlyHint: true },
    },
    ({ stage, source }) => {
      const maps = store
        .listMaps()
        .filter((map) => (stage === undefined || mapStage(map).kind === stage) && (source === undefined || map.source === source));
      return ok({ maps: maps.map(summary) });
    },
  );

  server.registerTool(
    "get_map",
    { description: "One map's full record with its derived stage and statuses.", inputSchema: z.object({ id: z.string().min(1) }), annotations: { readOnlyHint: true } },
    ({ id }) => {
      const map = store.getMap(id);
      return map === null ? failure(`no map ${id} in the state`) : ok(detail(map));
    },
  );

  server.registerTool(
    "record_map_event",
    {
      description: "Record one step of a map (a POV or Short published, a Short linked or its thumbnail checked, a skip, a clean). Illegal steps are refused with the reason.",
      inputSchema: z.object({ mapId: z.string().min(1), event: mapEvent }),
    },
    (request) => recordEvent(store, request),
  );

  server.registerTool(
    "import_legacy_state",
    {
      description: "Import the legacy JSON state (map.json folders with their *.youtube.json receipts, channel-snapshots.jsonl and videos-latest.json) from the configured folders. Only reads those folders. A dry run unless write is true.",
      inputSchema: z.object({ write: z.boolean().default(false) }),
    },
    ({ write }) => {
      try {
        const report = importLegacyState({ store, mapsDir: config.legacyMapsDir, metricsDir: config.legacyMetricsDir, dryRun: !write });
        return ok(report);
      } catch (error) {
        if (error instanceof LegacyImportError) return failure(error.message);
        throw error;
      }
    },
  );

  return server;
}
