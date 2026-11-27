import { createServer, type Server } from "node:http";
import { after } from "node:test";

export async function fakeCommunity(registered: Set<number>, status = 200): Promise<{ url: string; calls: string[]; server: Server }> {
  const calls: string[] = [];
  const server = createServer((req, res) => {
    calls.push(req.url ?? "");
    const url = new URL(req.url ?? "/", "http://x");
    if (url.pathname !== "/v1/pro-match-ids" || status !== 200) {
      res.writeHead(status === 200 ? 404 : status).end("{}");
      return;
    }
    const ids = (url.searchParams.get("ids") ?? "").split(",").map(Number);
    res.writeHead(200, { "Content-Type": "application/json" }).end(JSON.stringify({ registered: ids.filter((id) => registered.has(id)) }));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const addr = server.address();
  if (addr === null || typeof addr === "string") throw new Error("no port");
  after(() => server.close());
  return { url: `http://127.0.0.1:${addr.port}`, calls, server };
}
