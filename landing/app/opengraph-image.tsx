import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import path from "node:path";

export const alt = "Un puesto de ciber con ClipHub Studio en el monitor";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

const ACCENT = "#fb923c";

export default async function OpengraphImage() {
  const scene = await readFile(path.join(process.cwd(), "public", "scene", "stills", "og.jpg"));
  const mark = await readFile(path.join(process.cwd(), "public", "brand", "cliphub-mark.svg"), "utf8");
  const sceneSource = `data:image/jpeg;base64,${scene.toString("base64")}`;
  const markSource = `data:image/svg+xml;base64,${Buffer.from(mark).toString("base64")}`;

  return new ImageResponse(
    <div style={{ position: "relative", width: "100%", height: "100%", display: "flex", backgroundColor: "#050709" }}>
      <img
        alt=""
        src={sceneSource}
        width={1200}
        height={630}
        style={{ position: "absolute", top: 0, left: 0, width: "100%", height: "100%", objectFit: "cover" }}
      />
      {/* Satori has no `inset`, so the scrim takes explicit edges. */}
      <div
        style={{
          position: "absolute",
          top: 0,
          left: 0,
          width: "100%",
          height: "100%",
          display: "flex",
          backgroundImage: "linear-gradient(0deg, rgba(5,7,9,0.97) 0%, rgba(5,7,9,0.9) 34%, rgba(5,7,9,0) 66%)",
        }}
      />
      <div
        style={{
          position: "absolute",
          left: 56,
          top: 44,
          display: "flex",
          alignItems: "center",
          gap: 14,
          padding: "10px 16px",
          borderRadius: 12,
          backgroundColor: "rgba(10,13,17,0.92)",
        }}
      >
        <img alt="" src={markSource} width={40} height={40} style={{ width: 40, height: 40 }} />
        <div style={{ display: "flex", fontSize: 28, fontWeight: 700, color: "#f2f5f8" }}>
          ClipHub<span style={{ color: "#a7b0bc", marginLeft: 8, fontWeight: 500 }}>Studio</span>
        </div>
      </div>
      <div style={{ position: "absolute", left: 56, right: 56, bottom: 48, display: "flex", flexDirection: "column" }}>
        <div
          style={{
            display: "flex",
            fontSize: 58,
            fontWeight: 700,
            lineHeight: 1.05,
            color: "#ffffff",
            letterSpacing: "-0.02em",
          }}
        >
          Tus partidas de CS2, convertidas en vídeo
        </div>
        <div style={{ display: "flex", marginTop: 16, fontSize: 26, color: "#c9d1db" }}>
          Shorts, demo completa, clips de stream y análisis.
          <span style={{ color: ACCENT, marginLeft: 10 }}>Gratis y en tu PC.</span>
        </div>
      </div>
    </div>,
    { ...size },
  );
}
