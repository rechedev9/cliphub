/** Native WebMCP preview. No registry or DOM automation is installed as a fallback. */
export type WebTool = {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
  annotations?: { readOnlyHint?: boolean; consequentialHint?: boolean };
  execute: (args: Record<string, unknown>) => Promise<string> | string;
};

export type ModelContext = {
  registerTool: (tool: WebTool, options: { signal: AbortSignal }) => Promise<void> | void;
};

/** Abort handles React StrictMode, route unmounts and HMR without stale callbacks. */
export function registerWebTools(context: ModelContext | undefined, tools: WebTool[]): () => void {
  const controller = new AbortController();
  if (context) {
    void (async () => {
      try {
        for (const tool of tools) {
          if (controller.signal.aborted) break;
          await context.registerTool(tool, { signal: controller.signal });
        }
      } catch (error) {
        controller.abort();
        // A preview capability must never prevent the ordinary UI from working.
        if (!(error instanceof DOMException && error.name === 'AbortError')) {
          console.warn('ClipHub WebMCP registration unavailable', error);
        }
      }
    })();
  }
  return () => controller.abort();
}

export function previewModelContext(): ModelContext | undefined {
  if (process.env.NEXT_PUBLIC_WEBMCP_PREVIEW !== '1' || typeof document === 'undefined') return;
  if (!['localhost', '127.0.0.1', '[::1]'].includes(location.hostname)) return;
  return (document as Document & { modelContext?: ModelContext }).modelContext;
}

export async function waitForWebState(check: () => boolean): Promise<void> {
  const deadline = Date.now() + 4000;
  while (!check()) {
    if (Date.now() >= deadline) throw new Error('UI state not confirmed; inspect the current state before retrying');
    await new Promise((resolve) => setTimeout(resolve, 20));
  }
}
