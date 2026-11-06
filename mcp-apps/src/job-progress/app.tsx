import { useEffect, useRef, useState } from "react";

import type { Bridge, Theme, ToolResult } from "../bridge.ts";
import type { Json } from "../json.ts";
import { type Progress, describeVideos, followResult, formatElapsed, keepCalls, readProgress, videoNames } from "./progress.ts";

const RETRY_MS = 10_000;

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

// useOnScreen pauses polling for views scrolled out of the chat: each wait_for_job call
// in a conversation leaves a view behind, and only the ones in sight need to stay live.
function useOnScreen(node: HTMLElement | null): boolean {
  const [onScreen, setOnScreen] = useState(true);
  useEffect(() => {
    if (!node) return;
    // A view the host has not sized yet has no height and must not count as scrolled away.
    const observer = new IntersectionObserver((entries) => setOnScreen(entries.some((e) => e.isIntersecting || e.boundingClientRect.height === 0)));
    observer.observe(node);
    return () => observer.disconnect();
  }, [node]);
  return onScreen;
}

export function App({ bridge }: { bridge: Bridge }) {
  const [progress, setProgress] = useState<Progress | null>(null);
  const [videos, setVideos] = useState("");
  const [retrying, setRetrying] = useState("");
  const [stopped, setStopped] = useState(false);
  const [theme, setTheme] = useState<Theme>("");
  const [root, setRoot] = useState<HTMLElement | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const input = useRef<Json>({});
  const startedAt = useRef(Date.now());
  const onScreen = useOnScreen(root);

  useEffect(() => {
    bridge.on("input", (args) => {
      input.current = args;
    });
    bridge.on("result", (result: ToolResult) => setProgress((prev) => keepCalls(prev, readProgress(result, input.current))));
    bridge.on("cancelled", () => setStopped(true));
    bridge.on("teardown", () => setStopped(true));
    bridge.on("theme", setTheme);
    bridge.replay();
    bridge.start("cliphub-job-progress").then(setTheme, () => setStopped(true));
  }, [bridge]);

  useEffect(() => {
    if (theme) document.documentElement.dataset.theme = theme;
  }, [theme]);

  useEffect(() => bridge.observeSize(document.body), [bridge]);

  const follow = progress?.follow ?? null;
  useEffect(() => {
    if (!follow || stopped || !onScreen || !bridge.canCallTools()) return;
    let cancelled = false;
    void (async () => {
      while (!cancelled) {
        try {
          const result = await bridge.callTool(follow.tool, follow.args);
          if (cancelled) return;
          // A failed call is Studio being closed or busy, not the job failing: keep the last state.
          if (result.isError) throw new Error(result.text);
          setRetrying("");
          const next = followResult(follow, result);
          if (next) setProgress((prev) => keepCalls(prev, next));
          if (!next?.follow) return;
        } catch (_err) {
          if (cancelled) return;
          setRetrying("No hay respuesta de Studio; se reintenta en unos segundos.");
          await sleep(RETRY_MS);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bridge, follow, stopped, onScreen]);

  const running = progress !== null && progress.phase !== "done" && progress.phase !== "failed";
  // A view that opens on a finished job has no time of its own to show.
  const [sawRunning, setSawRunning] = useState(false);
  if (running && !sawRunning) setSawRunning(true);
  useEffect(() => {
    if (!running) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [running]);

  const videosCall = progress?.videos ?? null;
  const variant = progress?.variant ?? "";
  useEffect(() => {
    if (!videosCall || !bridge.canCallTools()) return;
    let cancelled = false;
    bridge.callTool(videosCall.tool, videosCall.args).then(
      (result) => {
        if (!cancelled && !result.isError) setVideos(describeVideos(videoNames(result, variant)));
      },
      () => undefined,
    );
    return () => {
      cancelled = true;
    };
  }, [bridge, videosCall, variant]);

  if (!progress) return <div ref={setRoot} />;
  const percent = progress.percent;
  const elapsed = formatElapsed(now - startedAt.current);
  return (
    <main ref={setRoot} className="card" data-phase={progress.phase}>
      <div className="head">
        <strong className="title">{progress.title}</strong>
        {percent !== null && progress.phase !== "done" ? <span className="percent">{percent}%</span> : null}
        {sawRunning ? (
          <span className="elapsed" title="Tiempo desde que se abrió esta vista">
            {elapsed}
          </span>
        ) : null}
      </div>
      <div
        className="track"
        role="progressbar"
        aria-label={progress.title}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent ?? undefined}
        data-indeterminate={percent === null && running ? "" : undefined}
      >
        <div className="fill" style={{ width: `${percent ?? (running ? 30 : 100)}%` }} />
      </div>
      {progress.detail ? <p className="detail">{progress.detail}</p> : null}
      {videos ? <p className="video">{videos}</p> : null}
      {retrying && running ? <p className="note">{retrying}</p> : null}
      {running && !bridge.canCallTools() ? <p className="note">Esta vista no se actualiza sola; el asistente sigue el progreso.</p> : null}
    </main>
  );
}
