"use client";

import { useEffect, useState } from "react";
import Wordmark from "@/components/wordmark";

interface SiteHeaderProps {
  nav: readonly { href: string; label: string }[];
  downloadUrl: string;
  ctaLabel: string;
}

export default function SiteHeader({ nav, downloadUrl, ctaLabel }: SiteHeaderProps) {
  const [scrolled, setScrolled] = useState(false);
  const [pastHero, setPastHero] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    const onScroll = () => {
      setScrolled(window.scrollY > 24);
      setPastHero(window.scrollY > window.innerHeight * 0.85);
    };
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setMenuOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [menuOpen]);

  return (
    <>
      <header
        className={`fixed inset-x-0 top-0 z-40 transition-colors duration-300 ${
          scrolled || menuOpen ? "border-b border-line bg-bg/95 backdrop-blur" : "border-b border-transparent"
        }`}
      >
        <div className="mx-auto flex h-16 w-full max-w-7xl items-center justify-between gap-6 px-5 sm:h-20 sm:px-10">
          <a href="#" aria-label="ClipHub Studio, inicio">
            <Wordmark />
          </a>
          <nav aria-label="Secciones" className="hidden items-center gap-8 lg:flex">
            {nav.map((item) => (
              <a
                key={item.href}
                href={item.href}
                className="font-display text-sm font-bold uppercase tracking-[0.2em] text-fg/75 transition-colors hover:text-brand"
              >
                {item.label}
              </a>
            ))}
          </nav>
          <div className="flex items-center gap-4">
            <a
              href={downloadUrl}
              className="hud-cut hidden min-h-11 items-center bg-brand px-5 font-display text-sm font-extrabold uppercase tracking-[0.18em] text-bg transition-colors hover:bg-fg sm:inline-flex"
            >
              Descargar
            </a>
            <button
              type="button"
              aria-expanded={menuOpen}
              aria-controls="menu-secciones"
              onClick={() => setMenuOpen((open) => !open)}
              className="min-h-11 font-display text-sm font-bold uppercase tracking-[0.2em] text-fg lg:hidden"
            >
              {menuOpen ? "Cerrar" : "Menú"}
            </button>
          </div>
        </div>
        <nav
          id="menu-secciones"
          aria-label="Secciones"
          hidden={!menuOpen}
          className="mx-auto w-full max-w-7xl px-5 pb-6 sm:px-10 lg:hidden"
        >
          <ul className="grid gap-1">
            {nav.map((item) => (
              <li key={item.href}>
                <a
                  href={item.href}
                  onClick={() => setMenuOpen(false)}
                  className="flex min-h-12 items-center border-b border-line font-display text-base font-bold uppercase tracking-[0.2em] text-fg/80 hover:text-brand"
                >
                  {item.label}
                </a>
              </li>
            ))}
            <li className="sm:hidden">
              <a
                href={downloadUrl}
                className="flex min-h-12 items-center font-display text-base font-bold uppercase tracking-[0.2em] text-brand"
              >
                Descargar
              </a>
            </li>
          </ul>
        </nav>
      </header>
      <div
        aria-hidden={!pastHero}
        className={`fixed inset-x-0 bottom-0 z-40 border-t border-line bg-bg/95 p-3 backdrop-blur transition-transform duration-300 sm:hidden ${
          pastHero ? "translate-y-0" : "translate-y-full"
        }`}
      >
        <a
          href={downloadUrl}
          tabIndex={pastHero ? 0 : -1}
          className="hud-cut flex min-h-12 items-center justify-center bg-brand font-display text-sm font-extrabold uppercase tracking-[0.2em] text-bg"
        >
          {ctaLabel}
        </a>
      </div>
    </>
  );
}
