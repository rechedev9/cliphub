"use client";

import { useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";

export function Reveal({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  // Server-rendered content starts visible: motion is an enhancement applied
  // after hydration, never a requirement for reading the page.
  const [hidden, setHidden] = useState(false);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    if (element.getBoundingClientRect().top <= window.innerHeight * 0.9) return;

    setHidden(true);
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry?.isIntersecting) return;
        setHidden(false);
        observer.disconnect();
      },
      { threshold: 0.12 },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={ref}
      className={cn(
        // Hiding is instant and happens off screen; only the entrance animates.
        hidden
          ? "translate-y-4 opacity-0"
          : "transition-[opacity,translate] duration-600 ease-entrance",
        className,
      )}
    >
      {children}
    </div>
  );
}
