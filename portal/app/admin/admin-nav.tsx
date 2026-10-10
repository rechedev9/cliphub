"use client";

import { usePathname } from "next/navigation";

const LINKS = [
  { href: "/admin", label: "Cola" },
  { href: "/admin/history", label: "Historial" },
  { href: "/admin/users", label: "Usuarios" },
  { href: "/admin/workers", label: "Workers" },
  { href: "/admin/activity", label: "Actividad reciente" },
];

const LINK = [
  "-mb-px border-b-2 border-transparent px-2.5 py-2.5 text-body-sm font-medium whitespace-nowrap text-fg-2 sm:px-3.5",
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
  return (
    // Wraps on a phone so the whole navigation stays in view.
    <nav className="mb-6 flex flex-wrap gap-x-1 border-b border-border-subtle" aria-label="Panel de control">
      {LINKS.map((link) => (
        <a
          key={link.href}
          href={link.href}
          className={LINK}
          aria-current={isActive(pathname, link.href) ? "page" : undefined}
        >
          {link.label}
        </a>
      ))}
    </nav>
  );
}
