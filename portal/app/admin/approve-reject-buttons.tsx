"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export function ApproveRejectButtons({ requestId }: { requestId: string }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  async function act(action: "approve" | "reject") {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(`/api/admin/requests/${requestId}/${action}`, {
        method: "POST",
      });
      if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error ?? `No se pudo ${action === "approve" ? "aprobar" : "rechazar"}.`);
      }
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error desconocido.");
      setBusy(false);
    }
  }

  return (
    <div>
      <button type="button" disabled={busy} onClick={() => act("approve")}>
        Aprobar
      </button>{" "}
      <button
        type="button"
        className="secondary"
        disabled={busy}
        onClick={() => act("reject")}
      >
        Rechazar
      </button>
      {error && (
        <p className="error-text" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
