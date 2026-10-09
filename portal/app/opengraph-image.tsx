import { readFile } from "node:fs/promises";
import path from "node:path";

import { ImageResponse } from "next/og";

export const alt = "ClipHub: tus mejores rondas de CS2, montadas a mano";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

// Hex twins of the oklch tokens in globals.css; the renderer has no oklch.
const CANVAS = "#080c17";
const PANEL = "#101626";
const EDGE = "#3a4460";
const TEXT = "#f2f8fc";
const MUTED = "#a9b6cc";
const CYAN = "#22d9ee";
const MAGENTA = "#ff2d78";

async function dataUri({ file, mime }: { file: string; mime: string }) {
  const bytes = await readFile(path.join(process.cwd(), "public", file));
  return `data:${mime};base64,${bytes.toString("base64")}`;
}

export default async function OpengraphImage() {
  const [frame, mark] = await Promise.all([
    dataUri({ file: "media/reel-sample-og.jpg", mime: "image/jpeg" }),
    dataUri({ file: "brand/cliphub-mark.svg", mime: "image/svg+xml" }),
  ]);

  return new ImageResponse(
    (
      <div
        style={{
          position: "relative",
          display: "flex",
          width: "100%",
          height: "100%",
          backgroundColor: CANVAS,
          backgroundImage: `radial-gradient(900px 520px at 78% 40%, rgba(34,217,238,0.2), transparent 70%)`,
          color: TEXT,
        }}
      >
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            width: 760,
            padding: "64px 0 60px 76px",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
            <img alt="" src={mark} width={56} height={56} />
            <div style={{ display: "flex", fontSize: 36, letterSpacing: "0.05em" }}>
              <span>Clip</span>
              <span style={{ color: CYAN }}>Hub</span>
            </div>
          </div>

          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 14,
              marginTop: 78,
              color: MUTED,
              fontSize: 22,
              letterSpacing: "0.2em",
            }}
          >
            <div style={{ width: 14, height: 14, borderRadius: 7, backgroundColor: MAGENTA }} />
            <span>CS2 · DE LA DEMO AL VÍDEO</span>
          </div>

          <div
            style={{
              display: "flex",
              flexDirection: "column",
              marginTop: 24,
              fontSize: 70,
              lineHeight: 1.08,
              letterSpacing: "-0.03em",
              whiteSpace: "nowrap",
            }}
          >
            <span>Tus mejores rondas,</span>
            <span style={{ color: CYAN }}>montadas a mano.</span>
          </div>

          <div style={{ display: "flex", marginTop: 30, color: MUTED, fontSize: 28, lineHeight: 1.4 }}>
            Envía la demo de tu partida y recibe un vídeo editado con tus
            jugadas. Gratis.
          </div>
        </div>

        <div
          style={{
            position: "absolute",
            top: 54,
            right: 96,
            display: "flex",
            width: 294,
            height: 522,
            padding: 8,
            borderRadius: 26,
            border: `1px solid ${EDGE}`,
            backgroundColor: PANEL,
            boxShadow: "0 40px 80px rgba(0,0,0,0.6)",
          }}
        >
          <img
            alt=""
            src={frame}
            width={276}
            height={504}
            style={{ width: 276, height: 504, borderRadius: 18, objectFit: "cover" }}
          />
        </div>
      </div>
    ),
    { ...size },
  );
}
