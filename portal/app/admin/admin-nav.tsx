"use client";

import { usePathname } from "next/navigation";
import { useEffect } from "react";

import { Badge } from "@/components/ui/badge";

import type { AdminCounts } from "../components/api-types";
import { parseAdminCounts } from "../components/parse";
import { ACTION_SETTLED_EVENT } from "../components/use-action";
import { usePoll } from "../components/use-poll";

interface NavLink {
  href: string;
  label: string;
  // What waits for a decision behind this link, shown as a badge when above zero.
  waiting?: (counts: AdminCounts) => number;
  waitingLabel?: string;
}

const LINKS: NavLink[] = [
  {
    href: "/admin",
    label: "Solicitudes y cola",
    waiting: (counts) => counts.requests,
    waitingLabel: "solicitudes por aprobar",
  },
  { href: "/admin/history", label: "Historial" },
  {
    href: "/admin/users",
    label: "Usuarios",
    waiting: (counts) => counts.pendingUsers,
    waitingLabel: "personas esperando acceso",
  },
  { href: "/admin/workers", label: "Workers" },
  { href: "/admin/activity", label: "Actividad reciente" },
];

const LINK = [
  "-mb-px inline-flex items-center gap-1.5 border-b-2 border-transparent px-2.5 py-2.5 sm:px-3.5",
  "text-body-sm font-medium whitespace-nowrap text-fg-2",
  "transition-colors duration-(--dur-fast) hover:text-fg-1",
  "aria-[current=page]:border-primary aria-[current=page]:text-primary",
].join(" ");

function isActive(pathname: string, href: string): boolean {
  if (href === "/admin") return pathname === "/admin";
  if (href === "/admin/history") return pathname.startsWith(href) || pathname.startsWith("/admin/jobs");
  return pathname.startsWith(href);
}

export function AdminNav() {
  const pathname = usePathname();
  // A failed load only hides the badges; every page still shows its own state.
  const { data: counts, refresh } = usePoll({ url: "/api/admin/counts", intervalMs: 15000, parse: parseAdminCounts });
  // An approval or a rejection changes what waits; the badges must not lag behind it.
  useEffect(() => {
    window.addEventListener(ACTION_SETTLED_EVENT, refresh);
    return () => window.removeEventListener(ACTION_SETTLED_EVENT, refresh);
  }, [refresh]);
  return (
    // Wraps on a phone so the whole navigation stays in view.
    <nav className="mb-6 flex flex-wrap gap-x-1 border-b border-border-subtle" aria-label="Panel de control">
      {LINKS.map((link) => {
        const waiting = counts === null || link.waiting === undefined ? 0 : link.waiting(counts);
        return (
          <a
            key={link.href}
            href={link.href}
            className={LINK}
            aria-current={isActive(pathname, link.href) ? "page" : undefined}
          >
            {link.label}
            {waiting > 0 && (
              <Badge variant="warning" className="font-mono tabular-nums">
                {waiting}
                <span className="sr-only"> {link.waitingLabel}</span>
              </Badge>
            )}
          </a>
        );
      })}
    </nav>
  );
}
