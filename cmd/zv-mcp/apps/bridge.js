// Minimal MCP Apps (ext-apps 2026-01-26) View: JSON-RPC 2.0 over window.postMessage with the host.
const bridge = (() => {
  const PROTOCOL_VERSION = "2026-01-26";
  const pending = new Map();
  const listeners = { input: [], result: [] };
  const teardownHandlers = [];
  const received = {};
  let nextId = 1;
  let hostCaps = {};

  const send = (msg) => parent.postMessage({ jsonrpc: "2.0", ...msg }, "*");

  function request(method, params) {
    return new Promise((resolve, reject) => {
      const id = nextId++;
      pending.set(id, { resolve, reject });
      send({ id, method, params });
    });
  }

  function deliver(kind, value) {
    received[kind] = value;
    listeners[kind].forEach((fn) => fn(value));
  }

  window.addEventListener("message", (event) => {
    if (event.source !== parent) return;
    const m = event.data;
    if (!m || m.jsonrpc !== "2.0") return;
    if (m.method === undefined) {
      const waiter = pending.get(m.id);
      if (!waiter) return;
      pending.delete(m.id);
      if (m.error) waiter.reject(new Error(m.error.message || "host error"));
      else waiter.resolve(m.result);
      return;
    }
    if (m.method === "ui/notifications/tool-input") deliver("input", (m.params || {}).arguments || {});
    else if (m.method === "ui/notifications/tool-result") deliver("result", m.params || {});
    if (m.id === undefined) return;
    // Teardown and ping expect an empty result; anything else is unsupported.
    if (m.method === "ui/resource-teardown") {
      teardownHandlers.forEach((fn) => fn());
      send({ id: m.id, result: {} });
    } else if (m.method === "ping") send({ id: m.id, result: {} });
    else send({ id: m.id, error: { code: -32601, message: "method not found" } });
  });

  function reportSize() {
    send({ method: "ui/notifications/size-changed", params: { width: Math.ceil(document.documentElement.scrollWidth), height: Math.ceil(document.body.scrollHeight + 24) } });
  }

  async function start(name) {
    try {
      const init = await request("ui/initialize", { appInfo: { name, version: "1.0.0" }, appCapabilities: {}, protocolVersion: PROTOCOL_VERSION });
      hostCaps = (init && init.hostCapabilities) || {};
      send({ method: "ui/notifications/initialized", params: {} });
    } catch (err) {
      return false;
    }
    new ResizeObserver(reportSize).observe(document.body);
    reportSize();
    return true;
  }

  function on(kind, fn) {
    listeners[kind].push(fn);
    if (received[kind] !== undefined) fn(received[kind]);
  }

  return {
    start,
    onToolInput: (fn) => on("input", fn),
    onToolResult: (fn) => on("result", fn),
    canCallTools: () => Boolean(hostCaps.serverTools),
    callTool: (name, args) => request("tools/call", { name, arguments: args }),
    onTeardown: (fn) => teardownHandlers.push(fn),
    // The SDK schema (McpUiMessageRequest) takes content as an array of blocks and
    // answers { isError: true } when the host refuses.
    sendMessage: (text) => request("ui/message", { role: "user", content: [{ type: "text", text }] }).then((res) => {
      if (res && res.isError) throw new Error("el asistente rechazó el mensaje");
      return res;
    }),
  };
})();

function el(tag, props, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(props || {})) {
    if (key === "class") node.className = value;
    else if (key.startsWith("on")) node.addEventListener(key.slice(2), value);
    else node.setAttribute(key, value);
  }
  for (const child of children.flat()) {
    if (child !== null && child !== undefined) node.append(child);
  }
  return node;
}

// Tool results hold text blocks only for the model; the UI reads the structured part when present.
function resultText(result) {
  return ((result && result.content) || []).filter((c) => c.type === "text").map((c) => c.text).join("\n");
}

function dataUrl(block, kind) {
  const ok = block && block.type === kind && new RegExp("^" + kind + "/[a-z0-9.+-]+$", "i").test(block.mimeType || "");
  return ok && typeof block.data === "string" ? "data:" + block.mimeType + ";base64," + block.data : "";
}
