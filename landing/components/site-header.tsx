"use client";

import { useEffect, useState } from "react";
import Wordmark from "@/components/wordmark";

interface SiteHeaderProps {
  nav: readonly { href: string; label: string }[];
  appUrl: string;
  ctaLabel: string;
}

/** Transparent over the hero, solid once the page scrolls; phones also get a bottom bar with the main action. */
export default function SiteHeader({ nav, appUrl, ctaLabel }: SiteHeaderProps) {
  const [scrolled, setScrolled] = useState(false);
  const [pastHero, setPastHero] = useState(false);

  useEffect(() => {
    const onScroll = () => {
      setScrolled(window.scrollY > 24);
      setPastHero(window.scrollY > window.innerHeight * 0.85);
    };
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <>
      <header
        className={`fixed inset-x-0 top-0 z-40 transition-colors duration-300 ${
          scrolled ? "border-b border-line bg-bg/90 backdrop-blur" : "border-b border-transparent"
        }`}
      >
        <div className="mx-auto flex h-16 w-full max-w-7xl items-center justify-between gap-6 px-5 sm:h-20 sm:px-10">
          <a href="#" aria-label="ClipHub, inicio">
            <Wordmark />
          </a>
          <nav aria-label="Secciones" className="hidden items-center gap-8 lg:flex">
            {nav.map((item) => (
              <a
                key={item.href}
                href={item.href}
                className="font-display text-sm font-bold uppercase tracking-[0.2em] text-fg/80 transition-colors hover:text-fg"
              >
                {item.label}
              </a>
            ))}
          </nav>
          <a
            href={appUrl}
            className="font-display text-sm font-bold uppercase tracking-[0.2em] text-fg underline decoration-2 underline-offset-8 hover:decoration-brand"
          >
            Abrir ClipHub
          </a>
        </div>
      </header>
      <div
        aria-hidden={!pastHero}
        className={`fixed inset-x-0 bottom-0 z-40 border-t border-line bg-bg/95 p-3 backdrop-blur transition-transform duration-300 sm:hidden ${
          pastHero ? "translate-y-0" : "translate-y-full"
        }`}
      >
        <a
          href={appUrl}
          tabIndex={pastHero ? 0 : -1}
          className="flex min-h-12 items-center justify-center border-2 border-fg bg-fg font-display text-sm font-bold uppercase tracking-[0.2em] text-bg"
        >
          {ctaLabel}
        </a>
      </div>
    </>
  );
}
