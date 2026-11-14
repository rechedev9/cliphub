"use client";

import { useRef, useState, type KeyboardEvent } from "react";
import type { ViewerTab } from "@/lib/content";

const ARROW_STEP: Partial<Record<string, number>> = { ArrowRight: 1, ArrowLeft: -1 };

interface ProductViewerProps {
  tabs: readonly ViewerTab[];
}

/** Real screens behind tabs; every panel is server-rendered so the page works before hydration. */
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
    <div>
      <div
        role="tablist"
        aria-label="Pantallas de ClipHub"
        onKeyDown={onKey}
        className="flex gap-6 overflow-x-auto border-b border-line [scrollbar-width:none] sm:gap-9"
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
            className="-mb-px shrink-0 border-b-2 border-transparent pb-4 font-display text-sm font-bold uppercase tracking-[0.18em] text-faint transition-colors hover:text-fg aria-selected:border-fg aria-selected:text-fg"
          >
            {tab.label}
          </button>
        ))}
      </div>
      {tabs.map((tab, i) => (
        <div key={tab.id} id={`panel-${tab.id}`} role="tabpanel" aria-labelledby={`tab-${tab.id}`} hidden={i !== active}>
          <a href={tab.shot.src} className="mt-8 block border border-line">
            <img
              src={tab.shot.src}
              alt={tab.shot.alt}
              width={1600}
              height={1200}
              loading="lazy"
              decoding="async"
              className="block h-auto w-full"
            />
          </a>
          <div className="mt-8 grid gap-8 md:grid-cols-[1fr_auto] md:items-end">
            <p className="max-w-md text-base text-muted">{tab.shot.caption}</p>
            <dl className="grid grid-cols-3 gap-4 sm:gap-12">
              {tab.shot.callouts.map((c) => (
                <div key={c.label} className="flex flex-col-reverse">
                  <dt className="mt-2 font-display text-[11px] font-bold uppercase tracking-[0.18em] text-faint">
                    {c.label}
                  </dt>
                  <dd className="font-display text-2xl font-bold leading-none whitespace-nowrap sm:text-5xl">{c.value}</dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
      ))}
    </div>
  );
}
