// Next.js calls register() once per server process, in every runtime. The
// node-only side effects live in their own file and are imported behind a
// positive runtime check, so the edge bundle never pulls in node: builtins.
export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    await import("./instrumentation-node");
  }
}
