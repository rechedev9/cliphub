import { type Json, isRecord, list, record, text } from "./json.ts";

// MCP Apps View side (ext-apps 2026-01-26): JSON-RPC 2.0 over postMessage with the host.
const PROTOCOL_VERSION = "2026-01-26";

export type ToolResult = {
  structured: Json;
  text: string;
  isError: boolean;
};

export type Theme = "light" | "dark" | "";

type Handlers = {
  input: (args: Json) => void;
  result: (result: ToolResult) => void;
  cancelled: () => void;
  teardown: () => void;
  theme: (theme: Theme) => void;
};

type Waiter = { resolve: (value: unknown) => void; reject: (err: Error) => void };

export function readToolResult(value: unknown): ToolResult {
  const r = record(value);
  const blocks = list(r.content).filter((c) => c.type === "text").map((c) => text(c.text));
  return { structured: record(r.structuredContent), text: blocks.join("\n"), isError: r.isError === true };
}

function readTheme(context: unknown): Theme {
  const theme = record(context).theme;
  return theme === "light" || theme === "dark" ? theme : "";
}

export class Bridge {
  private nextId = 1;
  private pending = new Map<number, Waiter>();
  private serverTools = false;
  private handlers: Partial<Handlers> = {};
  private seenInput: Json | undefined;
  private seenResult: ToolResult | undefined;

  constructor() {
    window.addEventListener("message", (event) => this.receive(event));
  }

  on<K extends keyof Handlers>(kind: K, fn: Handlers[K]): void {
    this.handlers[kind] = fn;
  }

  // Notifications can land before React subscribes; replay what came first.
  replay(): void {
    if (this.seenInput) this.handlers.input?.(this.seenInput);
    if (this.seenResult) this.handlers.result?.(this.seenResult);
  }

  canCallTools(): boolean {
    return this.serverTools;
  }

  async start(name: string): Promise<Theme> {
    const init = record(
      await this.request("ui/initialize", { appInfo: { name, version: "1.0.0" }, appCapabilities: {}, protocolVersion: PROTOCOL_VERSION }),
    );
    this.serverTools = isRecord(record(init.hostCapabilities).serverTools);
    this.send({ method: "ui/notifications/initialized", params: {} });
    return readTheme(init.hostContext);
  }

  async callTool(name: string, args: Json): Promise<ToolResult> {
    return readToolResult(await this.request("tools/call", { name, arguments: args }));
  }

  // The body wraps the whole view, border included, so its box is the size the host needs.
  observeSize(node: HTMLElement): () => void {
    const report = () => {
      const box = node.getBoundingClientRect();
      this.send({ method: "ui/notifications/size-changed", params: { width: Math.ceil(box.width), height: Math.ceil(box.height) } });
    };
    const observer = new ResizeObserver(report);
    observer.observe(node);
    report();
    return () => observer.disconnect();
  }

  private send(message: Json): void {
    window.parent.postMessage({ jsonrpc: "2.0", ...message }, "*");
  }

  private request(method: string, params: Json): Promise<unknown> {
    const id = this.nextId++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.send({ id, method, params });
    });
  }

  private receive(event: MessageEvent): void {
    if (event.source !== window.parent || !isRecord(event.data) || event.data.jsonrpc !== "2.0") return;
    const m = event.data;
    if (m.method === undefined) {
      const waiter = typeof m.id === "number" ? this.pending.get(m.id) : undefined;
      if (!waiter || typeof m.id !== "number") return;
      this.pending.delete(m.id);
      if (isRecord(m.error)) waiter.reject(new Error(text(m.error.message) || "error del anfitrión"));
      else waiter.resolve(m.result);
      return;
    }
    const params = record(m.params);
    switch (m.method) {
      case "ui/notifications/tool-input":
        this.seenInput = record(params.arguments);
        this.handlers.input?.(this.seenInput);
        break;
      case "ui/notifications/tool-result":
        this.seenResult = readToolResult(params);
        this.handlers.result?.(this.seenResult);
        break;
      case "ui/notifications/tool-cancelled":
        this.handlers.cancelled?.();
        break;
      case "ui/notifications/host-context-changed":
        if (readTheme(params)) this.handlers.theme?.(readTheme(params));
        break;
    }
    if (m.method === "ui/resource-teardown") this.handlers.teardown?.();
    if (m.id === undefined) return;
    // Teardown and ping expect an empty result; anything else is unsupported.
    if (m.method === "ui/resource-teardown" || m.method === "ping") this.send({ id: m.id, result: {} });
    else this.send({ id: m.id, error: { code: -32601, message: "method not found" } });
  }
}
