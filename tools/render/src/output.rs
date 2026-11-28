use anyhow::{Context, Result, bail};
use std::io::ErrorKind;
use std::path::{Path, PathBuf};

pub struct PendingOutput {
    final_path: PathBuf,
    work_dir: PathBuf,
}

impl PendingOutput {
    pub fn create(final_path: &Path) -> Result<Self> {
        ensure_new(final_path)?;
        let name = final_path
            .file_name()
            .with_context(|| format!("output path has no file name: {}", final_path.display()))?;
        if let Some(parent) = final_path.parent()
            && !parent.as_os_str().is_empty()
        {
            std::fs::create_dir_all(parent)?;
        }
        let mut work_name = name.to_os_string();
        work_name.push(format!(".{}.partial", std::process::id()));
        let work_dir = final_path.with_file_name(work_name);
        std::fs::create_dir(&work_dir)
            .with_context(|| format!("failed creating work directory {}", work_dir.display()))?;
        Ok(Self {
            final_path: final_path.to_path_buf(),
            work_dir,
        })
    }

    pub fn file(&self, name: &str) -> PathBuf {
        self.work_dir.join(name)
    }

    pub fn publish(self, finished: &Path) -> Result<()> {
        match std::fs::hard_link(finished, &self.final_path) {
            Ok(()) => Ok(()),
            Err(error) if error.kind() == ErrorKind::AlreadyExists => ensure_new(&self.final_path),
            Err(_) => {
                ensure_new(&self.final_path)?;
                std::fs::rename(finished, &self.final_path)
                    .with_context(|| format!("failed publishing {}", self.final_path.display()))
            }
        }
    }
}

impl Drop for PendingOutput {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.work_dir);
    }
}

fn ensure_new(path: &Path) -> Result<()> {
    if std::fs::symlink_metadata(path).is_ok() {
        bail!(
            "output already exists, choose a new file name: {}",
            path.display()
        );
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn scratch(name: &str) -> PathBuf {
        let dir = std::env::temp_dir().join(format!(
            "cliphub-render-output-{}-{name}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(&dir).unwrap();
        dir
    }

    #[test]
    fn existing_output_is_refused_and_left_untouched() {
        let dir = scratch("existing");
        let output = dir.join("video.mp4");
        std::fs::write(&output, b"keep me").unwrap();
        let error = PendingOutput::create(&output).err().unwrap();
        assert!(error.to_string().contains("already exists"));
        assert_eq!(std::fs::read(&output).unwrap(), b"keep me");
        assert_eq!(std::fs::read_dir(&dir).unwrap().count(), 1);
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn unpublished_work_leaves_nothing_at_the_output() {
        let dir = scratch("dropped");
        let output = dir.join("video.mp4");
        let pending = PendingOutput::create(&output).unwrap();
        let work = pending.file("render.mp4");
        assert_eq!(
            work.parent().unwrap(),
            dir.join(format!("video.mp4.{}.partial", std::process::id()))
        );
        std::fs::write(&work, b"half a video").unwrap();
        assert!(!output.exists());
        drop(pending);
        assert_eq!(std::fs::read_dir(&dir).unwrap().count(), 0);
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn publish_moves_the_finished_file_and_removes_the_work_directory() {
        let dir = scratch("published");
        let output = dir.join("video.mp4");
        let pending = PendingOutput::create(&output).unwrap();
        let work = pending.file("render.mp4");
        std::fs::write(&work, b"whole video").unwrap();
        pending.publish(&work).unwrap();
        assert_eq!(std::fs::read(&output).unwrap(), b"whole video");
        assert_eq!(std::fs::read_dir(&dir).unwrap().count(), 1);
        std::fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn publish_never_replaces_a_file_that_appeared_during_the_render() {
        let dir = scratch("raced");
        let output = dir.join("video.mp4");
        let pending = PendingOutput::create(&output).unwrap();
        let work = pending.file("render.mp4");
        std::fs::write(&work, b"mine").unwrap();
        std::fs::write(&output, b"theirs").unwrap();
        assert!(pending.publish(&work).is_err());
        assert_eq!(std::fs::read(&output).unwrap(), b"theirs");
        std::fs::remove_dir_all(&dir).unwrap();
    }
}
