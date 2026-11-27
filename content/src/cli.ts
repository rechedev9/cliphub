import { Client, InMemoryTransport } from "@modelcontextprotocol/client";
import { loadConfig } from "./config.ts";
import { createContentServer, SERVER_INFO } from "./mcp/server.ts";
import { openStateStore } from "./state/store.ts";
import { probeStudio } from "./studio/client.ts";

const USAGE = `usage: node src/cli.ts <tool> [json arguments]
       node src/cli.ts tools
Runs one cliphub-content MCP tool in process and prints its result.`;

const STUDIO_TIMEOUT_MS = 15_000;

function parseArguments(raw: string | undefined): Record<string, unknown> {
  if (raw === undefined) return {};
  const parsed: unknown = JSON.parse(raw);
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) throw new Error("arguments must be a JSON object");
  return { ...parsed };
}

async function run(argv: string[]): Promise<number> {
  const [tool, raw] = argv;
  if (tool === undefined || tool === "--help") {
    process.stdout.write(`${USAGE}\n`);
    return tool === undefined ? 2 : 0;
  }
  const config = loadConfig({ env: process.env });
  const store = openStateStore({ path: config.statePath });
  const server = createContentServer({ config, store, probeStudio: () => probeStudio({ config: config.studioMcp, timeoutMs: STUDIO_TIMEOUT_MS }) });
  const client = new Client({ name: `${SERVER_INFO.name}-cli`, version: SERVER_INFO.version });
  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  try {
    await server.connect(serverTransport);
    await client.connect(clientTransport);
    if (tool === "tools") {
      const { tools } = await client.listTools();
      for (const t of tools) process.stdout.write(`${t.name}\t${t.description ?? ""}\n`);
      return 0;
    }
    const result = await client.callTool({ name: tool, arguments: parseArguments(raw) });
    for (const block of result.content) if (block.type === "text") process.stdout.write(`${block.text}\n`);
    return result.isError === true ? 1 : 0;
  } finally {
    await client.close();
    await server.close();
    store.close();
  }
}

process.exitCode = await run(process.argv.slice(2));
