use crate::schema::{AudioClipSpec, EncoderConfig, ProjectSpec};
use anyhow::{Context, Result, bail};
use std::io::Write;
use std::path::Path;
use std::process::{Child, Command, Stdio};
use std::sync::OnceLock;
use std::sync::mpsc::{self, Receiver, SyncSender};
use std::thread::{self, JoinHandle};

pub const AUTO_ENCODER_CASCADE: &[&str] = &["h264_nvenc", "h264_amf", "h264_qsv"];

#[derive(Debug, Clone)]
pub struct DetectedEncoders {
    pub hardware_encoders: Vec<String>,
    pub nvenc_encoders: Vec<String>,
    pub recommended_codec: &'static str,
}

static DETECTED_ENCODERS: OnceLock<DetectedEncoders> = OnceLock::new();

pub fn is_nvenc_codec(codec: &str) -> bool {
    matches!(
        codec,
        "h264_nvenc" | "nvenc" | "hevc_nvenc" | "hevc" | "av1_nvenc" | "av1"
    )
}

pub fn is_amf_codec(codec: &str) -> bool {
    matches!(codec, "h264_amf" | "amf" | "hevc_amf")
}

pub fn is_qsv_codec(codec: &str) -> bool {
    matches!(codec, "h264_qsv" | "qsv" | "hevc_qsv")
}

pub fn is_hardware_codec(codec: &str) -> bool {
    is_nvenc_codec(codec) || is_amf_codec(codec) || is_qsv_codec(codec)
}

pub fn parse_ffmpeg_hardware_encoders(stdout: &str) -> Vec<String> {
    let mut out = Vec::new();
    for line in stdout.lines() {
        let trimmed = line.trim_start();
        if !trimmed.starts_with('V') {
            continue;
        }
        let Some(name) = trimmed.split_whitespace().nth(1) else {
            continue;
        };
        let is_modern_video = name.starts_with("h264_")
            || name.starts_with("hevc_")
            || name.starts_with("av1_");
        let is_hw_suffix = name.ends_with("_nvenc")
            || name.ends_with("_amf")
            || name.ends_with("_qsv");
        if is_modern_video && is_hw_suffix && !out.iter().any(|existing| existing == name) {
            out.push(name.to_string());
        }
    }
    out
}

pub fn select_cascade_codec<F>(available_encoders: &[String], mut probe_fn: F) -> &'static str
where
    F: FnMut(&str) -> bool,
{
    for &candidate in AUTO_ENCODER_CASCADE {
        if available_encoders.iter().any(|e| e == candidate) && probe_fn(candidate) {
            return candidate;
        }
    }
    "libx264"
}

fn probe_ffmpeg_encoder_works(encoder: &str) -> bool {
    let pix_fmt = if is_qsv_codec(encoder) {
        "nv12"
    } else {
        "yuv420p"
    };
    Command::new("ffmpeg")
        .args([
            "-nostdin",
            "-hide_banner",
            "-v",
            "error",
            "-f",
            "lavfi",
            "-i",
            "color=c=black:s=256x256:r=30:d=0.05",
            "-frames:v",
            "1",
            "-pix_fmt",
            pix_fmt,
            "-c:v",
            encoder,
            "-f",
            "null",
            "-",
        ])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .map(|s| s.success())
        .unwrap_or(false)
}

pub fn detect_encoders() -> &'static DetectedEncoders {
    DETECTED_ENCODERS.get_or_init(|| {
        let hardware_encoders = Command::new("ffmpeg")
            .args(["-hide_banner", "-encoders"])
            .stdin(Stdio::null())
            .stderr(Stdio::null())
            .output()
            .map(|o| parse_ffmpeg_hardware_encoders(&String::from_utf8_lossy(&o.stdout)))
            .unwrap_or_default();

        let nvenc_encoders = hardware_encoders
            .iter()
            .filter(|e| e.ends_with("_nvenc"))
            .cloned()
            .collect();

        let recommended_codec =
            select_cascade_codec(&hardware_encoders, probe_ffmpeg_encoder_works);

        DetectedEncoders {
            hardware_encoders,
            nvenc_encoders,
            recommended_codec,
        }
    })
}

pub fn detect_recommended_codec() -> &'static str {
    detect_encoders().recommended_codec
}

pub fn canonical_codec_name(codec: &str) -> Result<&'static str> {
    match codec.trim() {
        "auto" | "" => Ok(detect_recommended_codec()),
        "h264_nvenc" | "nvenc" => Ok("h264_nvenc"),
        "hevc_nvenc" | "hevc" => Ok("hevc_nvenc"),
        "av1_nvenc" | "av1" => Ok("av1_nvenc"),
        "h264_amf" | "amf" => Ok("h264_amf"),
        "hevc_amf" => Ok("hevc_amf"),
        "h264_qsv" | "qsv" => Ok("h264_qsv"),
        "hevc_qsv" => Ok("hevc_qsv"),
        "libx264" | "x264" => Ok("libx264"),
        other => bail!("unsupported video encoder codec: {other}"),
    }
}

pub fn verify_gpu_codec_name(codec: &str) -> Result<&'static str> {
    let canonical = canonical_codec_name(codec)?;
    if !is_hardware_codec(canonical) {
        bail!(
            "--require-gpu rejected software codec '{canonical}': use a hardware encoder (*_nvenc, *_amf, *_qsv)"
        );
    }
    Ok(canonical)
}

fn map_nvenc_preset(preset: &str) -> &str {
    match preset {
        "p1" | "p2" | "p3" | "p4" | "p5" | "p6" | "p7" => preset,
        "ultrafast" | "superfast" | "veryfast" | "speed" => "p2",
        "faster" | "fast" => "p3",
        "medium" | "balanced" => "p4",
        "slow" => "p5",
        "slower" => "p6",
        "veryslow" | "quality" => "p7",
        _ => "p4",
    }
}

fn map_amf_quality(preset: &str) -> &'static str {
    match preset {
        "speed" | "fast" | "faster" | "veryfast" | "superfast" | "ultrafast" | "p1" | "p2" => {
            "speed"
        }
        "quality" | "slow" | "slower" | "veryslow" | "p6" | "p7" => "quality",
        _ => "balanced",
    }
}

fn map_qsv_preset(preset: &str) -> &'static str {
    match preset {
        "veryfast" | "superfast" | "ultrafast" | "p1" => "veryfast",
        "faster" | "p2" => "faster",
        "fast" | "speed" | "p3" => "fast",
        "slow" | "p6" => "slow",
        "slower" => "slower",
        "veryslow" | "quality" | "p7" => "veryslow",
        _ => "medium",
    }
}

fn map_x264_preset(preset: &str) -> &str {
    match preset {
        "p1" | "p2" | "speed" => "veryfast",
        "p3" | "p4" | "p5" | "p6" | "p7" => "fast",
        "balanced" => "medium",
        "quality" => "slow",
        other => other,
    }
}

pub fn build_video_encoder_args(
    resolved_codec: &str,
    preset: &str,
    cq: u32,
) -> Result<Vec<String>> {
    let cq_str = cq.to_string();
    let args = match resolved_codec {
        "h264_nvenc" | "hevc_nvenc" | "av1_nvenc" => vec![
            "-c:v".to_string(),
            resolved_codec.to_string(),
            "-preset".to_string(),
            map_nvenc_preset(preset).to_string(),
            "-rc".to_string(),
            "vbr".to_string(),
            "-b:v".to_string(),
            "0".to_string(),
            "-cq".to_string(),
            cq_str,
            "-pix_fmt".to_string(),
            "yuv420p".to_string(),
        ],
        "h264_amf" | "hevc_amf" => vec![
            "-c:v".to_string(),
            resolved_codec.to_string(),
            "-quality".to_string(),
            map_amf_quality(preset).to_string(),
            "-rc".to_string(),
            "cqp".to_string(),
            "-qp_i".to_string(),
            cq_str.clone(),
            "-qp_p".to_string(),
            cq_str,
            "-pix_fmt".to_string(),
            "yuv420p".to_string(),
        ],
        "h264_qsv" | "hevc_qsv" => vec![
            "-c:v".to_string(),
            resolved_codec.to_string(),
            "-preset".to_string(),
            map_qsv_preset(preset).to_string(),
            "-global_quality".to_string(),
            cq_str,
            "-pix_fmt".to_string(),
            "nv12".to_string(),
        ],
        "libx264" => vec![
            "-c:v".to_string(),
            "libx264".to_string(),
            "-preset".to_string(),
            map_x264_preset(preset).to_string(),
            "-crf".to_string(),
            cq_str,
            "-pix_fmt".to_string(),
            "yuv420p".to_string(),
        ],
        other => bail!("unsupported video encoder codec: {other}"),
    };
    Ok(args)
}

pub fn concat_videos(part_paths: &[std::path::PathBuf], output_path: &Path) -> Result<()> {
    if part_paths.is_empty() {
        bail!("cannot concatenate zero video segments");
    }
    if part_paths.len() == 1 {
        std::fs::copy(&part_paths[0], output_path)?;
        return Ok(());
    }
    let list_path = output_path.with_extension("concat.txt");
    let mut lines = String::new();
    for p in part_paths {
        let abs = std::fs::canonicalize(p).unwrap_or_else(|_| p.clone());
        let cleaned = abs
            .to_string_lossy()
            .trim_start_matches(r"\\?\")
            .replace('\\', "/")
            .replace('\'', "'\\''");
        lines.push_str(&format!("file '{cleaned}'\n"));
    }
    std::fs::write(&list_path, &lines)?;

    let out = Command::new("ffmpeg")
        .args([
            "-nostdin",
            "-y",
            "-v",
            "error",
            "-f",
            "concat",
            "-safe",
            "0",
            "-i",
        ])
        .arg(&list_path)
        .args(["-c", "copy", "-movflags", "+faststart"])
        .arg(output_path)
        .output()
        .context("failed executing ffmpeg concat")?;
    let _ = std::fs::remove_file(&list_path);
    if !out.status.success() {
        let stderr = String::from_utf8_lossy(&out.stderr);
        bail!("ffmpeg concat failed: {}", stderr.trim());
    }
    Ok(())
}

pub struct VideoEncoder {
    tx: Option<SyncSender<Vec<u8>>>,
    pool_rx: Receiver<Vec<u8>>,
    writer_thread: Option<JoinHandle<Result<()>>>,
    child: Option<Child>,
    frame_bytes: usize,
}

impl VideoEncoder {
    pub fn spawn(
        output_path: &Path,
        width: u32,
        height: u32,
        fps: u32,
        cfg: &EncoderConfig,
    ) -> Result<Self> {
        let mut cmd = Command::new("ffmpeg");
        cmd.args([
            "-nostdin",
            "-y",
            "-v",
            "error",
            "-f",
            "rawvideo",
            "-pix_fmt",
            "rgba",
            "-s",
            &format!("{width}x{height}"),
            "-r",
            &fps.to_string(),
            "-i",
            "pipe:0",
        ]);

        let resolved_codec = canonical_codec_name(&cfg.codec)?;
        let codec_args = build_video_encoder_args(resolved_codec, &cfg.preset, cfg.cq)?;
        cmd.args(&codec_args);

        cmd.args(["-movflags", "+faststart"])
            .arg(output_path)
            .stdin(Stdio::piped())
            .stdout(Stdio::null())
            .stderr(Stdio::piped());

        let mut child = cmd
            .spawn()
            .with_context(|| format!("failed spawning ffmpeg encoder ({resolved_codec})"))?;
        let mut stdin = child.stdin.take().context("ffmpeg encoder stdin unavailable")?;

        let frame_bytes = (width as usize) * (height as usize) * 4;
        let (tx, rx) = mpsc::sync_channel::<Vec<u8>>(4);
        let (pool_tx, pool_rx) = mpsc::sync_channel::<Vec<u8>>(6);
        for _ in 0..4 {
            let _ = pool_tx.send(vec![0u8; frame_bytes]);
        }

        let writer_thread = thread::spawn(move || -> Result<()> {
            while let Ok(frame) = rx.recv() {
                stdin
                    .write_all(&frame)
                    .context("failed writing raw frame to ffmpeg stdin")?;
                let _ = pool_tx.try_send(frame);
            }
            stdin.flush()?;
            Ok(())
        });

        Ok(Self {
            tx: Some(tx),
            pool_rx,
            writer_thread: Some(writer_thread),
            child: Some(child),
            frame_bytes,
        })
    }

    pub fn take_buffer(&self) -> Vec<u8> {
        self.pool_rx
            .try_recv()
            .unwrap_or_else(|_| vec![0u8; self.frame_bytes])
    }

    pub fn push_frame(&self, frame: Vec<u8>) -> Result<()> {
        self.tx
            .as_ref()
            .context("encoder already closed")?
            .send(frame)
            .context("encoder writer thread closed unexpectedly")
    }

    pub fn finish(mut self) -> Result<()> {
        drop(self.tx.take());
        if let Some(handle) = self.writer_thread.take() {
            handle.join().expect("encoder writer thread panicked")?;
        }
        if let Some(child) = self.child.take() {
            let output = child.wait_with_output()?;
            if !output.status.success() {
                let stderr = String::from_utf8_lossy(&output.stderr);
                bail!("ffmpeg encoder failed ({}): {}", output.status, stderr.trim());
            }
        }
        Ok(())
    }
}

pub fn build_audio_filter_complex(
    clips: &[AudioClipSpec],
    total_duration: f64,
    normalize_lufs: Option<f64>,
) -> String {
    let mut parts = Vec::with_capacity(clips.len() + 2);
    let mut labels = Vec::with_capacity(clips.len());

    for (idx, clip) in clips.iter().enumerate() {
        let input_idx = idx + 1;
        let mut chain = vec![
            "aresample=48000:first_pts=0".to_string(),
            "aformat=channel_layouts=stereo".to_string(),
        ];
        if let Some(dur) = clip.duration {
            chain.push(format!("atrim=start=0:duration={dur:.4}"));
            chain.push("asetpts=PTS-STARTPTS".to_string());
            if clip.fade_in > 0.0 {
                chain.push(format!("afade=t=in:st=0:d={:.4}", clip.fade_in));
            }
            if clip.fade_out > 0.0 && dur > clip.fade_out {
                let fo_start = (dur - clip.fade_out).max(0.0);
                chain.push(format!(
                    "afade=t=out:st={fo_start:.4}:d={:.4}",
                    clip.fade_out
                ));
            }
        } else if clip.fade_in > 0.0 {
            chain.push(format!("afade=t=in:st=0:d={:.4}", clip.fade_in));
        }

        chain.push(format!("volume={:.4}", clip.volume));
        let delay_ms = (clip.start.max(0.0) * 1000.0).round() as u64;
        if delay_ms > 0 {
            chain.push(format!("adelay={delay_ms}|{delay_ms}"));
        }
        let lbl = format!("a{idx}");
        parts.push(format!("[{input_idx}:a]{}[{lbl}]", chain.join(",")));
        labels.push(format!("[{lbl}]"));
    }

    let n = clips.len();
    let master_tail = if let Some(lufs) = normalize_lufs {
        format!("loudnorm=I={lufs:.1}:TP=-1.5:LRA=11")
    } else {
        "alimiter=limit=0.95".to_string()
    };
    parts.push(format!(
        "{}amix=inputs={n}:duration=longest:normalize=0:dropout_transition=0,apad=whole_dur={total_duration:.4},atrim=0:{total_duration:.4},{master_tail}[aout]",
        labels.join("")
    ));
    parts.join(";")
}

pub fn mux_audio_clips(
    video_only_path: &Path,
    final_output_path: &Path,
    clips: &[AudioClipSpec],
    total_duration: f64,
    normalize_lufs: Option<f64>,
    base_dir: &Path,
) -> Result<()> {
    if clips.is_empty() {
        std::fs::rename(video_only_path, final_output_path).or_else(|_| {
            std::fs::copy(video_only_path, final_output_path)?;
            std::fs::remove_file(video_only_path)
        })?;
        return Ok(());
    }

    let filter = build_audio_filter_complex(clips, total_duration, normalize_lufs);
    let mut cmd = Command::new("ffmpeg");
    cmd.args(["-nostdin", "-y", "-v", "error", "-i"])
        .arg(video_only_path);

    for clip in clips {
        let p = std::path::PathBuf::from(&clip.src);
        let path = if p.is_absolute() { p } else { base_dir.join(p) };
        if clip.media_start > 0.0 {
            cmd.args(["-ss", &format!("{:.4}", clip.media_start)]);
        }
        if let Some(dur) = clip.duration {
            cmd.args(["-t", &format!("{:.4}", dur + 0.1)]);
        }
        cmd.arg("-i").arg(&path);
    }

    cmd.args([
        "-filter_complex",
        &filter,
        "-map",
        "0:v:0",
        "-map",
        "[aout]",
        "-c:v",
        "copy",
        "-c:a",
        "aac",
        "-b:a",
        "192k",
        "-ar",
        "48000",
        "-ac",
        "2",
        "-shortest",
        "-movflags",
        "+faststart",
    ])
    .arg(final_output_path);

    let out = cmd.output().context("failed executing ffmpeg audio mux")?;
    if !out.status.success() {
        let stderr = String::from_utf8_lossy(&out.stderr);
        bail!("ffmpeg audio mux failed: {}", stderr.trim());
    }
    let _ = std::fs::remove_file(video_only_path);
    Ok(())
}

pub fn mux_audio(
    video_only_path: &Path,
    final_output_path: &Path,
    spec: &ProjectSpec,
    base_dir: &Path,
) -> Result<()> {
    mux_audio_clips(
        video_only_path,
        final_output_path,
        &spec.audio,
        spec.duration,
        spec.normalize_lufs,
        base_dir,
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn audio_filter_builds_cleanly() {
        let clips = vec![
            AudioClipSpec {
                src: "vo.wav".into(),
                start: 0.0,
                duration: Some(8.8),
                media_start: 38.058,
                volume: 1.0,
                fade_in: 0.0,
                fade_out: 0.0,
            },
            AudioClipSpec {
                src: "sfx.mp3".into(),
                start: 3.291,
                duration: Some(1.5),
                media_start: 0.0,
                volume: 0.42,
                fade_in: 0.04,
                fade_out: 0.1,
            },
        ];
        let f = build_audio_filter_complex(&clips, 8.805, Some(-14.0));
        assert!(f.contains("amix=inputs=2"));
        assert!(f.contains("adelay=3291|3291"));
        assert!(f.contains("afade=t=in:st=0:d=0.0400"));
        assert!(f.contains("loudnorm=I=-14.0:TP=-1.5:LRA=11"));
        assert!(f.contains("[aout]"));
    }

    #[test]
    fn parses_ffmpeg_hardware_encoders_cleanly() {
        let sample = "\
Encoders:
 V..... = Video
 ------
 V....D av1_nvenc            NVIDIA NVENC av1 encoder (codec av1)
 V..... av1_qsv              AV1 (Intel Quick Sync Video acceleration) (codec av1)
 V....D av1_amf              AMD AMF AV1 encoder (codec av1)
 V....D libx264              libx264 H.264 / AVC (codec h264)
 V....D h264_amf             AMD AMF H.264 Encoder (codec h264)
 V....D h264_nvenc           NVIDIA NVENC H.264 encoder (codec h264)
 V..... h264_qsv             H.264 / AVC (Intel Quick Sync) (codec h264)
 V....D hevc_amf             AMD AMF HEVC encoder (codec hevc)
 V....D hevc_nvenc           NVIDIA NVENC hevc encoder (codec hevc)
 V..... hevc_qsv             HEVC (Intel Quick Sync) (codec hevc)
 V..... mjpeg_qsv            MJPEG (Intel Quick Sync) (codec mjpeg)
 A..... aac                  AAC (Advanced Audio Coding)
";
        let found = parse_ffmpeg_hardware_encoders(sample);
        assert_eq!(
            found,
            vec![
                "av1_nvenc",
                "av1_qsv",
                "av1_amf",
                "h264_amf",
                "h264_nvenc",
                "h264_qsv",
                "hevc_amf",
                "hevc_nvenc",
                "hevc_qsv",
            ]
        );
    }

    #[test]
    fn encoder_cascade_follows_nvenc_amf_qsv_libx264_order() {
        let all: Vec<String> = ["h264_qsv", "h264_amf", "h264_nvenc"]
            .iter()
            .map(|s| (*s).to_string())
            .collect();

        assert_eq!(select_cascade_codec(&all, |_| true), "h264_nvenc");
        assert_eq!(
            select_cascade_codec(&all, |enc| enc == "h264_amf" || enc == "h264_qsv"),
            "h264_amf"
        );
        assert_eq!(
            select_cascade_codec(&all, |enc| enc == "h264_qsv"),
            "h264_qsv"
        );
        assert_eq!(select_cascade_codec(&all, |_| false), "libx264");
        assert_eq!(select_cascade_codec(&[], |_| true), "libx264");

        let mut probed = Vec::new();
        let only_qsv = vec!["h264_qsv".to_string()];
        let chosen = select_cascade_codec(&only_qsv, |enc| {
            probed.push(enc.to_string());
            true
        });
        assert_eq!(chosen, "h264_qsv");
        assert_eq!(probed, vec!["h264_qsv"]);
    }

    #[test]
    fn hardware_codec_verification_accepts_nvenc_amf_qsv_and_rejects_software() {
        for codec in [
            "h264_nvenc",
            "nvenc",
            "hevc_nvenc",
            "av1_nvenc",
            "h264_amf",
            "amf",
            "hevc_amf",
            "h264_qsv",
            "qsv",
            "hevc_qsv",
        ] {
            let resolved = verify_gpu_codec_name(codec).expect("hardware codec should pass");
            assert!(is_hardware_codec(resolved));
        }

        assert!(verify_gpu_codec_name("libx264").is_err());
        assert!(verify_gpu_codec_name("x264").is_err());
        assert!(canonical_codec_name("unknown_codec").is_err());
    }

    #[test]
    fn builds_encoder_args_for_nvenc_amf_qsv_and_libx264() {
        let nvenc = build_video_encoder_args("h264_nvenc", "p4", 19).unwrap();
        assert_eq!(
            nvenc,
            vec![
                "-c:v",
                "h264_nvenc",
                "-preset",
                "p4",
                "-rc",
                "vbr",
                "-b:v",
                "0",
                "-cq",
                "19",
                "-pix_fmt",
                "yuv420p"
            ]
        );

        let amf_h264 = build_video_encoder_args("h264_amf", "p4", 20).unwrap();
        assert_eq!(
            amf_h264,
            vec![
                "-c:v",
                "h264_amf",
                "-quality",
                "balanced",
                "-rc",
                "cqp",
                "-qp_i",
                "20",
                "-qp_p",
                "20",
                "-pix_fmt",
                "yuv420p"
            ]
        );

        let amf_hevc = build_video_encoder_args("hevc_amf", "p7", 22).unwrap();
        assert_eq!(
            amf_hevc,
            vec![
                "-c:v",
                "hevc_amf",
                "-quality",
                "quality",
                "-rc",
                "cqp",
                "-qp_i",
                "22",
                "-qp_p",
                "22",
                "-pix_fmt",
                "yuv420p"
            ]
        );

        let qsv_h264 = build_video_encoder_args("h264_qsv", "p4", 21).unwrap();
        assert_eq!(
            qsv_h264,
            vec![
                "-c:v",
                "h264_qsv",
                "-preset",
                "medium",
                "-global_quality",
                "21",
                "-pix_fmt",
                "nv12"
            ]
        );

        let qsv_hevc = build_video_encoder_args("hevc_qsv", "fast", 23).unwrap();
        assert_eq!(
            qsv_hevc,
            vec![
                "-c:v",
                "hevc_qsv",
                "-preset",
                "fast",
                "-global_quality",
                "23",
                "-pix_fmt",
                "nv12"
            ]
        );

        let x264 = build_video_encoder_args("libx264", "p4", 19).unwrap();
        assert_eq!(
            x264,
            vec![
                "-c:v",
                "libx264",
                "-preset",
                "fast",
                "-crf",
                "19",
                "-pix_fmt",
                "yuv420p"
            ]
        );
    }
}
