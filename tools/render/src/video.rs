use anyhow::{Context, Result, bail};
use image::imageops::FilterType;
use std::collections::HashMap;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::mpsc::{self, Receiver, SyncSender};
use std::thread::{self, JoinHandle};

pub struct DecodedFrame {
    pub rgba: Vec<u8>,
}

/// Cover-fit an image to exact (target_w, target_h) RGBA pixels (matching CSS `object-fit: cover`).
pub fn load_image_cover(path: &Path, target_w: u32, target_h: u32) -> Result<DecodedFrame> {
    let img = image::open(path)
        .with_context(|| format!("failed opening image {}", path.display()))?
        .to_rgba8();
    let (src_w, src_h) = img.dimensions();
    if src_w == target_w && src_h == target_h {
        return Ok(DecodedFrame {
            rgba: img.into_raw(),
        });
    }
    let scale = (target_w as f64 / src_w as f64).max(target_h as f64 / src_h as f64);
    let scaled_w = ((src_w as f64 * scale).round() as u32).max(target_w);
    let scaled_h = ((src_h as f64 * scale).round() as u32).max(target_h);
    let resized = image::imageops::resize(&img, scaled_w, scaled_h, FilterType::Lanczos3);
    let crop_x = (scaled_w - target_w) / 2;
    let crop_y = (scaled_h - target_h) / 2;
    let cropped = image::imageops::crop_imm(&resized, crop_x, crop_y, target_w, target_h).to_image();
    Ok(DecodedFrame {
        rgba: cropped.into_raw(),
    })
}

/// Single-frame fast seek decoder for instant `snapshot` mode.
pub fn decode_single_frame(
    path: &Path,
    media_time: f64,
    width: u32,
    height: u32,
) -> Result<DecodedFrame> {
    let w = width.max(2);
    let h = height.max(2);
    let vf = format!(
        "scale={w}:{h}:force_original_aspect_ratio=increase:flags=bilinear,crop={w}:{h},format=rgba"
    );
    let mut cmd = Command::new("ffmpeg");
    cmd.args([
        "-nostdin",
        "-v",
        "error",
        "-ss",
        &format!("{:.4}", media_time.max(0.0)),
        "-i",
    ])
    .arg(path)
    .args([
        "-frames:v",
        "1",
        "-vf",
        &vf,
        "-f",
        "rawvideo",
        "-pix_fmt",
        "rgba",
        "pipe:1",
    ])
    .stdout(Stdio::piped())
    .stderr(Stdio::piped());

    let out = cmd.output().with_context(|| {
        format!(
            "failed running ffmpeg single-frame decode for {}",
            path.display()
        )
    })?;
    let expected = (w as usize) * (h as usize) * 4;
    if out.stdout.len() < expected {
        let stderr = String::from_utf8_lossy(&out.stderr);
        bail!(
            "ffmpeg returned {} bytes (expected {}) for {} at {:.3}s: {}",
            out.stdout.len(),
            expected,
            path.display(),
            media_time,
            stderr.trim()
        );
    }
    let mut rgba = out.stdout;
    rgba.truncate(expected);
    Ok(DecodedFrame { rgba })
}

/// Background streaming FFmpeg video decoder with recycled frame buffers and optional CUDA hwaccel.
pub struct VideoStreamDecoder {
    rx: Receiver<Vec<u8>>,
    recycle_tx: SyncSender<Vec<u8>>,
    last_frame: Option<Vec<u8>>,
    child: Option<Child>,
    reader_thread: Option<JoinHandle<()>>,
    pub hwaccel_used: bool,
}

impl VideoStreamDecoder {
    #[allow(clippy::too_many_arguments)]
    pub fn spawn(
        path: &Path,
        media_start: f64,
        duration: f64,
        playback_rate: f64,
        out_fps: u32,
        width: u32,
        height: u32,
        use_cuda_decode: bool,
    ) -> Result<Self> {
        if use_cuda_decode {
            let mut dec = Self::spawn_inner(
                path,
                media_start,
                duration,
                playback_rate,
                out_fps,
                width,
                height,
                true,
            )?;
            // Prime first frame to verify CUDA decode works on this stream; fallback cleanly if not.
            if let Ok(first) = dec.rx.recv() {
                dec.last_frame = Some(first);
                return Ok(dec);
            }
            drop(dec);
        }
        Self::spawn_inner(
            path,
            media_start,
            duration,
            playback_rate,
            out_fps,
            width,
            height,
            false,
        )
    }

    #[allow(clippy::too_many_arguments)]
    fn spawn_inner(
        path: &Path,
        media_start: f64,
        duration: f64,
        playback_rate: f64,
        out_fps: u32,
        width: u32,
        height: u32,
        cuda: bool,
    ) -> Result<Self> {
        let w = width.max(2);
        let h = height.max(2);
        let rate = playback_rate.max(0.01);
        let total_out_frames = ((duration * out_fps as f64).ceil() as u32).max(1) + 2;
        let media_dur = (duration * rate) + 1.0;

        let vf = format!(
            "setpts=(PTS-STARTPTS)/{rate:.6},fps={out_fps},scale={w}:{h}:force_original_aspect_ratio=increase:flags=bilinear,crop={w}:{h},format=rgba"
        );

        let mut cmd = Command::new("ffmpeg");
        cmd.args(["-nostdin", "-v", "error"]);
        if cuda {
            cmd.args(["-hwaccel", "cuda"]);
        }
        cmd.args([
            "-ss",
            &format!("{:.4}", media_start.max(0.0)),
            "-t",
            &format!("{:.4}", media_dur),
            "-i",
        ])
        .arg(path)
        .args([
            "-vf",
            &vf,
            "-frames:v",
            &total_out_frames.to_string(),
            "-f",
            "rawvideo",
            "-pix_fmt",
            "rgba",
            "pipe:1",
        ])
        .stdout(Stdio::piped())
        .stderr(Stdio::null());

        let mut child = cmd.spawn().with_context(|| {
            format!("failed spawning ffmpeg stream decoder for {}", path.display())
        })?;

        let mut stdout = child
            .stdout
            .take()
            .context("ffmpeg decoder stdout unavailable")?;
        let frame_bytes = (w as usize) * (h as usize) * 4;
        let (tx, rx) = mpsc::sync_channel::<Vec<u8>>(10);
        let (recycle_tx, recycle_rx) = mpsc::sync_channel::<Vec<u8>>(12);
        for _ in 0..8 {
            let _ = recycle_tx.try_send(vec![0u8; frame_bytes]);
        }

        let reader_thread = thread::spawn(move || {
            loop {
                let mut buf = recycle_rx
                    .try_recv()
                    .unwrap_or_else(|_| vec![0u8; frame_bytes]);
                if buf.len() != frame_bytes {
                    buf.resize(frame_bytes, 0);
                }
                let mut filled = 0;
                while filled < frame_bytes {
                    match stdout.read(&mut buf[filled..]) {
                        Ok(0) => return,
                        Ok(n) => filled += n,
                        Err(_) => return,
                    }
                }
                if tx.send(buf).is_err() {
                    return;
                }
            }
        });

        Ok(Self {
            rx,
            recycle_tx,
            last_frame: None,
            child: Some(child),
            reader_thread: Some(reader_thread),
            hwaccel_used: cuda,
        })
    }

    pub fn next_frame(&mut self) -> Option<&[u8]> {
        // If primed by CUDA check on frame 0, return that frame once before pulling next.
        if self.hwaccel_used && self.last_frame.is_some() {
            self.hwaccel_used = false;
            return self.last_frame.as_deref();
        }
        if let Ok(frame) = self.rx.recv()
            && let Some(old) = self.last_frame.replace(frame)
        {
            let _ = self.recycle_tx.try_send(old);
        }
        self.last_frame.as_deref()
    }
}

impl Drop for VideoStreamDecoder {
    fn drop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
        if let Some(handle) = self.reader_thread.take() {
            let _ = handle.join();
        }
    }
}

/// Manages decoded static images and lazy video stream decoders across a timeline.
pub struct MediaPool {
    images: HashMap<(PathBuf, u32, u32), DecodedFrame>,
    streams: HashMap<String, VideoStreamDecoder>,
    pub use_cuda_decode: bool,
}

impl MediaPool {
    pub fn new(use_cuda_decode: bool) -> Self {
        Self {
            images: HashMap::new(),
            streams: HashMap::new(),
            use_cuda_decode,
        }
    }

    pub fn get_image(&mut self, path: &Path, w: u32, h: u32) -> Result<&DecodedFrame> {
        let key = (path.to_path_buf(), w, h);
        if !self.images.contains_key(&key) {
            let decoded = load_image_cover(path, w, h)?;
            self.images.insert(key.clone(), decoded);
        }
        Ok(self.images.get(&key).unwrap())
    }

    #[allow(clippy::too_many_arguments)]
    pub fn next_video_frame(
        &mut self,
        stream_key: &str,
        path: &Path,
        media_start: f64,
        duration: f64,
        playback_rate: f64,
        out_fps: u32,
        w: u32,
        h: u32,
    ) -> Result<Option<&[u8]>> {
        if !self.streams.contains_key(stream_key) {
            let dec = VideoStreamDecoder::spawn(
                path,
                media_start,
                duration,
                playback_rate,
                out_fps,
                w,
                h,
                self.use_cuda_decode,
            )?;
            self.streams.insert(stream_key.to_string(), dec);
        }
        Ok(self.streams.get_mut(stream_key).unwrap().next_frame())
    }

    pub fn release_stream(&mut self, stream_key: &str) {
        self.streams.remove(stream_key);
    }
}
