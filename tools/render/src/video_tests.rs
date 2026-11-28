use super::*;
use std::time::Duration;

const SIDE: u32 = 64;
const FPS: u32 = 30;
const STEP: u32 = 8;

fn scratch(name: &str) -> PathBuf {
    let dir = std::env::temp_dir().join(format!(
        "cliphub-render-video-{}-{name}",
        std::process::id()
    ));
    let _ = std::fs::remove_dir_all(&dir);
    std::fs::create_dir_all(&dir).unwrap();
    dir
}

fn numbered_video(dir: &Path, frames: u32) -> PathBuf {
    let path = dir.join("numbered.mkv");
    let status = ffmpeg::command()
        .unwrap()
        .args(["-nostdin", "-y", "-v", "error", "-f", "lavfi", "-i"])
        .arg(format!(
            "color=c=black:s={SIDE}x{SIDE}:r={FPS},format=gray,geq=lum='N*{STEP}'"
        ))
        .args(["-frames:v", &frames.to_string(), "-c:v", "ffv1"])
        .arg(&path)
        .status()
        .unwrap();
    assert!(status.success());
    path
}

fn segment(media_start: f64, duration: f64) -> VideoSegmentSpec {
    VideoSegmentSpec {
        src: String::new(),
        start: 0.0,
        duration,
        media_start,
        playback_rate: 1.0,
    }
}

fn shown_frame(
    pool: &mut MediaPool,
    path: &Path,
    seg: &VideoSegmentSpec,
    index: u32,
) -> Result<u32> {
    let clock = FrameClock {
        t: index as f64 / FPS as f64,
        fps: FPS,
    };
    let rgba = pool.video_frame("layer#seg0", path, seg, clock, (SIDE, SIDE))?;
    Ok((rgba[0] as f64 / STEP as f64).round() as u32)
}

#[test]
fn a_decoder_that_starts_late_shows_the_frame_of_its_timeline_position() {
    let dir = scratch("late");
    let path = numbered_video(&dir, 30);
    let seg = segment(0.0, 1.0);
    let mut pool = MediaPool::new(false);
    for index in [2, 3, 4, 9, 20] {
        assert_eq!(shown_frame(&mut pool, &path, &seg, index).unwrap(), index);
    }
    drop(pool);
    std::fs::remove_dir_all(&dir).unwrap();
}

#[test]
fn media_start_offsets_every_frame() {
    let dir = scratch("offset");
    let path = numbered_video(&dir, 30);
    let seg = segment(5.0 / FPS as f64, 0.5);
    let mut pool = MediaPool::new(false);
    for index in [0, 1, 7] {
        assert_eq!(
            shown_frame(&mut pool, &path, &seg, index).unwrap(),
            index + 5
        );
    }
    drop(pool);
    std::fs::remove_dir_all(&dir).unwrap();
}

#[test]
fn dropping_a_decoder_with_unread_frames_returns() {
    let dir = scratch("drop");
    let path = numbered_video(&dir, 30);
    let seg = segment(0.0, 10.0);
    let (done_tx, done_rx) = mpsc::channel();
    let source = path.clone();
    thread::spawn(move || {
        let mut pool = MediaPool::new(false);
        shown_frame(&mut pool, &source, &seg, 0).unwrap();
        thread::sleep(Duration::from_millis(500));
        drop(pool);
        done_tx.send(()).unwrap();
    });
    done_rx
        .recv_timeout(Duration::from_secs(20))
        .expect("dropping the decoder deadlocked");
    std::fs::remove_dir_all(&dir).unwrap();
}

#[test]
fn a_source_without_frames_fails_the_render() {
    let dir = scratch("junk");
    let junk = dir.join("junk.mp4");
    std::fs::write(&junk, b"not a video").unwrap();
    let mut pool = MediaPool::new(false);
    assert!(shown_frame(&mut pool, &junk, &segment(0.0, 1.0), 0).is_err());

    let short = numbered_video(&dir, 15);
    pool.release_stream("layer#seg0");
    assert!(shown_frame(&mut pool, &short, &segment(100.0, 1.0), 0).is_err());
    drop(pool);
    std::fs::remove_dir_all(&dir).unwrap();
}

#[test]
fn a_source_shorter_than_its_segment_fails_beyond_a_quarter_second() {
    let dir = scratch("short");
    let path = numbered_video(&dir, 15);
    let seg = segment(0.0, 2.0);
    let mut pool = MediaPool::new(false);
    for index in 0..=20 {
        assert_eq!(
            shown_frame(&mut pool, &path, &seg, index).unwrap(),
            index.min(14)
        );
    }
    assert!(shown_frame(&mut pool, &path, &seg, 30).is_err());
    drop(pool);
    std::fs::remove_dir_all(&dir).unwrap();
}

#[test]
fn a_decoder_that_starts_in_the_hold_tail_still_shows_a_frame() {
    let dir = scratch("tail");
    let path = numbered_video(&dir, 15);
    let seg = segment(0.0, 0.5);
    let mut pool = MediaPool::new(false);
    assert_eq!(shown_frame(&mut pool, &path, &seg, 15).unwrap(), 14);
    drop(pool);
    std::fs::remove_dir_all(&dir).unwrap();
}

#[test]
fn contiguous_segments_switch_on_their_boundary() {
    let segments = vec![segment(0.0, 1.0), {
        let mut next = segment(0.0, 1.0);
        next.start = 1.0;
        next
    }];
    let at = |frame: u32| active_segment(&segments, frame as f64 / FPS as f64).map(|(i, _)| i);
    assert_eq!(at(29), Some(0));
    assert_eq!(at(30), Some(1));
    assert_eq!(at(60), Some(1));
    assert_eq!(at(62), None);
}
