import { Client } from "@modelcontextprotocol/client";
import { StdioClientTransport } from "@modelcontextprotocol/client/stdio";
import type { StudioMcpConfig } from "../config.ts";

export type StudioProbe =
  | { kind: "not_configured" }
  | { kind: "answered"; status: unknown }
  | { kind: "failed"; error: string };

type StudioCall = { tool: string; arguments: Record<string, unknown> };

const CLIENT_INFO = { name: "cliphub-content", version: "0.1.0" };

function payload(result: Awaited<ReturnType<Client["callTool"]>>): unknown {
  if (result.structuredContent !== undefined) return result.structuredContent;
  const texts = result.content.flatMap((block) => (block.type === "text" ? [block.text] : []));
  const joined = texts.join("\n");
  try {
    return JSON.parse(joined);
  } catch {
    return joined;
  }
}

function withTimeout<T>(work: Promise<T>, limit: { timeoutMs: number; what: string }): Promise<T> {
  const { timeoutMs, what } = limit;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${what} took longer than ${timeoutMs} ms`)), timeoutMs);
  });
  return Promise.race([work, timeout]).finally(() => clearTimeout(timer));
}

async function callStudio(options: { config: StudioMcpConfig; call: StudioCall; timeoutMs: number }): Promise<unknown> {
  const { config, call, timeoutMs } = options;
  const client = new Client(CLIENT_INFO);
  const transport = new StdioClientTransport({ command: config.command, args: config.args, stderr: "ignore" });
  try {
    await withTimeout(client.connect(transport), { timeoutMs, what: "connecting to the Studio MCP" });
    const result = await withTimeout(client.callTool({ name: call.tool, arguments: call.arguments }), { timeoutMs, what: call.tool });
    const body = payload(result);
    if (result.isError === true) throw new Error(`${call.tool} failed: ${typeof body === "string" ? body : JSON.stringify(body)}`);
    return body;
  } finally {
    await client.close().catch(() => undefined);
  }
}

export async function probeStudio(options: { config: StudioMcpConfig | null; timeoutMs: number }): Promise<StudioProbe> {
  if (options.config === null) return { kind: "not_configured" };
  try {
    const status = await callStudio({ config: options.config, call: { tool: "studio_status", arguments: {} }, timeoutMs: options.timeoutMs });
    return { kind: "answered", status };
  } catch (error) {
    return { kind: "failed", error: error instanceof Error ? error.message : String(error) };
  }
}
