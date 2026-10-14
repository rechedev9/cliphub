export type LeaseLoss =
  | { kind: "expired"; at: string }
  | { kind: "over_limit"; phase: "running" | "uploading"; limitSeconds: number };

export function encodeLeaseLoss(loss: LeaseLoss): string {
  return JSON.stringify(loss);
}

export function decodeLeaseLoss(raw: string): LeaseLoss | null {
  const trimmed = raw.trim();
  if (!trimmed.startsWith("{")) return null;
  try {
    const parsed: unknown = JSON.parse(trimmed);
    if (!isLeaseLoss(parsed)) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function leaseLossCause(loss: LeaseLoss): string {
  if (loss.kind === "expired") return `lease expired at ${loss.at}`;
  return `${loss.phase} past its limit of ${loss.limitSeconds} s`;
}

function isLeaseLoss(value: unknown): value is LeaseLoss {
  if (typeof value !== "object" || value === null) return false;
  const loss = value as Record<string, unknown>;
  if (loss.kind === "expired") return typeof loss.at === "string";
  return (
    loss.kind === "over_limit" &&
    (loss.phase === "running" || loss.phase === "uploading") &&
    typeof loss.limitSeconds === "number" &&
    Number.isFinite(loss.limitSeconds)
  );
}
