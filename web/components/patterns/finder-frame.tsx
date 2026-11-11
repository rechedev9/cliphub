'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

/** The rounds a finder keeps over the rounds its scope reads: the live counter of its panel. */
export type FinderCount = { matched: number; total: number };

/** What a finder shows, cut by where each piece sits on the Buscador's canvas. */
export type FinderParts = {
  /** The finder's own controls, the body of its panel. */
  panel: ReactNode;
  /** The map with the toolbar over it. */
  map: ReactNode;
  /** The Rondas tab: histogram and the rounds list. */
  rounds: ReactNode;
  /** The Tácticas tab, for the finders that keep saved patterns. */
  tactics?: ReactNode;
  /** The bottom bar: replay of the kept rounds and the time window. */
  footer: ReactNode;
  count: FinderCount | null;
};

export type FinderSlots = {
  panel: HTMLElement | null;
  rounds: HTMLElement | null;
  tactics: HTMLElement | null;
  footer: HTMLElement | null;
};

/**
 * Where a finder puts its parts when a canvas hosts it: the canvas keeps one element per slot, so
 * its filters, panel headers and tabs stay on screen while the finder loads. The map renders where
 * the finder is mounted.
 */
export type FinderFrame = { slots: FinderSlots; onCount: (count: FinderCount | null) => void };

/** One element per slot, as state so the finder renders again once the canvas has mounted them. */
export function useFinderSlots(): { slots: FinderSlots; refs: { [K in keyof FinderSlots]: (element: HTMLElement | null) => void } } {
  const [panel, setPanel] = useState<HTMLElement | null>(null);
  const [rounds, setRounds] = useState<HTMLElement | null>(null);
  const [tactics, setTactics] = useState<HTMLElement | null>(null);
  const [footer, setFooter] = useState<HTMLElement | null>(null);
  return { slots: { panel, rounds, tactics, footer }, refs: { panel: setPanel, rounds: setRounds, tactics: setTactics, footer: setFooter } };
}

function into(slot: HTMLElement | null, content: ReactNode): ReactNode {
  return slot === null || content === undefined ? null : createPortal(content, slot);
}

/** The finder's output inside a canvas: portals into the slots, the map inline. */
export function FramedParts({ frame, parts }: { frame: FinderFrame; parts: FinderParts }): ReactNode {
  const { onCount } = frame;
  const matched = parts.count?.matched;
  const total = parts.count?.total;
  useEffect(() => {
    onCount(matched === undefined || total === undefined ? null : { matched, total });
  }, [onCount, matched, total]);
  const { slots } = frame;
  return (
    <>
      {into(slots.panel, parts.panel)}
      {into(slots.rounds, parts.rounds)}
      {into(slots.tactics, parts.tactics)}
      {into(slots.footer, parts.footer)}
      {parts.map}
    </>
  );
}
