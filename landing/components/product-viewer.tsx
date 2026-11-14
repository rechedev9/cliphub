"use client";

import { useRef, useState, type KeyboardEvent } from "react";
import type { ViewerTab } from "@/lib/content";

const ARROW_STEP: Partial<Record<string, number>> = { ArrowRight: 1, ArrowLeft: -1 };

interface ProductViewerProps {
  tabs: readonly ViewerTab[];
}

/** The hero: real screens behind tabs, every image server-rendered so the first paint already shows the product. */
export default function ProductViewer({ tabs }: ProductViewerProps) {
  const [active, setActive] = useState(0);
  const buttons = useRef<(HTMLButtonElement | null)[]>([]);

  function onKey(event: KeyboardEvent<HTMLDivElement>) {
    const step = ARROW_STEP[event.key];
    if (step === undefined) return;
    event.preventDefault();
    const next = (active + step + tabs.length) % tabs.length;
    setActive(next);
    buttons.current[next]?.focus();
  }

  return (
    <div className="overflow-hidden rounded-xl border border-line-strong bg-surface shadow-[0_30px_80px_-40px_rgb(0_0_0/0.9)]">
      <div
        role="tablist"
        aria-label="Pantallas de ClipHub"
        onKeyDown={onKey}
        className="flex gap-1 overflow-x-auto border-b border-line bg-bg/60 px-2 py-2"
      >
        {tabs.map((tab, i) => (
          <button
            key={tab.id}
            ref={(el) => {
              buttons.current[i] = el;
            }}
            id={`tab-${tab.id}`}
            role="tab"
            type="button"
            aria-selected={i === active}
            aria-controls={`panel-${tab.id}`}
            tabIndex={i === active ? 0 : -1}
            onClick={() => setActive(i)}
            className="shrink-0 rounded-md px-3 py-1.5 font-mono text-xs uppercase tracking-wider text-faint transition-colors hover:text-fg aria-selected:bg-raised aria-selected:text-brand"
          >
            {tab.label}
          </button>
        ))}
      </div>
      {tabs.map((tab, i) => (
        <div
          key={tab.id}
          id={`panel-${tab.id}`}
          role="tabpanel"
          aria-labelledby={`tab-${tab.id}`}
          hidden={i !== active}
          className="grid lg:grid-cols-[1fr_15rem]"
        >
          <a href={tab.shot.src} className="block border-line lg:border-r">
            <img
              src={tab.shot.src}
              alt={tab.shot.alt}
              width={1600}
              height={1200}
              loading={i === 0 ? "eager" : "lazy"}
              fetchPriority={i === 0 ? "high" : "auto"}
              decoding="async"
              className="block h-auto w-full"
            />
          </a>
          <div className="flex flex-col gap-5 border-t border-line p-5 lg:border-t-0">
            <p className="text-sm leading-relaxed text-muted">{tab.shot.caption}</p>
            <dl className="grid gap-4 sm:grid-cols-3 lg:grid-cols-1">
              {tab.shot.callouts.map((c) => (
                <div key={c.label} className="flex flex-col-reverse border-l-2 border-brand/60 pl-3">
                  <dt className="mt-1 text-xs leading-snug text-faint">{c.label}</dt>
                  <dd className="font-mono text-xl font-medium text-fg lg:text-2xl">{c.value}</dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
      ))}
    </div>
  );
}
