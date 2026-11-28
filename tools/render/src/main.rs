mod easing;
mod encode;
mod essay;
mod ffmpeg;
mod gpu;
mod output;
mod render;
mod schema;
mod text;
mod timing;
mod video;

use anyhow::{Result, bail};
use clap::{Parser, Subcommand};
use render::RenderOptions;
use schema::{SpecDocument, export_essay_schema, export_project_schema};
use std::path::PathBuf;

#[derive(Parser)]
#[command(
    name = "cliphub-render",
    about = "GPU-accelerated Rust video & motion-graphics renderer (wgpu + Vello + NVENC/AMF/QSV)"
)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Inspect detected GPU adapter and FFmpeg hardware encoder availability
    Info {
        /// Output machine-readable JSON
        #[arg(long)]
        json: bool,
    },
    /// Export canonical JSON Schema derived from Rust spec structs
    Schema {
        /// Which schema to export: project or essay
        #[arg(long, default_value = "essay")]
        kind: String,
        /// Optional output path (prints to stdout when omitted)
        #[arg(short, long)]
        out: Option<PathBuf>,
    },
    /// Validate a single-scene ProjectSpec or multi-scene EssaySpec JSON file
    Validate {
        /// Path to the JSON scene or essay spec
        spec: PathBuf,
    },
    /// Render PNG snapshots at specific timestamps (seconds) for fast visual QA
    Snapshot {
        /// Path to the JSON scene or essay spec
        spec: PathBuf,
        /// Comma-separated timestamps in seconds (e.g. 0.5,2.5,3.5,6.0)
        #[arg(long, value_delimiter = ',')]
        at: Vec<f64>,
        /// For multi-scene essays, which scene ID to snapshot (defaults to first scene)
        #[arg(long)]
        scene: Option<String>,
        /// Output directory for PNG frames
        #[arg(short, long, default_value = "snapshots")]
        out: PathBuf,
        /// Abort if a discrete GPU adapter is not available
        #[arg(long)]
        require_gpu: bool,
    },
    /// Render a single scene or multi-scene essay JSON to MP4 on GPU + hardware encoder
    Render {
        /// Path to the JSON scene or essay spec
        spec: PathBuf,
        /// Output MP4 file path
        #[arg(short, long)]
        out: PathBuf,
        /// Override video encoder codec (auto, h264_nvenc, hevc_nvenc, av1_nvenc, h264_amf, hevc_amf, h264_qsv, hevc_qsv, libx264)
        #[arg(long)]
        codec: Option<String>,
        /// Override encoder preset (e.g. p1..p7 for NVENC, speed/balanced/quality for AMF, fast/medium for QSV/libx264)
        #[arg(long)]
        preset: Option<String>,
        /// Override CQ/CRF quality value
        #[arg(long)]
        cq: Option<u32>,
        /// Parallel scene workers for multi-scene essays (default: 3)
        #[arg(long, default_value_t = 3)]
        workers: usize,
        /// Override audio loudness normalization target in LUFS (e.g. -14.0 for YouTube)
        #[arg(long, allow_hyphen_values = true)]
        lufs: Option<f64>,
        /// Fail immediately unless discrete GPU + hardware encoding (*_nvenc, *_amf, *_qsv) are active
        #[arg(long)]
        require_gpu: bool,
        /// Enable CUDA hardware-accelerated video decoding (useful for 4K HEVC/AV1)
        #[arg(long)]
        cuda_decode: bool,
        /// Print structured JSON render telemetry to stdout
        #[arg(long)]
        stats_json: bool,
    },
}

fn main() -> Result<()> {
    let cli = Cli::parse();
    match cli.command {
        Commands::Info { json } => {
            ffmpeg::command()?;
            let gpu = gpu::probe_gpu()?;
            let encoders = encode::detect_encoders();

            if json {
                let obj = serde_json::json!({
                    "gpu_adapter": gpu.adapter_name,
                    "backend": gpu.backend,
                    "device_type": gpu.device_type,
                    "is_discrete_gpu": gpu.is_discrete_gpu,
                    "nvenc_encoders": encoders.nvenc_encoders,
                    "hardware_encoders": encoders.hardware_encoders,
                    "recommended_codec": encoders.recommended_codec,
                });
                println!("{}", serde_json::to_string_pretty(&obj)?);
            } else {
                println!("GPU Adapter : {}", gpu.adapter_name);
                println!("Backend     : {}", gpu.backend);
                println!("Device Type : {}", gpu.device_type);
                println!("FFmpeg NVENC: {}", encoders.nvenc_encoders.join(", "));
                println!("HW Encoders : {}", encoders.hardware_encoders.join(", "));
                println!("Recommended : {}", encoders.recommended_codec);
            }
        }
        Commands::Schema { kind, out } => {
            let json = match kind.as_str() {
                "project" => export_project_schema()?,
                "essay" => export_essay_schema()?,
                other => bail!("unsupported schema kind '{other}' (expected 'project' or 'essay')"),
            };
            if let Some(path) = out {
                if let Some(parent) = path.parent()
                    && !parent.as_os_str().is_empty()
                {
                    std::fs::create_dir_all(parent)?;
                }
                std::fs::write(&path, &json)?;
                println!("Wrote {} schema to {}", kind, path.display());
            } else {
                println!("{json}");
            }
        }
        Commands::Validate { spec: spec_path } => match SpecDocument::load(&spec_path)? {
            SpecDocument::Project(spec) => {
                let warnings = spec.validate(&spec_path)?;
                println!(
                    "OK: {} (project {}x{} @ {} fps, {:.3}s = {} frames, {} layers, {} audio clips)",
                    spec_path.display(),
                    spec.width,
                    spec.height,
                    spec.fps,
                    spec.duration,
                    spec.total_frames(),
                    spec.layers.len(),
                    spec.audio.len()
                );
                for w in warnings {
                    println!("WARN: {w}");
                }
            }
            SpecDocument::Essay(spec) => {
                let warnings = spec.validate(&spec_path)?;
                println!(
                    "OK: {} (essay {} scenes, {}x{} @ {} fps, {:.3}s = {} frames, {} audio clips)",
                    spec_path.display(),
                    spec.scenes.len(),
                    spec.width,
                    spec.height,
                    spec.fps,
                    spec.total_duration(),
                    spec.total_frames(),
                    spec.audio.len()
                );
                for w in warnings {
                    println!("WARN: {w}");
                }
            }
        },
        Commands::Snapshot {
            spec: spec_path,
            at,
            scene,
            out,
            require_gpu,
        } => {
            let proj = match SpecDocument::load(&spec_path)? {
                SpecDocument::Project(p) => {
                    p.validate(&spec_path)?;
                    p
                }
                SpecDocument::Essay(e) => {
                    e.validate(&spec_path)?;
                    let chosen = if let Some(sid) = scene {
                        e.scenes
                            .iter()
                            .find(|s| s.id == sid)
                            .ok_or_else(|| anyhow::anyhow!("scene '{sid}' not found in essay"))?
                    } else {
                        &e.scenes[0]
                    };
                    e.scene_to_project(chosen)
                }
            };
            let t0 = std::time::Instant::now();
            let paths = render::render_snapshots(&proj, &spec_path, &at, &out, require_gpu)?;
            println!(
                "Rendered {} snapshots in {} ms:",
                paths.len(),
                t0.elapsed().as_millis()
            );
            for p in paths {
                println!("  {}", p.display());
            }
        }
        Commands::Render {
            spec: spec_path,
            out,
            codec,
            preset,
            cq,
            workers,
            lufs,
            require_gpu,
            cuda_decode,
            stats_json,
        } => {
            let opts = RenderOptions {
                require_gpu,
                cuda_decode,
            };
            match SpecDocument::load(&spec_path)? {
                SpecDocument::Project(mut spec) => {
                    if let Some(c) = codec {
                        spec.encoder.codec = c;
                    }
                    if let Some(p) = preset {
                        spec.encoder.preset = p;
                    }
                    if let Some(q) = cq {
                        spec.encoder.cq = q;
                    }
                    if let Some(l) = lufs {
                        spec.normalize_lufs = Some(l);
                    }
                    spec.validate(&spec_path)?;
                    let stats = render::render_project(&spec, &spec_path, &out, opts)?;
                    if stats_json {
                        println!("{}", serde_json::to_string_pretty(&stats)?);
                    } else {
                        println!("=== cliphub-render complete ===");
                        println!("Output       : {}", stats.output);
                        println!("GPU          : {} ({})", stats.gpu_name, stats.gpu_backend);
                        println!(
                            "Video        : {}x{} @ {} fps ({} frames, {:.3}s) via {}",
                            stats.width,
                            stats.height,
                            stats.fps,
                            stats.total_frames,
                            stats.duration_sec,
                            stats.codec
                        );
                        println!("GPU init     : {} ms", stats.init_ms);
                        println!("Render+Pipe  : {} ms", stats.render_ms);
                        println!("Encode flush : {} ms", stats.encode_flush_ms);
                        println!("Audio mux    : {} ms", stats.audio_mux_ms);
                        println!(
                            "Total wall   : {} ms ({:.1} fps | {:.2}x realtime)",
                            stats.total_ms, stats.effective_fps, stats.realtime_factor
                        );
                    }
                }
                SpecDocument::Essay(mut spec) => {
                    if let Some(c) = codec {
                        spec.encoder.codec = c;
                    }
                    if let Some(p) = preset {
                        spec.encoder.preset = p;
                    }
                    if let Some(q) = cq {
                        spec.encoder.cq = q;
                    }
                    if let Some(l) = lufs {
                        spec.normalize_lufs = Some(l);
                    }
                    spec.validate(&spec_path)?;
                    let stats = essay::render_essay(&spec, &spec_path, &out, opts, workers)?;
                    if stats_json {
                        println!("{}", serde_json::to_string_pretty(&stats)?);
                    } else {
                        println!("=== cliphub-render essay complete ===");
                        println!("Output       : {}", stats.output);
                        println!(
                            "GPU          : {} ({}) | {} workers",
                            stats.gpu_name, stats.gpu_backend, stats.workers
                        );
                        println!(
                            "Essay        : {} scenes, {}x{} @ {} fps ({} frames, {:.3}s) via {}",
                            stats.scene_count,
                            stats.width,
                            stats.height,
                            stats.fps,
                            stats.total_frames,
                            stats.duration_sec,
                            stats.codec
                        );
                        println!(
                            "Scenes wall  : {} ms ({:.1} aggregate fps)",
                            stats.scenes_wall_ms, stats.aggregate_fps
                        );
                        println!("Concat       : {} ms", stats.concat_ms);
                        println!("Audio mux    : {} ms", stats.audio_mux_ms);
                        println!(
                            "Total wall   : {} ms ({:.2}x realtime)",
                            stats.total_ms, stats.realtime_factor
                        );
                    }
                }
            }
        }
    }
    Ok(())
}
