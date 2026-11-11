"use client";

import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { Check, Sparkles } from "lucide-react";

const MESSAGE = "Hazme un Short con mi ACE en Mirage, con música.";
const TOOLS = [
  { name: "import_demo", detail: "5kMirage.dem" },
  { name: "list_moments", detail: "ACE en la ronda 1, 5 tiros a la cabeza" },
  { name: "list_short_options", detail: "15 temas" },
] as const;

const TYPE_START = 300;
const TYPE_END = 1500;
const ASSISTANT_AT = 2400;
const TOOL_AT = 3300;
const TOOL_GAP = 900;
const TOOL_DONE = 450;
const APPROVAL_AT = 6000;
const CLICK_AT = 6900;
const PRESS_MS = 200;
const PROGRESS_AT = 7400;
const PROGRESS_MS = 2000;
const RESULT_AT = 9600;
const HOLD_MS = 3000;
const FADE_MS = 400;
const FINAL_AT = RESULT_AT + 100;
const LOOP_END = RESULT_AT + HOLD_MS + FADE_MS;
const TICK_MS = 40;

const REVEAL_AT = [
  TYPE_START,
  TYPE_END,
  ASSISTANT_AT,
  TOOL_AT,
  TOOL_AT + TOOL_GAP,
  TOOL_AT + TOOL_GAP * 2,
  APPROVAL_AT,
  PROGRESS_AT,
  RESULT_AT,
] as const;

const EASE = "ease-[cubic-bezier(0.23,1,0.32,1)]";

function clamp01(value: number) {
  return Math.min(1, Math.max(0, value));
}

interface RevealProps {
  show: boolean;
  children: ReactNode;
}

/** Each step fades in once and stays for the rest of the cycle, so the body only grows. */
function Reveal({ show, children }: RevealProps) {
  const [entered, setEntered] = useState(show);
  const firstRender = useRef(true);

  useEffect(() => {
    if (!show) {
      setEntered(false);
      return;
    }
    if (firstRender.current) {
      firstRender.current = false;
      return;
    }
    const frame = window.requestAnimationFrame(() => setEntered(true));
    return () => window.cancelAnimationFrame(frame);
  }, [show]);

  if (!show) return null;

  return (
    <div
      className={`transition-[opacity,transform] duration-300 ${EASE} motion-reduce:transition-none`}
      style={{ opacity: entered ? 1 : 0, transform: entered ? "none" : "translateY(4px)" }}
    >
      {children}
    </div>
  );
}

export default function AssistantChat() {
  const descriptionId = useId();
  const rootRef = useRef<HTMLElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);
  const firstScroll = useRef(true);
  const [elapsed, setElapsed] = useState(FINAL_AT);
  const [motionOn, setMotionOn] = useState(false);
  const [onScreen, setOnScreen] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const paused = hovered || focused;

  useEffect(() => {
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    if (media.matches) return;
    setMotionOn(true);
    const root = rootRef.current;
    if (!root) return;
    const observer = new IntersectionObserver((entries) => {
      setOnScreen(entries[0]?.isIntersecting ?? false);
    });
    observer.observe(root);
    const onChange = () => setMotionOn(!media.matches);
    media.addEventListener("change", onChange);
    return () => {
      observer.disconnect();
      media.removeEventListener("change", onChange);
    };
  }, []);

  useEffect(() => {
    if (!motionOn) setElapsed(FINAL_AT);
  }, [motionOn]);

  useEffect(() => {
    if (!motionOn || !onScreen || paused) return;
    const id = window.setInterval(() => {
      setElapsed((current) => {
        const next = current + TICK_MS;
        return next >= LOOP_END ? 0 : next;
      });
    }, TICK_MS);
    return () => window.clearInterval(id);
  }, [motionOn, onScreen, paused]);

  const typing = clamp01((elapsed - TYPE_START) / (TYPE_END - TYPE_START));
  const typedMessage = MESSAGE.slice(0, Math.round(MESSAGE.length * typing));
  const isTyping = elapsed >= TYPE_START && elapsed < TYPE_END;
  const approved = elapsed >= CLICK_AT + PRESS_MS;
  const pressing = elapsed >= CLICK_AT && !approved;
  const progress = elapsed >= PROGRESS_AT ? clamp01((elapsed - PROGRESS_AT) / PROGRESS_MS) : 0;
  const percent = Math.round(progress * 100);
  const editing = progress >= 0.5;
  const fading = elapsed >= RESULT_AT + HOLD_MS;
  const sequence = REVEAL_AT.filter((at) => elapsed >= at).length;

  useEffect(() => {
    if (firstScroll.current) {
      firstScroll.current = false;
      return;
    }
    const body = bodyRef.current;
    if (body) body.scrollTop = body.scrollHeight;
  }, [sequence]);

  return (
    <figure
      ref={rootRef}
      aria-label="Ejemplo: un asistente crea un Short con ClipHub Studio"
      aria-describedby={descriptionId}
      tabIndex={0}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      className="mx-auto w-full max-w-[34rem] rounded-lg outline-none focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
    >
      <p id={descriptionId} className="sr-only">
        Una persona pide en español un Short de su ACE en Mirage con música; el asistente busca
        la jugada en su demo, propone grabar tres jugadas, la persona aprueba, Studio graba en CS2,
        edita y deja el Short de 0:27 listo en Mis vídeos.
      </p>

      <div
        aria-hidden="true"
        className={`overflow-hidden rounded-lg border border-line bg-bg shadow-2xl transition-opacity duration-300 ${EASE} motion-reduce:transition-none`}
        style={{ opacity: fading ? 0 : 1 }}
      >
        <div className="flex h-[38rem] flex-col">
          <header className="flex items-center gap-2.5 border-b border-line bg-surface px-3.5 py-2.5">
            <span className="grid size-7 shrink-0 place-items-center rounded-md border border-line bg-surface-2 text-primary">
              <Sparkles size={14} aria-hidden="true" />
            </span>
            <div className="min-w-0">
              <p className="font-display text-sm font-semibold leading-tight text-fg">
                Tu asistente
              </p>
              <p className="mt-0.5 flex items-center gap-1.5 text-[11px] leading-tight text-muted">
                <span className="size-1.5 shrink-0 rounded-full bg-primary" />
                ClipHub Studio conectado
              </p>
            </div>
          </header>

          <div
            ref={bodyRef}
            className="min-h-0 flex-1 overflow-y-auto [scrollbar-width:thin]"
          >
            <div className="flex flex-col gap-2.5 px-3 py-3">
              <Reveal show={elapsed >= TYPE_START}>
                <div className="ml-auto max-w-[85%] rounded-lg rounded-br-sm border border-line bg-surface-2 px-3 py-2">
                  <p className="min-h-[1.25rem] text-[13px] leading-relaxed text-fg">
                    {typedMessage}
                    {isTyping ? (
                      <span className="ml-0.5 inline-block h-3.5 w-[2px] translate-y-[2px] bg-primary" />
                    ) : null}
                  </p>
                </div>
              </Reveal>

              <Reveal show={elapsed >= ASSISTANT_AT}>
                <p className="max-w-[95%] text-[13px] leading-relaxed text-fg">
                  Busco la jugada en tu demo.
                </p>
              </Reveal>

              <div className="space-y-1.5 empty:hidden">
                {TOOLS.map((tool, index) => {
                  const done = elapsed >= TOOL_AT + TOOL_GAP * index + TOOL_DONE;
                  return (
                    <Reveal key={tool.name} show={elapsed >= TOOL_AT + TOOL_GAP * index}>
                      <div className="flex items-start gap-2 rounded-sm border border-line bg-surface px-2.5 py-1.5">
                        <span className="relative mt-[3px] size-3 shrink-0">
                          <span
                            className={`absolute inset-0 grid place-items-center transition-opacity duration-200 ${EASE}`}
                            style={{ opacity: done ? 0 : 1 }}
                          >
                            <span className="size-1.5 rounded-full bg-muted opacity-50" />
                          </span>
                          <Check
                            size={12}
                            strokeWidth={2.5}
                            aria-hidden="true"
                            className={`absolute inset-0 text-primary transition-opacity duration-200 ${EASE}`}
                            style={{ opacity: done ? 1 : 0 }}
                          />
                        </span>
                        <p className="min-w-0 font-mono text-[11px] leading-snug">
                          <span className="text-fg">{tool.name}</span>
                          <span className="text-muted"> · {tool.detail}</span>
                        </p>
                      </div>
                    </Reveal>
                  );
                })}
              </div>

              <Reveal show={elapsed >= APPROVAL_AT}>
                <div className="rounded-md border border-line bg-surface p-3">
                  <p className="text-[13px] font-semibold leading-snug text-fg">
                    Grabar un Short con 3 jugadas
                  </p>
                  <p className="mt-1 text-[11px] leading-snug text-muted">
                    CS2 se abrirá y grabará unos minutos.
                  </p>
                  <div className="mt-3 flex items-center gap-2">
                    <span
                      aria-hidden="true"
                      className={`inline-flex h-7 w-24 items-center justify-center gap-1 rounded text-[11px] font-semibold transition-transform duration-200 ${EASE} ${
                        approved
                          ? "border border-primary/40 bg-primary/10 text-primary"
                          : "bg-accent text-accent-fg"
                      }`}
                      style={{ transform: pressing ? "scale(0.97)" : "none" }}
                    >
                      {approved ? (
                        <>
                          <Check size={11} strokeWidth={2.5} aria-hidden="true" />
                          Aprobado
                        </>
                      ) : (
                        "Aprobar"
                      )}
                    </span>
                    <span
                      aria-hidden="true"
                      className="inline-flex h-7 w-20 items-center justify-center rounded border border-line text-[11px] font-medium text-muted"
                    >
                      Cancelar
                    </span>
                  </div>
                </div>
              </Reveal>

              <Reveal show={elapsed >= PROGRESS_AT}>
                <div className="rounded-md border border-line bg-surface px-3 py-2.5">
                  <div className="flex items-center justify-between gap-3">
                    <span className="flex min-w-0 items-center gap-1.5 text-[11px] font-medium text-fg">
                      <span
                        className={`size-1.5 shrink-0 rounded-full ${editing ? "bg-primary" : "bg-accent"}`}
                      />
                      <span className="truncate">{editing ? "Editando" : "Grabando en CS2"}</span>
                    </span>
                    <span className="shrink-0 font-mono text-[10px] tabular-nums text-muted">
                      {percent}%
                    </span>
                  </div>
                  <div className="mt-2 h-1 overflow-hidden rounded-full bg-surface-2">
                    <span
                      className="block h-full origin-left rounded-full bg-primary"
                      style={{ transform: `scaleX(${progress})` }}
                    />
                  </div>
                </div>
              </Reveal>

              <Reveal show={elapsed >= RESULT_AT}>
                <div className="flex items-center gap-3 rounded-md border border-line bg-surface p-3">
                  <img
                    src="/media/video/short.webp"
                    alt="Fotograma del Short del ACE en Mirage, en formato 9:16"
                    width={80}
                    height={142}
                    loading="lazy"
                    decoding="async"
                    className="h-[142px] w-20 shrink-0 rounded-sm border border-line object-cover"
                  />
                  <div className="min-w-0">
                    <p className="flex items-center gap-1.5 text-[13px] font-semibold text-fg">
                      <Check
                        size={13}
                        strokeWidth={2.5}
                        aria-hidden="true"
                        className="shrink-0 text-primary"
                      />
                      Listo en Mis vídeos
                    </p>
                    <p className="mt-1 font-mono text-[11px] text-muted">0:27 · 9:16 · ACE</p>
                    <span className="mt-2 inline-block text-[11px] font-medium text-blue underline decoration-blue/40 underline-offset-2">
                      Abrir en Studio
                    </span>
                  </div>
                </div>
              </Reveal>
            </div>
          </div>
        </div>
      </div>
    </figure>
  );
}
