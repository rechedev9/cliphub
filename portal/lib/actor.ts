export type Actor =
  | { kind: "system" }
  | { kind: "worker"; id: string }
  | { kind: "admin"; id: string }
  | { kind: "user"; id: string };

export function formatActor(actor: Actor): string {
  switch (actor.kind) {
    case "system":
      return "system";
    case "worker":
      return `worker:${actor.id}`;
    case "admin":
      return `admin:${actor.id}`;
    case "user":
      return `user:${actor.id}`;
  }
}

export function parseActor(raw: string): Actor | null {
  if (raw === "system") return { kind: "system" };
  const separator = raw.indexOf(":");
  if (separator <= 0) return null;
  const kind = raw.slice(0, separator);
  const id = raw.slice(separator + 1);
  if (id === "") return null;
  if (kind === "worker" || kind === "admin" || kind === "user") return { kind, id };
  return null;
}
