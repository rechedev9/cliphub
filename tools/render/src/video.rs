use crate::ffmpeg;
use crate::schema::VideoSegmentSpec;
use crate::timing::frame_count;
use anyhow::{Context, Result, bail};
use image::imageops::FilterType;
use std::collections::HashMap;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::process::{Child, Stdio};
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
    let cropped =
        image::imageops::crop_imm(&resized, crop_x, crop_y, target_w, target_h).to_image();
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
    let mut cmd = ffmpeg::command()?;
    cmd.args([
        "-nostdin",
        "-v",
        "error",
        "-ss",
        &ffmpeg::seek_argument(media_time),
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

const SEGMENT_START_SLACK: f64 = 0.001;
const SEGMENT_HOLD_SECONDS: f64 = 0.05;
const SOURCE_SHORTFALL_SECONDS: f64 = 0.25;
const DECODER_LOG_BYTES: usize = 4096;

pub fn active_segment(segments: &[VideoSegmentSpec], t: f64) -> Option<(usize, &VideoSegmentSpec)> {
    let find = |tail: f64| {
        segments.iter().enumerate().find(|(_, seg)| {
            t >= seg.start - SEGMENT_START_SLACK && t < seg.start + seg.duration + tail
        })
    };
    find(-SEGMENT_START_SLACK).or_else(|| find(SEGMENT_HOLD_SECONDS))
}

#[derive(Debug, Clone, Copy)]
pub struct FrameClock {
    pub t: f64,
    pub fps: u32,
}

#[derive(Debug, Clone, Copy)]
struct DecodeRequest<'a> {
    path: &'a Path,
    media_start: f64,
    duration: f64,
    playback_rate: f64,
    out_fps: u32,
    width: u32,
    height: u32,
}

pub struct VideoStreamDecoder {
    path: PathBuf,
    rx: Receiver<Vec<u8>>,
    recycle_tx: SyncSender<Vec<u8>>,
    last_frame: Option<Vec<u8>>,
    first_index: u64,
    decoded: u64,
    shortfall_frames: u64,
    child: Option<Child>,
    reader_thread: Option<JoinHandle<()>>,
    log_thread: Option<JoinHandle<String>>,
}

impl VideoStreamDecoder {
    fn spawn(request: DecodeRequest<'_>, first_index: u64, use_cuda_decode: bool) -> Result<Self> {
        if use_cuda_decode {
            let mut decoder = Self::spawn_inner(request, first_index, true)?;
            if let Ok(first) = decoder.rx.recv() {
                decoder.last_frame = Some(first);
                decoder.decoded = 1;
                return Ok(decoder);
            }
            drop(decoder);
        }
        Self::spawn_inner(request, first_index, false)
    }

    fn spawn_inner(request: DecodeRequest<'_>, first_index: u64, cuda: bool) -> Result<Self> {
        let w = request.width.max(2);
        let h = request.height.max(2);
        let out_fps = request.out_fps;
        let rate = request.playback_rate.max(0.01);
        let skipped = first_index as f64 / out_fps as f64;
        let duration = (request.duration - skipped).max(1.0 / out_fps as f64);
        let total_out_frames = ((duration * out_fps as f64).ceil() as u32).max(1) + 2;
        let media_dur = (duration * rate) + 1.0;

        let vf = format!(
            "setpts=(PTS-STARTPTS)/{rate:.6},fps={out_fps},scale={w}:{h}:force_original_aspect_ratio=increase:flags=bilinear,crop={w}:{h},format=rgba"
        );

        let mut cmd = ffmpeg::command()?;
        cmd.args(["-nostdin", "-v", "error"]);
        if cuda {
            cmd.args(["-hwaccel", "cuda"]);
        }
        cmd.args([
            "-ss",
            &ffmpeg::seek_argument(request.media_start + skipped * rate),
            "-t",
            &format!("{:.4}", media_dur),
            "-i",
        ])
        .arg(request.path)
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
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

        let mut child = cmd.spawn().with_context(|| {
            format!(
                "failed spawning ffmpeg stream decoder for {}",
                request.path.display()
            )
        })?;

        let mut stdout = child
            .stdout
            .take()
            .context("ffmpeg decoder stdout unavailable")?;
        let stderr = child
            .stderr
            .take()
            .context("ffmpeg decoder stderr unavailable")?;
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
        let log_thread = thread::spawn(move || read_tail(stderr));

        Ok(Self {
            path: request.path.to_path_buf(),
            rx,
            recycle_tx,
            last_frame: None,
            first_index,
            decoded: 0,
            shortfall_frames: (SOURCE_SHORTFALL_SECONDS * out_fps as f64).ceil() as u64,
            child: Some(child),
            reader_thread: Some(reader_thread),
            log_thread: Some(log_thread),
        })
    }

    fn frame_at(&mut self, index: u64) -> Result<&[u8]> {
        let wanted = index.saturating_sub(self.first_index);
        while self.decoded <= wanted {
            let Ok(frame) = self.rx.recv() else {
                self.check_end_of_stream(wanted)?;
                break;
            };
            if let Some(old) = self.last_frame.replace(frame) {
                let _ = self.recycle_tx.try_send(old);
            }
            self.decoded += 1;
        }
        self.last_frame
            .as_deref()
            .with_context(|| format!("video source produced no frames: {}", self.path.display()))
    }

    fn check_end_of_stream(&mut self, wanted: u64) -> Result<()> {
        let mut log = String::new();
        if let Some(mut child) = self.child.take() {
            let status = child.wait()?;
            if let Some(handle) = self.log_thread.take() {
                log = handle.join().unwrap_or_default();
            }
            if !status.success() {
                bail!(
                    "ffmpeg failed decoding {}: {}",
                    self.path.display(),
                    log.trim()
                );
            }
        }
        if self.decoded == 0 {
            bail!(
                "video source produced no frames: {} {}",
                self.path.display(),
                log.trim()
            );
        }
        if wanted - self.decoded >= self.shortfall_frames {
            bail!(
                "video source ends before its segment does: {}",
                self.path.display()
            );
        }
        Ok(())
    }
}

impl Drop for VideoStreamDecoder {
    fn drop(&mut self) {
        drop(std::mem::replace(&mut self.rx, mpsc::sync_channel(0).1));
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
        if let Some(handle) = self.reader_thread.take() {
            let _ = handle.join();
        }
        if let Some(handle) = self.log_thread.take() {
            let _ = handle.join();
        }
    }
}

fn read_tail(mut source: impl Read) -> String {
    let mut tail = Vec::new();
    let mut chunk = [0u8; 1024];
    while let Ok(read) = source.read(&mut chunk) {
        if read == 0 {
            break;
        }
        tail.extend_from_slice(&chunk[..read]);
        if tail.len() > DECODER_LOG_BYTES {
            tail.drain(..tail.len() - DECODER_LOG_BYTES);
        }
    }
    String::from_utf8_lossy(&tail).into_owned()
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

    pub fn video_frame(
        &mut self,
        stream_key: &str,
        path: &Path,
        segment: &VideoSegmentSpec,
        clock: FrameClock,
        (width, height): (u32, u32),
    ) -> Result<&[u8]> {
        let index = ((clock.t - segment.start).max(0.0) * clock.fps as f64).round() as u64;
        if !self.streams.contains_key(stream_key) {
            let request = DecodeRequest {
                path,
                media_start: segment.media_start,
                duration: segment.duration,
                playback_rate: segment.playback_rate,
                out_fps: clock.fps,
                width,
                height,
            };
            let last_index = u64::from(frame_count(segment.duration, clock.fps)) - 1;
            let decoder =
                VideoStreamDecoder::spawn(request, index.min(last_index), self.use_cuda_decode)?;
            self.streams.insert(stream_key.to_string(), decoder);
        }
        self.streams.get_mut(stream_key).unwrap().frame_at(index)
    }

    pub fn release_stream(&mut self, stream_key: &str) {
        self.streams.remove(stream_key);
    }
}

#[cfg(test)]
#[path = "video_tests.rs"]
mod tests;
