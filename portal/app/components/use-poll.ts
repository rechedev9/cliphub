"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { fetchJson } from "./fetch-json";

export interface PollOptions<T> {
  // null disables polling, for example while a required id is missing.
  url: string | null;
  intervalMs: number;
  parse: (value: unknown) => T;
}

export interface PollState<T> {
  data: T | null;
  // The error of the latest attempt; data keeps the last good response.
  error: { code: string; detail: string | null } | null;
  // When the data on screen arrived (browser clock), so a failed refresh can say how old it is.
  loadedAt: number | null;
  loading: boolean;
  refresh: () => void;
}

// Fetches a JSON route on an interval, pauses while the tab is hidden, and refetches on return.
export function usePoll<T>(options: PollOptions<T>): PollState<T> {
  const { url, intervalMs } = options;
  const parseRef = useRef(options.parse);
  parseRef.current = options.parse;

  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<PollState<T>["error"]>(null);
  const [loadedAt, setLoadedAt] = useState<number | null>(null);
  const [loading, setLoading] = useState(url !== null);
  // Responses of superseded requests are dropped so an old one cannot overwrite a newer one.
  const sequence = useRef(0);

  const load = useCallback(async () => {
    if (url === null) return;
    const ticket = ++sequence.current;
    const result = await fetchJson(url, (value) => parseRef.current(value));
    if (ticket !== sequence.current) return;
    if (result.ok) {
      setData(result.data);
      setLoadedAt(Date.now());
      setError(null);
    } else {
      setError({ code: result.code, detail: result.detail });
    }
    setLoading(false);
  }, [url]);

  useEffect(() => {
    if (url === null) return;
    setLoading(true);
    void load();

    let timer: ReturnType<typeof setInterval> | null = null;
    const start = () => {
      if (timer === null) timer = setInterval(() => void load(), intervalMs);
    };
    const stop = () => {
      if (timer !== null) clearInterval(timer);
      timer = null;
    };
    const onVisibility = () => {
      if (document.hidden) {
        stop();
      } else {
        void load();
        start();
      }
    };
    if (!document.hidden) start();
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVisibility);
      sequence.current += 1;
    };
  }, [url, intervalMs, load]);

  const refresh = useCallback(() => void load(), [load]);
  return { data, error, loadedAt, loading, refresh };
}
