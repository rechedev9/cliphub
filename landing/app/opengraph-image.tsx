import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import path from "node:path";

export const alt = "Jugada de CS2 en primera persona grabada con el HUD de ClipHub y el titular de ClipHub Studio";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";
export const dynamic = "force-static";

const FILL = { position: "absolute", top: 0, left: 0, width: "100%", height: "100%", display: "flex" } as const;

export default async function OpengraphImage() {
  const frame = await readFile(path.join(process.cwd(), "public", "shots", "og-base.jpg"));
  const frameSource = `data:image/jpeg;base64,${frame.toString("base64")}`;

  return new ImageResponse(
    <div style={{ position: "relative", width: "100%", height: "100%", display: "flex", backgroundColor: "#000" }}>
      <img alt="" src={frameSource} width={1200} height={630} style={{ ...FILL, objectFit: "cover" }} />
      <div
        style={{
          ...FILL,
          backgroundImage: "linear-gradient(90deg, rgba(0,0,0,0.95) 0%, rgba(0,0,0,0.75) 50%, rgba(0,0,0,0.2) 100%)",
        }}
      />
      <div style={{ position: "absolute", left: 64, top: 52, display: "flex", alignItems: "center", gap: 14 }}>
        <div style={{ display: "flex", fontSize: 30, fontWeight: 800, letterSpacing: 6, color: "#fff" }}>CLIPHUB</div>
        <div style={{ display: "flex", fontSize: 18, fontWeight: 700, letterSpacing: 4, color: "#000", backgroundColor: "#22e07c", padding: "4px 10px" }}>
          STUDIO
        </div>
      </div>
      <div style={{ position: "absolute", left: 64, right: 64, bottom: 56, display: "flex", flexDirection: "column" }}>
        <div style={{ display: "flex", fontSize: 20, letterSpacing: 6, color: "#22e07c" }}>CLIPS Y SHORTS DE CS2 · HAZLO TÚ O LO HACEMOS POR TI</div>
        <div style={{ display: "flex", marginTop: 14, fontSize: 104, fontWeight: 800, lineHeight: 0.95, color: "#fff", fontStyle: "italic" }}>
          TU DEMO, HECHA VÍDEO.
        </div>
        <div style={{ display: "flex", marginTop: 22, width: 220, height: 10, backgroundColor: "#22e07c" }} />
      </div>
    </div>,
    { ...size },
  );
}
