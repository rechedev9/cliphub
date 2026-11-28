use crate::encode::{canonical_codec_name, concat_videos, mux_audio_clips, verify_gpu_codec_name};
use crate::gpu::GpuCompositor;
use crate::output::PendingOutput;
use crate::render::{RenderOptions, RenderStats, render_project_with_ctx};
use crate::schema::EssaySpec;
use crate::text::FontRegistry;
use anyhow::{Context, Result, bail};
use serde::Serialize;
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Instant;

#[derive(Debug, Clone, Serialize)]
pub struct EssaySceneResult {
    pub id: String,
    pub duration_sec: f64,
    pub total_frames: u32,
    pub render_ms: u128,
    pub effective_fps: f64,
}

#[derive(Debug, Clone, Serialize)]
pub struct EssayRenderStats {
    pub output: String,
    pub gpu_name: String,
    pub gpu_backend: String,
    pub codec: String,
    pub workers: usize,
    pub width: u32,
    pub height: u32,
    pub fps: u32,
    pub scene_count: usize,
    pub total_frames: u32,
    pub duration_sec: f64,
    pub scenes_wall_ms: u128,
    pub concat_ms: u128,
    pub audio_mux_ms: u128,
    pub total_ms: u128,
    pub aggregate_fps: f64,
    pub realtime_factor: f64,
    pub output_bytes: u64,
    pub scenes: Vec<EssaySceneResult>,
}

pub fn render_essay(
    spec: &EssaySpec,
    spec_path: &Path,
    output_path: &Path,
    opts: RenderOptions,
    requested_workers: usize,
) -> Result<EssayRenderStats> {
    let t_all = Instant::now();
    let resolved_codec = canonical_codec_name(&spec.encoder.codec)?;
    if opts.require_gpu {
        verify_gpu_codec_name(resolved_codec)?;
    }
    if spec.scenes.is_empty() {
        bail!("essay has no scenes to render");
    }

    let workers = requested_workers.clamp(1, spec.scenes.len().min(6));
    let pending = PendingOutput::create(output_path)?;

    let part_paths: Vec<PathBuf> = (0..spec.scenes.len())
        .map(|i| pending.file(&format!("scene_{i:03}.mp4")))
        .collect();
    let scene_frames = spec.scene_frame_counts();
    let results: Mutex<Vec<Option<RenderStats>>> = Mutex::new(vec![None; spec.scenes.len()]);
    let first_err: Mutex<Option<anyhow::Error>> = Mutex::new(None);
    let next_idx = AtomicUsize::new(0);

    let base_dir = spec.resolve_base_dir(spec_path);
    let shared_fonts = FontRegistry::load(&spec.fonts, &base_dir)?;

    let t_scenes = Instant::now();
    std::thread::scope(|scope| {
        for _ in 0..workers {
            let fonts_ref = &shared_fonts;
            let results_ref = &results;
            let first_err_ref = &first_err;
            let next_idx_ref = &next_idx;
            let part_paths_ref = &part_paths;
            let scene_frames_ref = &scene_frames;
            scope.spawn(move || {
                let t_gpu = Instant::now();
                let mut gpu = match GpuCompositor::new(spec.width, spec.height, opts.require_gpu) {
                    Ok(g) => g,
                    Err(e) => {
                        let mut slot = first_err_ref.lock().unwrap();
                        if slot.is_none() {
                            *slot = Some(e);
                        }
                        return;
                    }
                };
                let mut init_ms = t_gpu.elapsed().as_millis();
                loop {
                    if first_err_ref.lock().unwrap().is_some() {
                        break;
                    }
                    let idx = next_idx_ref.fetch_add(1, Ordering::Relaxed);
                    if idx >= spec.scenes.len() {
                        break;
                    }
                    let scene = &spec.scenes[idx];
                    let mut proj = spec.scene_to_project(scene);
                    proj.duration = scene_frames_ref[idx] as f64 / spec.fps as f64;
                    gpu.clear_media_cache();
                    match render_project_with_ctx(
                        &proj,
                        spec_path,
                        &part_paths_ref[idx],
                        opts,
                        fonts_ref,
                        &mut gpu,
                        init_ms,
                    ) {
                        Ok(st) => {
                            init_ms = 0;
                            results_ref.lock().unwrap()[idx] = Some(st);
                        }
                        Err(e) => {
                            let mut slot = first_err_ref.lock().unwrap();
                            if slot.is_none() {
                                *slot = Some(e.context(format!("scene '{}' failed", scene.id)));
                            }
                            break;
                        }
                    }
                }
            });
        }
    });
    let scenes_wall_ms = t_scenes.elapsed().as_millis();

    if let Some(err) = first_err.into_inner().unwrap() {
        return Err(err);
    }

    let scene_stats: Vec<RenderStats> = results
        .into_inner()
        .unwrap()
        .into_iter()
        .enumerate()
        .map(|(i, s)| s.with_context(|| format!("missing stats for scene {i}")))
        .collect::<Result<Vec<_>>>()?;

    let t_concat = Instant::now();
    let finished_path = pending.file("essay.mp4");
    let concat_video_path = if spec.audio.is_empty() {
        finished_path.clone()
    } else {
        pending.file("concat_video.mp4")
    };
    concat_videos(&part_paths, &concat_video_path)?;
    let concat_ms = t_concat.elapsed().as_millis();

    let t_mux = Instant::now();
    if !spec.audio.is_empty() {
        let base_dir = spec.resolve_base_dir(spec_path);
        mux_audio_clips(
            &concat_video_path,
            &finished_path,
            &spec.audio,
            spec.total_frames() as f64 / spec.fps as f64,
            spec.normalize_lufs,
            &base_dir,
        )?;
    }
    let audio_mux_ms = t_mux.elapsed().as_millis();
    pending.publish(&finished_path)?;

    let total_ms = t_all.elapsed().as_millis();
    let total_frames = spec.total_frames();
    let duration_sec = spec.total_duration();
    let scenes_wall_s = (scenes_wall_ms as f64 / 1000.0).max(0.001);
    let aggregate_fps = total_frames as f64 / scenes_wall_s;
    let total_s = (total_ms as f64 / 1000.0).max(0.001);
    let realtime_factor = duration_sec / total_s;
    let output_bytes = std::fs::metadata(output_path).map(|m| m.len()).unwrap_or(0);

    let gpu_name = scene_stats
        .first()
        .map(|s| s.gpu_name.clone())
        .unwrap_or_default();
    let gpu_backend = scene_stats
        .first()
        .map(|s| s.gpu_backend.clone())
        .unwrap_or_default();
    let codec = scene_stats
        .first()
        .map(|s| s.codec.clone())
        .unwrap_or_else(|| resolved_codec.to_string());

    let per_scene = spec
        .scenes
        .iter()
        .zip(scene_stats.iter())
        .map(|(sc, st)| EssaySceneResult {
            id: sc.id.clone(),
            duration_sec: sc.duration,
            total_frames: st.total_frames,
            render_ms: st.render_ms,
            effective_fps: st.effective_fps,
        })
        .collect();

    Ok(EssayRenderStats {
        output: output_path.display().to_string(),
        gpu_name,
        gpu_backend,
        codec,
        workers,
        width: spec.width,
        height: spec.height,
        fps: spec.fps,
        scene_count: spec.scenes.len(),
        total_frames,
        duration_sec,
        scenes_wall_ms,
        concat_ms,
        audio_mux_ms,
        total_ms,
        aggregate_fps,
        realtime_factor,
        output_bytes,
        scenes: per_scene,
    })
}
