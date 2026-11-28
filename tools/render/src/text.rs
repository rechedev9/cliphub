use crate::schema::{FontConfig, FontRole, TextAlign};
use anyhow::{Context, Result};
use skrifa::instance::{LocationRef, Size};
use skrifa::{FontRef, MetadataProvider};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use vello::kurbo::Affine;
use vello::peniko::{Blob, Color, Fill, FontData};
use vello::{Glyph, Scene};

const EMBEDDED_DISPLAY_FONT: &[u8] = include_bytes!("../assets/fonts/Saira-ExtraBold.ttf");
const EMBEDDED_MONO_FONT: &[u8] = include_bytes!("../assets/fonts/IBMPlexMono-SemiBold.ttf");

pub struct LoadedFont {
    pub font_data: FontData,
    pub _path: PathBuf,
}

pub struct FontRegistry {
    pub display: LoadedFont,
    pub mono: LoadedFont,
    pub sans: LoadedFont,
}

impl FontRegistry {
    pub fn load(cfg: &FontConfig, base_dir: &Path) -> Result<Self> {
        let local_fonts = std::env::var("LOCALAPPDATA")
            .ok()
            .map(|d| PathBuf::from(d).join("Microsoft").join("Windows").join("Fonts"));
        let local_saira = local_fonts.as_ref().map(|d| d.join("Saira-ExtraBold.ttf"));
        let local_jb = local_fonts
            .as_ref()
            .map(|d| d.join("JetBrainsMonoNerdFont-Bold.ttf"));

        let display_candidates = [
            cfg.display.as_deref(),
            local_saira.as_deref().and_then(|p| p.to_str()),
            Some(r"C:\Windows\Fonts\impact.ttf"),
            Some(r"C:\Windows\Fonts\segoeuib.ttf"),
            Some(r"C:\Windows\Fonts\arialbd.ttf"),
        ];
        let mono_candidates = [
            cfg.mono.as_deref(),
            Some(r"C:\Windows\Fonts\JetBrainsMonoNerdFont-Bold.ttf"),
            local_jb.as_deref().and_then(|p| p.to_str()),
            Some(r"C:\Windows\Fonts\consolab.ttf"),
            Some(r"C:\Windows\Fonts\arialbd.ttf"),
        ];
        let sans_candidates = [
            cfg.sans.as_deref(),
            Some(r"C:\Windows\Fonts\segoeuib.ttf"),
            Some(r"C:\Windows\Fonts\arialbd.ttf"),
            Some(r"C:\Windows\Fonts\impact.ttf"),
        ];

        Ok(Self {
            display: load_first_existing_or_embedded(
                &display_candidates,
                base_dir,
                "display",
                EMBEDDED_DISPLAY_FONT,
                "<embedded:Saira-ExtraBold.ttf>",
            )?,
            mono: load_first_existing_or_embedded(
                &mono_candidates,
                base_dir,
                "mono",
                EMBEDDED_MONO_FONT,
                "<embedded:IBMPlexMono-SemiBold.ttf>",
            )?,
            sans: load_first_existing_or_embedded(
                &sans_candidates,
                base_dir,
                "sans",
                EMBEDDED_DISPLAY_FONT,
                "<embedded:Saira-ExtraBold.ttf>",
            )?,
        })
    }

    #[cfg(test)]
    pub fn load_embedded() -> Result<Self> {
        Ok(Self {
            display: load_embedded_font(
                EMBEDDED_DISPLAY_FONT,
                "<embedded:Saira-ExtraBold.ttf>",
                "display",
            )?,
            mono: load_embedded_font(
                EMBEDDED_MONO_FONT,
                "<embedded:IBMPlexMono-SemiBold.ttf>",
                "mono",
            )?,
            sans: load_embedded_font(
                EMBEDDED_DISPLAY_FONT,
                "<embedded:Saira-ExtraBold.ttf>",
                "sans",
            )?,
        })
    }

    pub fn get(&self, role: FontRole) -> &LoadedFont {
        match role {
            FontRole::Display => &self.display,
            FontRole::Mono => &self.mono,
            FontRole::Sans => &self.sans,
        }
    }
}

fn load_embedded_font(bytes: &'static [u8], label: &str, role: &str) -> Result<LoadedFont> {
    let font_data = FontData::new(Blob::new(Arc::new(bytes)), 0);
    FontRef::from_index(font_data.data.as_ref(), font_data.index)
        .with_context(|| format!("invalid embedded {role} font {label}"))?;
    Ok(LoadedFont {
        font_data,
        _path: PathBuf::from(label),
    })
}

fn load_first_existing_or_embedded(
    candidates: &[Option<&str>],
    base_dir: &Path,
    role: &str,
    embedded_bytes: &'static [u8],
    embedded_label: &str,
) -> Result<LoadedFont> {
    for cand in candidates.iter().flatten() {
        let p = PathBuf::from(cand);
        let resolved = if p.is_absolute() { p } else { base_dir.join(p) };
        if resolved.exists()
            && let Ok(bytes) = std::fs::read(&resolved)
        {
            let font_data = FontData::new(Blob::new(Arc::new(bytes)), 0);
            if FontRef::from_index(font_data.data.as_ref(), font_data.index).is_ok() {
                return Ok(LoadedFont {
                    font_data,
                    _path: resolved,
                });
            }
        }
    }
    load_embedded_font(embedded_bytes, embedded_label, role)
}

#[derive(Debug, Clone)]
pub struct TextMetrics {
    #[allow(dead_code)]
    pub width: f32,
    pub height: f32,
    pub ascent: f32,
    pub line_height: f32,
    pub lines: Vec<String>,
}

fn map_char_fallback(font: &FontRef<'_>, ch: char) -> Option<skrifa::GlyphId> {
    let charmap = font.charmap();
    if let Some(gid) = charmap.map(ch) {
        return Some(gid);
    }
    let alt = match ch {
        '✖' | '✕' | '×' => 'X',
        '−' | '–' | '—' => '-',
        '→' | '➔' => '>',
        '·' => '.',
        '«' | '»' | '“' | '”' => '"',
        '’' | '‘' => '\'',
        _ => return None,
    };
    charmap.map(alt)
}

pub fn measure_line_width(font_data: &FontData, text: &str, font_size: f32, letter_spacing: f32) -> f32 {
    let Ok(font) = FontRef::from_index(font_data.data.as_ref(), font_data.index) else {
        return text.chars().count() as f32 * font_size * 0.6;
    };
    let glyph_metrics = font.glyph_metrics(Size::new(font_size), LocationRef::default());
    let mut pen_x = 0.0_f32;
    let mut count = 0usize;
    for ch in text.chars() {
        if let Some(gid) = map_char_fallback(&font, ch) {
            pen_x += glyph_metrics.advance_width(gid).unwrap_or(font_size * 0.5);
            count += 1;
        }
    }
    if count > 1 {
        pen_x += (count - 1) as f32 * letter_spacing;
    }
    pen_x
}

pub fn layout_text(
    font_data: &FontData,
    text: &str,
    font_size: f32,
    max_width: f32,
    letter_spacing: f32,
    line_height_mult: f32,
) -> TextMetrics {
    let ascent = FontRef::from_index(font_data.data.as_ref(), font_data.index)
        .ok()
        .map(|f| f.metrics(Size::new(font_size), LocationRef::default()).ascent)
        .unwrap_or(font_size * 0.8);

    let line_height = font_size * line_height_mult;
    let mut lines = Vec::new();
    let mut max_line_w = 0.0_f32;

    for raw_para in text.split('\n') {
        let words: Vec<&str> = raw_para.split_whitespace().collect();
        if words.is_empty() {
            lines.push(String::new());
            continue;
        }
        let mut current = String::new();
        for w in words {
            if current.is_empty() {
                current.push_str(w);
            } else {
                let candidate = format!("{current} {w}");
                let cw = measure_line_width(font_data, &candidate, font_size, letter_spacing);
                if max_width > 0.0 && cw > max_width {
                    let lw = measure_line_width(font_data, &current, font_size, letter_spacing);
                    max_line_w = max_line_w.max(lw);
                    lines.push(current);
                    current = w.to_string();
                } else {
                    current = candidate;
                }
            }
        }
        if !current.is_empty() {
            let lw = measure_line_width(font_data, &current, font_size, letter_spacing);
            max_line_w = max_line_w.max(lw);
            lines.push(current);
        }
    }

    let height = (lines.len().max(1) as f32) * line_height;
    TextMetrics {
        width: max_line_w,
        height,
        ascent,
        line_height,
        lines,
    }
}

#[allow(clippy::too_many_arguments)]
pub fn draw_text_block(
    scene: &mut Scene,
    font_data: &FontData,
    text: &str,
    font_size: f32,
    x: f32,
    y: f32,
    box_width: f32,
    align: TextAlign,
    color: Color,
    transform: Affine,
    letter_spacing: f32,
    line_height_mult: f32,
) -> TextMetrics {
    let metrics = layout_text(
        font_data,
        text,
        font_size,
        box_width,
        letter_spacing,
        line_height_mult,
    );
    let Ok(font) = FontRef::from_index(font_data.data.as_ref(), font_data.index) else {
        return metrics;
    };
    let glyph_metrics = font.glyph_metrics(Size::new(font_size), LocationRef::default());
    let mut glyphs = Vec::with_capacity(text.len());

    for (line_idx, line) in metrics.lines.iter().enumerate() {
        let line_w = measure_line_width(font_data, line, font_size, letter_spacing);
        let start_x = match align {
            TextAlign::Left => x,
            TextAlign::Center => x + ((box_width - line_w) * 0.5).max(0.0),
            TextAlign::Right => x + (box_width - line_w).max(0.0),
        };
        let baseline_y = y + metrics.ascent + (line_idx as f32) * metrics.line_height;
        let mut pen_x = start_x;
        for ch in line.chars() {
            if let Some(gid) = map_char_fallback(&font, ch) {
                glyphs.push(Glyph {
                    id: gid.to_u32(),
                    x: pen_x,
                    y: baseline_y,
                });
                pen_x += glyph_metrics.advance_width(gid).unwrap_or(font_size * 0.5) + letter_spacing;
            }
        }
    }

    if !glyphs.is_empty() {
        scene
            .draw_glyphs(font_data)
            .font_size(font_size)
            .transform(transform)
            .brush(color)
            .draw(Fill::NonZero, glyphs.into_iter());
    }

    metrics
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn loads_system_fonts_and_wraps() {
        let reg = FontRegistry::load(&FontConfig::default(), Path::new(".")).unwrap();
        let m = layout_text(
            &reg.display.font_data,
            "HUASOPEEK MUERE PRIMERO GANA MAS",
            60.0,
            400.0,
            0.0,
            1.05,
        );
        assert!(m.lines.len() >= 2, "expected wrapping, got {:?}", m.lines);
        assert!(m.width > 50.0);
    }

    #[test]
    fn falls_back_to_embedded_fonts_when_candidates_missing() {
        let missing = [Some("/definitely/missing/font_a.ttf"), Some("missing_b.ttf")];
        let display = load_first_existing_or_embedded(
            &missing,
            Path::new("."),
            "display",
            EMBEDDED_DISPLAY_FONT,
            "<embedded:Saira-ExtraBold.ttf>",
        )
        .unwrap();
        assert_eq!(display._path, PathBuf::from("<embedded:Saira-ExtraBold.ttf>"));

        let mono = load_first_existing_or_embedded(
            &missing,
            Path::new("."),
            "mono",
            EMBEDDED_MONO_FONT,
            "<embedded:IBMPlexMono-SemiBold.ttf>",
        )
        .unwrap();
        assert_eq!(mono._path, PathBuf::from("<embedded:IBMPlexMono-SemiBold.ttf>"));

        let embedded_reg = FontRegistry::load_embedded().unwrap();
        for role in [FontRole::Display, FontRole::Mono, FontRole::Sans] {
            let font_data = &embedded_reg.get(role).font_data;
            let metrics = layout_text(
                font_data,
                "MURIÓ EL PRIMERO DE LA RONDA · 9Z",
                48.0,
                360.0,
                0.0,
                1.1,
            );
            assert!(metrics.lines.len() >= 2, "expected wrap for {role:?}");
            assert!(metrics.width > 50.0, "expected positive width for {role:?}");
        }
    }
}
