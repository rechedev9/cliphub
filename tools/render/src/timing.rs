use crate::schema::EssaySpec;

const FRAME_SLACK: f64 = 1e-6;

impl EssaySpec {
    pub fn scene_frame_counts(&self) -> Vec<u32> {
        scene_frame_counts(self.scenes.iter().map(|scene| scene.duration), self.fps)
    }
}

pub fn frame_count(duration: f64, fps: u32) -> u32 {
    (duration * fps as f64 - FRAME_SLACK).ceil().max(1.0) as u32
}

fn scene_frame_counts(durations: impl IntoIterator<Item = f64>, fps: u32) -> Vec<u32> {
    let mut elapsed = 0.0;
    let mut emitted = 0;
    durations
        .into_iter()
        .map(|duration| {
            elapsed += duration;
            let boundary = frame_count(elapsed, fps).max(emitted + 1);
            let frames = boundary - emitted;
            emitted = boundary;
            frames
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn whole_frame_durations_do_not_gain_a_frame_from_rounding_noise() {
        assert_eq!(frame_count(1.1, 30), 33);
        assert_eq!(frame_count(31.0 / 30.0, 30), 31);
        assert_eq!(frame_count(1.01, 30), 31);
    }

    #[test]
    fn scene_boundaries_never_drift_more_than_one_frame() {
        let durations = [1.01; 10];
        let counts = scene_frame_counts(durations, 30);
        let mut elapsed = 0.0;
        let mut emitted = 0;
        for (duration, frames) in durations.iter().zip(&counts) {
            elapsed += duration;
            emitted += frames;
            let drift = emitted as f64 - elapsed * 30.0;
            assert!((0.0..1.0).contains(&(drift + FRAME_SLACK)), "drift {drift}");
        }
        assert_eq!(emitted, frame_count(elapsed, 30));
    }
}
