use anyhow::{Context, Result, bail};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct ProjectSpec {
    #[serde(default = "default_width")]
    pub width: u32,
    #[serde(default = "default_height")]
    pub height: u32,
    #[serde(default = "default_fps")]
    pub fps: u32,
    pub duration: f64,
    #[serde(default)]
    pub base_dir: Option<String>,
    #[serde(default = "default_bg")]
    pub background: String,
    #[serde(default = "default_true")]
    pub desk_pattern: bool,
    #[serde(default)]
    pub bg_mood: Option<String>,
    #[serde(default = "default_vignette")]
    pub vignette: f32,
    #[serde(default)]
    pub normalize_lufs: Option<f64>,
    #[serde(default)]
    pub encoder: EncoderConfig,
    #[serde(default)]
    pub fonts: FontConfig,
    #[serde(default)]
    pub camera: Vec<CameraTween>,
    #[serde(default)]
    pub camera_shakes: Vec<CameraShake>,
    #[serde(default)]
    pub layers: Vec<LayerSpec>,
    #[serde(default)]
    pub audio: Vec<AudioClipSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct EssayScene {
    pub id: String,
    pub duration: f64,
    #[serde(default = "default_bg")]
    pub background: String,
    #[serde(default = "default_true")]
    pub desk_pattern: bool,
    #[serde(default)]
    pub bg_mood: Option<String>,
    #[serde(default = "default_vignette")]
    pub vignette: f32,
    #[serde(default)]
    pub camera: Vec<CameraTween>,
    #[serde(default)]
    pub camera_shakes: Vec<CameraShake>,
    #[serde(default)]
    pub layers: Vec<LayerSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct EssaySpec {
    #[serde(default = "default_width")]
    pub width: u32,
    #[serde(default = "default_height")]
    pub height: u32,
    #[serde(default = "default_fps")]
    pub fps: u32,
    #[serde(default)]
    pub base_dir: Option<String>,
    #[serde(default = "default_lufs")]
    pub normalize_lufs: Option<f64>,
    #[serde(default)]
    pub encoder: EncoderConfig,
    #[serde(default)]
    pub fonts: FontConfig,
    pub scenes: Vec<EssayScene>,
    #[serde(default)]
    pub audio: Vec<AudioClipSpec>,
}

fn default_lufs() -> Option<f64> {
    Some(-14.0)
}

pub enum SpecDocument {
    Project(ProjectSpec),
    Essay(EssaySpec),
}

impl SpecDocument {
    pub fn load(path: &Path) -> Result<Self> {
        let text = std::fs::read_to_string(path)
            .with_context(|| format!("failed reading spec {}", path.display()))?;
        let val: serde_json::Value = serde_json::from_str(&text)
            .with_context(|| format!("failed parsing JSON in {}", path.display()))?;
        if val.get("scenes").is_some() {
            let essay: EssaySpec = serde_json::from_value(val)
                .with_context(|| format!("failed parsing EssaySpec in {}", path.display()))?;
            Ok(Self::Essay(essay))
        } else {
            let proj: ProjectSpec = serde_json::from_value(val)
                .with_context(|| format!("failed parsing ProjectSpec in {}", path.display()))?;
            Ok(Self::Project(proj))
        }
    }
}

pub fn export_project_schema() -> Result<String> {
    let schema = schemars::schema_for!(ProjectSpec);
    Ok(serde_json::to_string_pretty(&schema)?)
}

pub fn export_essay_schema() -> Result<String> {
    let schema = schemars::schema_for!(EssaySpec);
    Ok(serde_json::to_string_pretty(&schema)?)
}

fn default_width() -> u32 {
    1920
}
fn default_height() -> u32 {
    1080
}
fn default_fps() -> u32 {
    60
}
fn default_bg() -> String {
    "#07090E".to_string()
}
fn default_true() -> bool {
    true
}
fn default_vignette() -> f32 {
    0.45
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct EncoderConfig {
    #[serde(default = "default_codec")]
    pub codec: String,
    #[serde(default = "default_preset")]
    pub preset: String,
    #[serde(default = "default_cq")]
    pub cq: u32,
}

impl Default for EncoderConfig {
    fn default() -> Self {
        Self {
            codec: default_codec(),
            preset: default_preset(),
            cq: default_cq(),
        }
    }
}

fn default_codec() -> String {
    "auto".to_string()
}
fn default_preset() -> String {
    "p4".to_string()
}
fn default_cq() -> u32 {
    19
}

#[derive(Debug, Clone, Default, Serialize, Deserialize, JsonSchema)]
pub struct FontConfig {
    #[serde(default)]
    pub display: Option<String>,
    #[serde(default)]
    pub mono: Option<String>,
    #[serde(default)]
    pub sans: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, JsonSchema, PartialEq, Eq, Default)]
#[serde(rename_all = "snake_case")]
pub enum FontRole {
    #[default]
    Display,
    Mono,
    Sans,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, JsonSchema, PartialEq, Eq, Default)]
#[serde(rename_all = "snake_case")]
pub enum TextAlign {
    #[default]
    Left,
    Center,
    Right,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, JsonSchema, PartialEq, Eq, Default)]
#[serde(rename_all = "snake_case")]
pub enum EaseKind {
    Linear,
    Power1InOut,
    Power2In,
    Power2Out,
    Power3In,
    #[default]
    Power3Out,
    Power3InOut,
    Power4In,
    Power4Out,
    BackOut,
    ElasticOut,
    SineInOut,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, JsonSchema, PartialEq, Eq, Default)]
#[serde(rename_all = "snake_case")]
pub enum EnterKind {
    None,
    #[default]
    Fade,
    Up,
    Down,
    Left,
    Right,
    Pop,
    Slam,
    Zoom,
    Stamp,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, JsonSchema, PartialEq, Eq, Default)]
#[serde(rename_all = "snake_case")]
pub enum ExitKind {
    #[default]
    Fade,
    Up,
    None,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct CameraTween {
    pub start: f64,
    pub duration: f64,
    #[serde(default = "default_ease_linear")]
    pub ease: EaseKind,
    #[serde(default)]
    pub from_x: Option<f32>,
    #[serde(default)]
    pub to_x: Option<f32>,
    #[serde(default)]
    pub from_y: Option<f32>,
    #[serde(default)]
    pub to_y: Option<f32>,
    #[serde(default)]
    pub from_scale: Option<f32>,
    #[serde(default)]
    pub to_scale: Option<f32>,
}

fn default_ease_linear() -> EaseKind {
    EaseKind::Linear
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct CameraShake {
    pub at: f64,
    #[serde(default = "default_shake_dur")]
    pub duration: f64,
    #[serde(default = "default_shake_x")]
    pub amplitude_x: f32,
    #[serde(default = "default_shake_y")]
    pub amplitude_y: f32,
}

fn default_shake_dur() -> f64 {
    0.22
}
fn default_shake_x() -> f32 {
    -7.0
}
fn default_shake_y() -> f32 {
    4.0
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct PulseSpec {
    pub start: f64,
    #[serde(default = "default_pulse_scale")]
    pub scale: f32,
    #[serde(default = "default_pulse_dur")]
    pub half_period: f64,
    #[serde(default = "default_pulse_repeats")]
    pub repeats: u32,
}

fn default_pulse_scale() -> f32 {
    1.18
}
fn default_pulse_dur() -> f64 {
    0.45
}
fn default_pulse_repeats() -> u32 {
    3
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct LayerSpec {
    pub id: String,
    pub start: f64,
    #[serde(default)]
    pub end: Option<f64>,
    #[serde(default)]
    pub enter: EnterKind,
    #[serde(default)]
    pub enter_duration: Option<f64>,
    #[serde(default)]
    pub exit: ExitKind,
    #[serde(default)]
    pub exit_duration: Option<f64>,
    #[serde(default)]
    pub rotation_deg: f32,
    #[serde(default)]
    pub pulse: Option<PulseSpec>,
    #[serde(default)]
    pub fixed: bool,
    #[serde(flatten)]
    pub kind: LayerKind,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct TagSpec {
    pub text: String,
    pub bg: String,
    pub fg: String,
    #[serde(default = "default_tag_size")]
    pub font_size: f32,
}

fn default_tag_size() -> f32 {
    28.0
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct VideoSegmentSpec {
    pub src: String,
    pub start: f64,
    pub duration: f64,
    #[serde(default)]
    pub media_start: f64,
    #[serde(default = "default_rate")]
    pub playback_rate: f64,
}

fn default_rate() -> f64 {
    1.0
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct ValueKeyframe {
    pub at: f64,
    pub text: String,
    #[serde(default)]
    pub color: Option<String>,
    #[serde(default = "default_jump_scale")]
    pub jump_scale: f32,
}

fn default_jump_scale() -> f32 {
    1.6
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct CounterSpec {
    pub start: f64,
    pub duration: f64,
    pub from: f64,
    pub to: f64,
    #[serde(default)]
    pub decimals: usize,
    #[serde(default)]
    pub suffix: String,
    #[serde(default = "default_counter_ease")]
    pub ease: EaseKind,
}

fn default_counter_ease() -> EaseKind {
    EaseKind::Power1InOut
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct ColorKeyframe {
    pub at: f64,
    pub fill: String,
    #[serde(default)]
    pub border_color: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct CaptionWord {
    pub start: f64,
    pub end: f64,
    pub text: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct CaptionItem {
    pub start: f64,
    pub end: f64,
    pub text: String,
    #[serde(default)]
    pub words: Vec<CaptionWord>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum LayerKind {
    VideoBox {
        x: f32,
        y: f32,
        width: f32,
        height: f32,
        #[serde(default = "default_box_radius")]
        radius: f32,
        #[serde(default = "default_border_width")]
        border_width: f32,
        #[serde(default = "default_border_color")]
        border_color: String,
        #[serde(default = "default_true")]
        shadow: bool,
        #[serde(default = "default_one_f32")]
        zoom: f32,
        #[serde(default = "default_one_f32")]
        contrast: f32,
        #[serde(default)]
        tag: Option<TagSpec>,
        segments: Vec<VideoSegmentSpec>,
    },
    ImageBox {
        src: String,
        x: f32,
        y: f32,
        width: f32,
        height: f32,
        #[serde(default = "default_box_radius")]
        radius: f32,
        #[serde(default = "default_border_width")]
        border_width: f32,
        #[serde(default = "default_border_color")]
        border_color: String,
        #[serde(default = "default_true")]
        shadow: bool,
        #[serde(default = "default_one_f32")]
        zoom: f32,
        #[serde(default = "default_one_f32")]
        contrast: f32,
        #[serde(default)]
        tag: Option<TagSpec>,
    },
    Rect {
        x: f32,
        y: f32,
        width: f32,
        height: f32,
        #[serde(default)]
        radius: f32,
        fill: String,
        #[serde(default)]
        border_width: f32,
        #[serde(default)]
        border_color: Option<String>,
        #[serde(default)]
        text: Option<String>,
        #[serde(default)]
        text_color: Option<String>,
        #[serde(default)]
        text_size: Option<f32>,
        #[serde(default)]
        value_keyframes: Vec<ValueKeyframe>,
        #[serde(default)]
        fill_keyframes: Vec<ColorKeyframe>,
    },
    Panel {
        x: f32,
        y: f32,
        width: f32,
        height: f32,
        #[serde(default = "default_panel_radius")]
        radius: f32,
        #[serde(default = "default_panel_bg")]
        bg: String,
        #[serde(default = "default_panel_border")]
        border_color: String,
        #[serde(default)]
        accent_top: Option<String>,
        #[serde(default = "default_true")]
        shadow: bool,
    },
    Pedestal {
        x: f32,
        y: f32,
        width: f32,
        max_height: f32,
        fraction: f32,
        color: String,
        top_text: String,
        bottom_text: String,
        #[serde(default)]
        sub_text: Option<String>,
        #[serde(default = "default_bar_anim_dur")]
        anim_duration: f64,
    },
    Donut {
        cx: f32,
        cy: f32,
        radius: f32,
        #[serde(default = "default_donut_stroke")]
        stroke_width: f32,
        fraction: f32,
        color: String,
        #[serde(default = "default_donut_track")]
        track_color: String,
        value_text: String,
        #[serde(default)]
        label: Option<String>,
        #[serde(default)]
        ref_fraction: Option<f32>,
        #[serde(default)]
        ref_color: Option<String>,
        #[serde(default = "default_donut_anim_dur")]
        anim_duration: f64,
    },
    Door {
        x: f32,
        y: f32,
        width: f32,
        height: f32,
        open_at: f64,
        #[serde(default = "default_door_open_dur")]
        open_duration: f64,
        #[serde(default = "default_gold")]
        frame_color: String,
        #[serde(default = "default_door_fill")]
        door_color: String,
        #[serde(default = "default_gold")]
        glow_color: String,
    },
    Line {
        x1: f32,
        y1: f32,
        x2: f32,
        y2: f32,
        #[serde(default = "default_line_stroke")]
        stroke_width: f32,
        color: String,
        #[serde(default = "default_line_anim_dur")]
        anim_duration: f64,
    },
    Arrow {
        x1: f32,
        y1: f32,
        x2: f32,
        y2: f32,
        #[serde(default = "default_arrow_stroke")]
        stroke_width: f32,
        #[serde(default = "default_arrow_head")]
        head_size: f32,
        color: String,
        #[serde(default = "default_line_anim_dur")]
        anim_duration: f64,
    },
    FloorGlow {
        cx: f32,
        cy: f32,
        rx: f32,
        ry: f32,
        color: String,
        #[serde(default = "default_glow_alpha")]
        intensity: f32,
    },
    Card {
        x: f32,
        y: f32,
        width: f32,
        height: f32,
        #[serde(default = "default_paper")]
        bg: String,
        #[serde(default)]
        accent_top: Option<String>,
        #[serde(default = "default_true")]
        fold: bool,
        #[serde(default)]
        kicker: Option<String>,
        #[serde(default)]
        title: Option<String>,
        #[serde(default)]
        subtitle: Option<String>,
    },
    Text {
        text: String,
        x: f32,
        y: f32,
        #[serde(default = "default_text_width")]
        width: f32,
        size: f32,
        color: String,
        #[serde(default)]
        font: FontRole,
        #[serde(default)]
        align: TextAlign,
        #[serde(default)]
        counter: Option<CounterSpec>,
        #[serde(default)]
        value_keyframes: Vec<ValueKeyframe>,
    },
    Chip {
        text: String,
        x: f32,
        y: f32,
        bg: String,
        fg: String,
        #[serde(default = "default_chip_size")]
        font_size: f32,
    },
    HudBox {
        label: String,
        value: String,
        x: f32,
        y: f32,
        width: f32,
        #[serde(default = "default_cyan")]
        value_color: String,
        #[serde(default = "default_hud_value_size")]
        value_size: f32,
        #[serde(default)]
        accent_top: Option<String>,
        #[serde(default)]
        value_keyframes: Vec<ValueKeyframe>,
        #[serde(default)]
        label_keyframes: Vec<ValueKeyframe>,
    },
    Ring {
        cx: f32,
        cy: f32,
        diameter: f32,
        color: String,
        #[serde(default = "default_ring_stroke")]
        stroke_width: f32,
        #[serde(default = "default_true")]
        glow: bool,
    },
    Stamp {
        text: String,
        x: f32,
        y: f32,
        color: String,
        #[serde(default = "default_stamp_size")]
        font_size: f32,
    },
    Legend {
        label: String,
        dot_color: String,
        x: f32,
        y: f32,
    },
    ProgressBar {
        label: String,
        value_text: String,
        fraction: f32,
        x: f32,
        y: f32,
        bar_width: f32,
        #[serde(default = "default_bar_height")]
        height: f32,
        #[serde(default = "default_label_width")]
        label_width: f32,
        color: String,
        #[serde(default)]
        label_color: Option<String>,
        #[serde(default = "default_bar_font_size")]
        font_size: f32,
        #[serde(default = "default_bar_val_size")]
        value_size: f32,
        #[serde(default = "default_bar_anim_dur")]
        anim_duration: f64,
    },
    Captions {
        items: Vec<CaptionItem>,
        #[serde(default = "default_caption_y")]
        y: f32,
        #[serde(default = "default_caption_size")]
        font_size: f32,
    },
}

fn default_one_f32() -> f32 {
    1.0
}
fn default_panel_radius() -> f32 {
    14.0
}
fn default_panel_bg() -> String {
    "rgba(13,18,29,0.90)".to_string()
}
fn default_panel_border() -> String {
    "rgba(255,255,255,0.13)".to_string()
}
fn default_donut_stroke() -> f32 {
    34.0
}
fn default_donut_track() -> String {
    "rgba(255,255,255,0.09)".to_string()
}
fn default_donut_anim_dur() -> f64 {
    1.1
}
fn default_door_open_dur() -> f64 {
    1.1
}
fn default_gold() -> String {
    "#FACC15".to_string()
}
fn default_door_fill() -> String {
    "#131926".to_string()
}
fn default_line_stroke() -> f32 {
    6.0
}
fn default_line_anim_dur() -> f64 {
    0.45
}
fn default_arrow_stroke() -> f32 {
    10.0
}
fn default_arrow_head() -> f32 {
    34.0
}
fn default_glow_alpha() -> f32 {
    0.22
}
fn default_bar_font_size() -> f32 {
    32.0
}
fn default_bar_val_size() -> f32 {
    46.0
}

fn default_box_radius() -> f32 {
    14.0
}
fn default_border_width() -> f32 {
    3.0
}
fn default_border_color() -> String {
    "rgba(255,255,255,0.16)".to_string()
}
fn default_paper() -> String {
    "#ECE5D5".to_string()
}
fn default_text_width() -> f32 {
    1700.0
}
fn default_chip_size() -> f32 {
    30.0
}
fn default_cyan() -> String {
    "#00D4FF".to_string()
}
fn default_hud_value_size() -> f32 {
    78.0
}
fn default_ring_stroke() -> f32 {
    6.0
}
fn default_stamp_size() -> f32 {
    76.0
}
fn default_bar_height() -> f32 {
    62.0
}
fn default_label_width() -> f32 {
    260.0
}
fn default_bar_anim_dur() -> f64 {
    0.8
}
fn default_caption_y() -> f32 {
    955.0
}
fn default_caption_size() -> f32 {
    44.0
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct AudioClipSpec {
    pub src: String,
    #[serde(default)]
    pub start: f64,
    #[serde(default)]
    pub duration: Option<f64>,
    #[serde(default)]
    pub media_start: f64,
    #[serde(default = "default_vol")]
    pub volume: f32,
    #[serde(default)]
    pub fade_in: f64,
    #[serde(default)]
    pub fade_out: f64,
}

fn default_vol() -> f32 {
    1.0
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub struct Rgba {
    pub r: u8,
    pub g: u8,
    pub b: u8,
    pub a: u8,
}

impl Rgba {
    pub fn parse(s: &str) -> Result<Self> {
        let s = s.trim();
        if let Some(hex) = s.strip_prefix('#') {
            match hex.len() {
                3 => {
                    let r = u8::from_str_radix(&hex[0..1], 16)? * 17;
                    let g = u8::from_str_radix(&hex[1..2], 16)? * 17;
                    let b = u8::from_str_radix(&hex[2..3], 16)? * 17;
                    Ok(Self { r, g, b, a: 255 })
                }
                6 => {
                    let r = u8::from_str_radix(&hex[0..2], 16)?;
                    let g = u8::from_str_radix(&hex[2..4], 16)?;
                    let b = u8::from_str_radix(&hex[4..6], 16)?;
                    Ok(Self { r, g, b, a: 255 })
                }
                8 => {
                    let r = u8::from_str_radix(&hex[0..2], 16)?;
                    let g = u8::from_str_radix(&hex[2..4], 16)?;
                    let b = u8::from_str_radix(&hex[4..6], 16)?;
                    let a = u8::from_str_radix(&hex[6..8], 16)?;
                    Ok(Self { r, g, b, a })
                }
                _ => bail!("invalid hex color: {s}"),
            }
        } else if let Some(inner) = s.strip_prefix("rgba(").and_then(|v| v.strip_suffix(')')) {
            let parts: Vec<&str> = inner.split(',').map(|p| p.trim()).collect();
            if parts.len() != 4 {
                bail!("invalid rgba color: {s}");
            }
            let r: u8 = parts[0].parse()?;
            let g: u8 = parts[1].parse()?;
            let b: u8 = parts[2].parse()?;
            let af: f32 = parts[3].parse()?;
            let a = (af.clamp(0.0, 1.0) * 255.0).round() as u8;
            Ok(Self { r, g, b, a })
        } else {
            bail!("unsupported color format: {s}");
        }
    }

    pub fn to_f32_array(self) -> [f32; 4] {
        [
            self.r as f32 / 255.0,
            self.g as f32 / 255.0,
            self.b as f32 / 255.0,
            self.a as f32 / 255.0,
        ]
    }

    pub fn with_alpha_mul(self, factor: f32) -> Self {
        let a = ((self.a as f32) * factor.clamp(0.0, 1.0)).round() as u8;
        Self { a, ..self }
    }
}

impl ProjectSpec {
    pub fn resolve_base_dir(&self, spec_path: &Path) -> PathBuf {
        if let Some(dir) = &self.base_dir {
            let p = PathBuf::from(dir);
            if p.is_absolute() {
                p
            } else {
                spec_path
                    .parent()
                    .unwrap_or_else(|| Path::new("."))
                    .join(p)
            }
        } else {
            spec_path
                .parent()
                .unwrap_or_else(|| Path::new("."))
                .to_path_buf()
        }
    }

    pub fn resolve_asset(&self, base_dir: &Path, rel_or_abs: &str) -> PathBuf {
        let p = PathBuf::from(rel_or_abs);
        if p.is_absolute() {
            p
        } else {
            base_dir.join(p)
        }
    }

    pub fn total_frames(&self) -> u32 {
        (self.duration * self.fps as f64).ceil().max(1.0) as u32
    }

    pub fn validate(&self, spec_path: &Path) -> Result<Vec<String>> {
        let mut warnings = Vec::new();
        if self.width == 0
            || self.height == 0
            || !self.width.is_multiple_of(2)
            || !self.height.is_multiple_of(2)
        {
            bail!(
                "canvas dimensions must be positive even numbers, got {}x{}",
                self.width,
                self.height
            );
        }
        if self.fps == 0 || self.fps > 240 {
            bail!("fps must be in 1..=240, got {}", self.fps);
        }
        if self.duration <= 0.0 {
            bail!("duration must be positive, got {}", self.duration);
        }
        Rgba::parse(&self.background)?;

        let base_dir = self.resolve_base_dir(spec_path);
        for layer in &self.layers {
            if let Some(end) = layer.end
                && end < layer.start
            {
                bail!(
                    "layer '{}' has end ({}) < start ({})",
                    layer.id,
                    end,
                    layer.start
                );
            }
            match &layer.kind {
                LayerKind::VideoBox {
                    border_color,
                    segments,
                    ..
                } => {
                    Rgba::parse(border_color)?;
                    if segments.is_empty() {
                        warnings.push(format!("video_box '{}' has no segments", layer.id));
                    }
                    for seg in segments {
                        if seg.duration <= 0.0 {
                            bail!("video_box '{}' has non-positive segment duration ({})", layer.id, seg.duration);
                        }
                        if seg.playback_rate <= 0.0 {
                            bail!("video_box '{}' has non-positive playback_rate", layer.id);
                        }
                        let path = self.resolve_asset(&base_dir, &seg.src);
                        if !path.exists() {
                            bail!(
                                "video_box '{}' references missing file: {}",
                                layer.id,
                                path.display()
                            );
                        }
                    }
                }
                LayerKind::ImageBox {
                    src, border_color, ..
                } => {
                    Rgba::parse(border_color)?;
                    let path = self.resolve_asset(&base_dir, src);
                    if !path.exists() {
                        bail!(
                            "image_box '{}' references missing file: {}",
                            layer.id,
                            path.display()
                        );
                    }
                }
                LayerKind::Rect {
                    fill,
                    border_color,
                    fill_keyframes,
                    ..
                } => {
                    Rgba::parse(fill)?;
                    if let Some(bc) = border_color {
                        Rgba::parse(bc)?;
                    }
                    for kf in fill_keyframes {
                        Rgba::parse(&kf.fill)?;
                        if let Some(bc) = &kf.border_color {
                            Rgba::parse(bc)?;
                        }
                    }
                }
                LayerKind::Panel {
                    bg,
                    border_color,
                    accent_top,
                    ..
                } => {
                    Rgba::parse(bg)?;
                    Rgba::parse(border_color)?;
                    if let Some(a) = accent_top {
                        Rgba::parse(a)?;
                    }
                }
                LayerKind::Pedestal { color, .. } => {
                    Rgba::parse(color)?;
                }
                LayerKind::Donut {
                    color,
                    track_color,
                    ref_color,
                    ..
                } => {
                    Rgba::parse(color)?;
                    Rgba::parse(track_color)?;
                    if let Some(rc) = ref_color {
                        Rgba::parse(rc)?;
                    }
                }
                LayerKind::Door {
                    frame_color,
                    door_color,
                    glow_color,
                    ..
                } => {
                    Rgba::parse(frame_color)?;
                    Rgba::parse(door_color)?;
                    Rgba::parse(glow_color)?;
                }
                LayerKind::Line { color, .. } => {
                    Rgba::parse(color)?;
                }
                LayerKind::Arrow { color, .. } => {
                    Rgba::parse(color)?;
                }
                LayerKind::FloorGlow { color, .. } => {
                    Rgba::parse(color)?;
                }
                LayerKind::Card { bg, accent_top, .. } => {
                    Rgba::parse(bg)?;
                    if let Some(a) = accent_top {
                        Rgba::parse(a)?;
                    }
                }
                LayerKind::Text { color, .. } => {
                    Rgba::parse(color)?;
                }
                LayerKind::Chip { bg, fg, .. } => {
                    Rgba::parse(bg)?;
                    Rgba::parse(fg)?;
                }
                LayerKind::HudBox { value_color, .. } => {
                    Rgba::parse(value_color)?;
                }
                LayerKind::Ring { color, .. } => {
                    Rgba::parse(color)?;
                }
                LayerKind::Stamp { color, .. } => {
                    Rgba::parse(color)?;
                }
                LayerKind::Legend { dot_color, .. } => {
                    Rgba::parse(dot_color)?;
                }
                LayerKind::ProgressBar { color, .. } => {
                    Rgba::parse(color)?;
                }
                LayerKind::Captions { .. } => {}
            }
        }

        for clip in &self.audio {
            let path = self.resolve_asset(&base_dir, &clip.src);
            if !path.exists() {
                bail!("audio clip references missing file: {}", path.display());
            }
        }

        Ok(warnings)
    }
}

impl EssaySpec {
    pub fn resolve_base_dir(&self, spec_path: &Path) -> PathBuf {
        if let Some(dir) = &self.base_dir {
            let p = PathBuf::from(dir);
            if p.is_absolute() {
                p
            } else {
                spec_path
                    .parent()
                    .unwrap_or_else(|| Path::new("."))
                    .join(p)
            }
        } else {
            spec_path
                .parent()
                .unwrap_or_else(|| Path::new("."))
                .to_path_buf()
        }
    }

    pub fn total_duration(&self) -> f64 {
        self.scenes.iter().map(|s| s.duration).sum()
    }

    pub fn total_frames(&self) -> u32 {
        self.scenes
            .iter()
            .map(|s| (s.duration * self.fps as f64).ceil().max(1.0) as u32)
            .sum()
    }

    pub fn scene_to_project(&self, scene: &EssayScene) -> ProjectSpec {
        ProjectSpec {
            width: self.width,
            height: self.height,
            fps: self.fps,
            duration: scene.duration,
            base_dir: self.base_dir.clone(),
            background: scene.background.clone(),
            desk_pattern: scene.desk_pattern,
            bg_mood: scene.bg_mood.clone(),
            vignette: scene.vignette,
            normalize_lufs: None,
            encoder: self.encoder.clone(),
            fonts: self.fonts.clone(),
            camera: scene.camera.clone(),
            camera_shakes: scene.camera_shakes.clone(),
            layers: scene.layers.clone(),
            audio: Vec::new(),
        }
    }

    pub fn validate(&self, spec_path: &Path) -> Result<Vec<String>> {
        if self.scenes.is_empty() {
            bail!("essay spec must contain at least one scene");
        }
        let mut warnings = Vec::new();
        let mut seen_ids = std::collections::HashSet::new();
        for sc in &self.scenes {
            if !seen_ids.insert(sc.id.as_str()) {
                bail!("duplicate scene id '{}' in essay", sc.id);
            }
            let proj = self.scene_to_project(sc);
            for w in proj.validate(spec_path)? {
                warnings.push(format!("[{}] {w}", sc.id));
            }
        }
        let base_dir = self.resolve_base_dir(spec_path);
        for clip in &self.audio {
            let p = PathBuf::from(&clip.src);
            let path = if p.is_absolute() { p } else { base_dir.join(p) };
            if !path.exists() {
                bail!("essay audio clip references missing file: {}", path.display());
            }
        }
        Ok(warnings)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_colors() {
        assert_eq!(
            Rgba::parse("#07090E").unwrap(),
            Rgba {
                r: 7,
                g: 9,
                b: 14,
                a: 255
            }
        );
        assert_eq!(
            Rgba::parse("#fff").unwrap(),
            Rgba {
                r: 255,
                g: 255,
                b: 255,
                a: 255
            }
        );
        assert_eq!(
            Rgba::parse("rgba(255, 255, 255, 0.5)").unwrap(),
            Rgba {
                r: 255,
                g: 255,
                b: 255,
                a: 128
            }
        );
        assert!(Rgba::parse("not-a-color").is_err());
    }

    #[test]
    fn json_schemas_export_valid_json() {
        let p = export_project_schema().unwrap();
        let e = export_essay_schema().unwrap();
        let pv: serde_json::Value = serde_json::from_str(&p).unwrap();
        let ev: serde_json::Value = serde_json::from_str(&e).unwrap();
        assert_eq!(pv["title"], "ProjectSpec");
        assert_eq!(ev["title"], "EssaySpec");
    }

    #[test]
    fn encoder_defaults_to_auto_codec() {
        let cfg = EncoderConfig::default();
        assert_eq!(cfg.codec, "auto");
        let parsed: EncoderConfig = serde_json::from_str("{}").unwrap();
        assert_eq!(parsed.codec, "auto");
    }
}
