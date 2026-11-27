import { McpServer } from "@modelcontextprotocol/server";
import { serveStdio } from "@modelcontextprotocol/server/stdio";

const FAKE_STUDIO_STATUS = { studio_open: true, version: "fake-studio", capture_available: false };

serveStdio(() => {
  const server = new McpServer({ name: "fake-cliphub-studio", version: "0.0.0" }, { capabilities: { tools: {} } });
  server.registerTool("studio_status", { description: "Fake Studio status for cliphub-content tests" }, async () => ({
    content: [{ type: "text", text: JSON.stringify(FAKE_STUDIO_STATUS) }],
  }));
  return server;
});
