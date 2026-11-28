use crate::encode::{VideoEncoder, canonical_codec_name, mux_audio, verify_gpu_codec_name};
use crate::gpu::GpuCompositor;
use crate::output::PendingOutput;
use crate::schema::ProjectSpec;
use crate::text::FontRegistry;
use crate::video::MediaPool;
use anyhow::{Context, Result};
use serde::Serialize;
use std::path::Path;
use std::time::Instant;

#[derive(Debug, Clone, Copy, Default)]
pub struct RenderOptions {
    pub require_gpu: bool,
    pub cuda_decode: bool,
}

#[derive(Debug, Clone, Serialize)]
pub struct RenderStats {
    pub output: String,
    pub gpu_name: String,
    pub gpu_backend: String,
    pub is_discrete_gpu: bool,
    pub codec: String,
    pub width: u32,
    pub height: u32,
    pub fps: u32,
    pub total_frames: u32,
    pub duration_sec: f64,
    pub init_ms: u128,
    pub render_ms: u128,
    pub encode_flush_ms: u128,
    pub audio_mux_ms: u128,
    pub total_ms: u128,
    pub effective_fps: f64,
    pub realtime_factor: f64,
    pub output_bytes: u64,
}

pub fn render_project(
    spec: &ProjectSpec,
    spec_path: &Path,
    output_path: &Path,
    opts: RenderOptions,
) -> Result<RenderStats> {
    let pending = PendingOutput::create(output_path)?;
    let base_dir = spec.resolve_base_dir(spec_path);
    let fonts = FontRegistry::load(&spec.fonts, &base_dir)?;
    let t_init = Instant::now();
    let mut gpu = GpuCompositor::new(spec.width, spec.height, opts.require_gpu)?;
    let init_ms = t_init.elapsed().as_millis();
    let rendered = pending.file("render.mp4");
    let mut stats =
        render_project_with_ctx(spec, spec_path, &rendered, opts, &fonts, &mut gpu, init_ms)?;
    pending.publish(&rendered)?;
    stats.output = output_path.display().to_string();
    Ok(stats)
}

pub fn render_project_with_ctx(
    spec: &ProjectSpec,
    spec_path: &Path,
    output_path: &Path,
    opts: RenderOptions,
    fonts: &FontRegistry,
    gpu: &mut GpuCompositor,
    init_ms: u128,
) -> Result<RenderStats> {
    let t_all = Instant::now();
    let resolved_codec = canonical_codec_name(&spec.encoder.codec)?;
    if opts.require_gpu {
        verify_gpu_codec_name(resolved_codec)?;
    }
    let codec = resolved_codec.to_string();
    let mut resolved_encoder = spec.encoder.clone();
    resolved_encoder.codec = codec.clone();

    let base_dir = spec.resolve_base_dir(spec_path);

    let video_only_path = if spec.audio.is_empty() {
        output_path.to_path_buf()
    } else {
        output_path.with_extension("video_only.mp4")
    };

    let encoder = VideoEncoder::spawn(
        &video_only_path,
        spec.width,
        spec.height,
        spec.fps,
        &resolved_encoder,
    )?;

    let mut media = MediaPool::new(opts.cuda_decode);
    let total_frames = spec.total_frames();
    let t_render = Instant::now();

    for frame_idx in 0..total_frames {
        let t = frame_idx as f64 / spec.fps as f64;
        let mut buf = encoder.take_buffer();
        let ready =
            gpu.submit_and_collect(spec, &base_dir, fonts, &mut media, t, false, &mut buf)?;
        if ready {
            encoder.push_frame(buf)?;
        }
    }
    let mut final_buf = encoder.take_buffer();
    if gpu.flush_pending(&mut final_buf)? {
        encoder.push_frame(final_buf)?;
    }

    let render_ms = t_render.elapsed().as_millis();
    let t_flush = Instant::now();
    encoder.finish()?;
    let encode_flush_ms = t_flush.elapsed().as_millis();

    let t_mux = Instant::now();
    if !spec.audio.is_empty() {
        mux_audio(&video_only_path, output_path, spec, &base_dir)?;
    }
    let audio_mux_ms = t_mux.elapsed().as_millis();

    let total_ms = t_all.elapsed().as_millis() + init_ms;
    let render_plus_flush_s = ((render_ms + encode_flush_ms) as f64 / 1000.0).max(0.001);
    let effective_fps = total_frames as f64 / render_plus_flush_s;
    let total_s = (total_ms as f64 / 1000.0).max(0.001);
    let realtime_factor = spec.duration / total_s;
    let output_bytes = std::fs::metadata(output_path).map(|m| m.len()).unwrap_or(0);

    Ok(RenderStats {
        output: output_path.display().to_string(),
        gpu_name: gpu.info.adapter_name.clone(),
        gpu_backend: gpu.info.backend.clone(),
        is_discrete_gpu: gpu.info.is_discrete_gpu,
        codec,
        width: spec.width,
        height: spec.height,
        fps: spec.fps,
        total_frames,
        duration_sec: spec.duration,
        init_ms,
        render_ms,
        encode_flush_ms,
        audio_mux_ms,
        total_ms,
        effective_fps,
        realtime_factor,
        output_bytes,
    })
}

pub fn render_snapshots(
    spec: &ProjectSpec,
    spec_path: &Path,
    timestamps: &[f64],
    out_dir: &Path,
    require_gpu: bool,
) -> Result<Vec<std::path::PathBuf>> {
    std::fs::create_dir_all(out_dir)?;
    let base_dir = spec.resolve_base_dir(spec_path);
    let fonts = FontRegistry::load(&spec.fonts, &base_dir)?;
    let mut gpu = GpuCompositor::new(spec.width, spec.height, require_gpu)?;
    let mut media = MediaPool::new(false);
    let mut buf = vec![0u8; (spec.width as usize) * (spec.height as usize) * 4];
    let mut saved = Vec::with_capacity(timestamps.len());

    for &t in timestamps {
        gpu.render_frame_sync(spec, &base_dir, &fonts, &mut media, t, true, &mut buf)?;
        let fname = format!("snap_{:06.2}s.png", t).replace('.', "_");
        let fname = fname.replacen("_png", ".png", 1);
        let out_path = out_dir.join(fname);
        image::save_buffer(
            &out_path,
            &buf,
            spec.width,
            spec.height,
            image::ExtendedColorType::Rgba8,
        )
        .with_context(|| format!("failed saving snapshot {}", out_path.display()))?;
        saved.push(out_path);
    }

    Ok(saved)
}
