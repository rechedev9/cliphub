import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import path from "node:path";

export const alt = "Radar de Mirage con los jugadores marcados y el titular de ClipHub";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";
// The static export renders the image once at build time.
export const dynamic = "force-static";

export default async function OpengraphImage() {
  const frame = await readFile(path.join(process.cwd(), "public", "shots", "og-base.jpg"));
  const frameSource = `data:image/jpeg;base64,${frame.toString("base64")}`;

  return new ImageResponse(
    <div style={{ position: "relative", width: "100%", height: "100%", display: "flex", backgroundColor: "#000" }}>
      <img
        alt=""
        src={frameSource}
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
          backgroundImage: "linear-gradient(0deg, rgba(0,0,0,0.95) 0%, rgba(0,0,0,0.6) 45%, rgba(0,0,0,0.1) 100%)",
        }}
      />
      <div style={{ position: "absolute", left: 64, top: 52, display: "flex", fontSize: 30, fontWeight: 800, letterSpacing: 6, color: "#fff" }}>
        CLIPHUB
      </div>
      <div style={{ position: "absolute", left: 64, right: 64, bottom: 56, display: "flex", flexDirection: "column" }}>
        <div style={{ display: "flex", fontSize: 20, letterSpacing: 6, color: "#a3a3a3" }}>ANÁLISIS DE CS2 · GRATIS</div>
        <div style={{ display: "flex", marginTop: 14, fontSize: 92, fontWeight: 800, lineHeight: 0.95, color: "#fff" }}>
          LEE AL RIVAL ANTES DE QUE JUEGUE.
        </div>
      </div>
    </div>,
    { ...size },
  );
}
