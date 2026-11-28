use anyhow::{Result, bail};
use std::ffi::OsString;
use std::path::PathBuf;
use std::process::Command;

pub const FFMPEG_ENV: &str = "ZV_FFMPEG_PATH";

const SEEK_SLACK_SECONDS: f64 = 0.0005;

fn resolve(configured: Option<OsString>) -> Result<PathBuf> {
    let Some(configured) = configured.filter(|value| !value.is_empty()) else {
        return Ok(PathBuf::from("ffmpeg"));
    };
    let path = PathBuf::from(configured);
    if !path.is_file() {
        bail!("{FFMPEG_ENV} does not point to a file: {}", path.display());
    }
    Ok(path)
}

pub fn command() -> Result<Command> {
    Ok(Command::new(resolve(std::env::var_os(FFMPEG_ENV))?))
}

pub fn seek_argument(media_time: f64) -> String {
    format!("{:.6}", (media_time - SEEK_SLACK_SECONDS).max(0.0))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn configured_ffmpeg_wins_over_path() {
        let exe = std::env::current_exe().unwrap();
        assert_eq!(resolve(Some(exe.clone().into_os_string())).unwrap(), exe);
    }

    #[test]
    fn missing_configured_ffmpeg_never_falls_back_to_path() {
        let missing = std::env::temp_dir().join("cliphub-no-such-ffmpeg.exe");
        let error = resolve(Some(missing.into_os_string())).unwrap_err();
        assert!(error.to_string().contains(FFMPEG_ENV));
    }

    #[test]
    fn unconfigured_ffmpeg_uses_path() {
        assert_eq!(resolve(None).unwrap(), PathBuf::from("ffmpeg"));
        assert_eq!(
            resolve(Some(OsString::new())).unwrap(),
            PathBuf::from("ffmpeg")
        );
    }

    #[test]
    fn seek_never_lands_after_the_requested_frame() {
        let frame_two = 2.0 / 30.0;
        assert!(seek_argument(frame_two).parse::<f64>().unwrap() < frame_two);
        assert_eq!(seek_argument(0.0), "0.000000");
    }
}
