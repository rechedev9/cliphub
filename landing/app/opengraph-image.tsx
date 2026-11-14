import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import path from "node:path";

export const alt = "ClipHub Studio: de la demo de CS2 al vídeo editado";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

const ACCENT = "#ff5d4f";

export default async function OpengraphImage() {
  const shot = await readFile(path.join(process.cwd(), "public", "media", "shots", "og.jpg"));
  const mark = await readFile(path.join(process.cwd(), "public", "brand", "cliphub-mark.svg"), "utf8");
  const shotSource = `data:image/jpeg;base64,${shot.toString("base64")}`;
  const markSource = `data:image/svg+xml;base64,${Buffer.from(mark).toString("base64")}`;

  return new ImageResponse(
    <div style={{ position: "relative", width: "100%", height: "100%", display: "flex", backgroundColor: "#0b0e0d" }}>
      <img
        alt=""
        src={shotSource}
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
          backgroundImage:
            "linear-gradient(0deg, rgba(11,14,13,0.97) 0%, rgba(11,14,13,0.9) 34%, rgba(11,14,13,0) 66%)",
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
          backgroundColor: "rgba(18,23,21,0.92)",
        }}
      >
        <img alt="" src={markSource} width={40} height={40} style={{ width: 40, height: 40 }} />
        <div style={{ display: "flex", fontSize: 28, fontWeight: 700, color: "#eef3f0" }}>
          ClipHub<span style={{ color: "#9aa8a1", marginLeft: 8, fontWeight: 500 }}>Studio</span>
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
          De la demo al vídeo, sin editar a mano
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
