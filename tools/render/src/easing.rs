use crate::schema::{CameraShake, CameraTween, EaseKind, EnterKind, ExitKind, LayerSpec, PulseSpec};
use std::f32::consts::PI;

pub fn ease(kind: EaseKind, t: f32) -> f32 {
    let t = t.clamp(0.0, 1.0);
    match kind {
        EaseKind::Linear => t,
        EaseKind::Power1InOut => {
            if t < 0.5 {
                2.0 * t * t
            } else {
                1.0 - (-2.0 * t + 2.0).powi(2) / 2.0
            }
        }
        EaseKind::Power2In => t * t,
        EaseKind::Power2Out => 1.0 - (1.0 - t).powi(2),
        EaseKind::Power3In => t * t * t,
        EaseKind::Power3Out => 1.0 - (1.0 - t).powi(3),
        EaseKind::Power3InOut => {
            if t < 0.5 {
                4.0 * t * t * t
            } else {
                1.0 - (-2.0 * t + 2.0).powi(3) / 2.0
            }
        }
        EaseKind::Power4In => t.powi(4),
        EaseKind::Power4Out => 1.0 - (1.0 - t).powi(4),
        EaseKind::BackOut => {
            // Matches GSAP back.out(2.2)
            let c1 = 2.2_f32;
            let c3 = c1 + 1.0;
            1.0 + c3 * (t - 1.0).powi(3) + c1 * (t - 1.0).powi(2)
        }
        EaseKind::ElasticOut => {
            if t <= 0.0 {
                0.0
            } else if t >= 1.0 {
                1.0
            } else {
                let p = 0.3_f32;
                2.0_f32.powf(-10.0 * t) * ((t - p / 4.0) * (2.0 * PI) / p).sin() + 1.0
            }
        }
        EaseKind::SineInOut => -(PI * t).cos() * 0.5 + 0.5,
    }
}

#[derive(Debug, Clone, Copy)]
pub struct AnimState {
    pub visible: bool,
    pub opacity: f32,
    pub dx: f32,
    pub dy: f32,
    pub scale: f32,
    pub rotation_deg: f32,
}

impl Default for AnimState {
    fn default() -> Self {
        Self {
            visible: false,
            opacity: 0.0,
            dx: 0.0,
            dy: 0.0,
            scale: 1.0,
            rotation_deg: 0.0,
        }
    }
}

pub fn default_enter_duration(kind: EnterKind) -> f64 {
    match kind {
        EnterKind::None => 0.0,
        EnterKind::Fade => 0.45,
        EnterKind::Up | EnterKind::Down | EnterKind::Left | EnterKind::Right => 0.50,
        EnterKind::Pop => 0.40,
        EnterKind::Slam => 0.30,
        EnterKind::Zoom => 0.55,
        EnterKind::Stamp => 0.20,
    }
}

pub fn default_exit_duration(kind: ExitKind) -> f64 {
    match kind {
        ExitKind::None => 0.0,
        ExitKind::Fade | ExitKind::Up => 0.30,
    }
}

pub fn eval_layer_anim(layer: &LayerSpec, t: f64) -> AnimState {
    if t < layer.start {
        return AnimState::default();
    }

    let mut opacity = 1.0_f32;
    let mut dx = 0.0_f32;
    let mut dy = 0.0_f32;
    let mut scale = 1.0_f32;
    let rotation_deg = layer.rotation_deg;

    // Enter phase
    let enter_dur = layer
        .enter_duration
        .unwrap_or_else(|| default_enter_duration(layer.enter));
    if enter_dur > 0.0 && layer.enter != EnterKind::None {
        let u = ((t - layer.start) / enter_dur).clamp(0.0, 1.0) as f32;
        match layer.enter {
            EnterKind::None => {}
            EnterKind::Fade => {
                opacity = ease(EaseKind::Power2Out, u);
            }
            EnterKind::Up => {
                let e = ease(EaseKind::Power3Out, u);
                opacity = e;
                dy = 36.0 * (1.0 - e);
            }
            EnterKind::Down => {
                let e = ease(EaseKind::Power3Out, u);
                opacity = e;
                dy = -60.0 * (1.0 - e);
            }
            EnterKind::Left => {
                let e = ease(EaseKind::Power3Out, u);
                opacity = e;
                dx = -70.0 * (1.0 - e);
            }
            EnterKind::Right => {
                let e = ease(EaseKind::Power3Out, u);
                opacity = e;
                dx = 70.0 * (1.0 - e);
            }
            EnterKind::Pop => {
                let e_scale = ease(EaseKind::BackOut, u);
                let e_op = ease(EaseKind::Power2Out, u);
                opacity = e_op;
                scale = 0.6 + 0.4 * e_scale;
            }
            EnterKind::Slam => {
                let e = ease(EaseKind::Power4Out, u);
                opacity = e;
                scale = 1.7 - 0.7 * e;
            }
            EnterKind::Zoom => {
                let e = ease(EaseKind::Power3Out, u);
                opacity = e;
                scale = 0.9 + 0.1 * e;
            }
            EnterKind::Stamp => {
                let e = ease(EaseKind::Power4In, u);
                opacity = 0.94 * e;
                scale = 2.2 - 1.2 * e;
            }
        }
    } else if layer.enter == EnterKind::Stamp {
        opacity = 0.94;
    }

    // Pulse / yoyo
    if let Some(pulse) = &layer.pulse {
        scale *= eval_pulse(pulse, t);
    }

    // Exit phase (starts at `end` for `exit_dur`, matching hf_lib.py `tl.to(..., exit_at)`)
    if let Some(end_at) = layer.end {
        let exit_dur = layer
            .exit_duration
            .unwrap_or_else(|| default_exit_duration(layer.exit));
        if layer.exit == ExitKind::None || exit_dur <= 0.0 {
            if t >= end_at {
                return AnimState::default();
            }
        } else if t >= end_at {
            let u = ((t - end_at) / exit_dur) as f32;
            if u >= 1.0 {
                return AnimState::default();
            }
            let e = ease(EaseKind::Power2In, u);
            opacity *= 1.0 - e;
            if layer.exit == ExitKind::Up {
                dy -= 30.0 * e;
            }
        }
    }

    if opacity <= 0.002 {
        return AnimState::default();
    }

    AnimState {
        visible: true,
        opacity: opacity.clamp(0.0, 1.0),
        dx,
        dy,
        scale,
        rotation_deg,
    }
}

fn eval_pulse(pulse: &PulseSpec, t: f64) -> f32 {
    if t < pulse.start || pulse.half_period <= 0.0 {
        return 1.0;
    }
    let elapsed = t - pulse.start;
    let total_cycles = (pulse.repeats + 1) as f64;
    let total_dur = total_cycles * pulse.half_period;
    if elapsed >= total_dur {
        return 1.0;
    }
    let cycle_idx = (elapsed / pulse.half_period).floor() as u32;
    let local_u = ((elapsed % pulse.half_period) / pulse.half_period) as f32;
    let e = ease(EaseKind::SineInOut, local_u);
    if cycle_idx.is_multiple_of(2) {
        1.0 + (pulse.scale - 1.0) * e
    } else {
        pulse.scale - (pulse.scale - 1.0) * e
    }
}

#[derive(Debug, Clone, Copy)]
pub struct CameraState {
    pub x: f32,
    pub y: f32,
    pub scale: f32,
}

impl Default for CameraState {
    fn default() -> Self {
        Self {
            x: 0.0,
            y: 0.0,
            scale: 1.0,
        }
    }
}

pub fn eval_camera(tweens: &[CameraTween], shakes: &[CameraShake], t: f64) -> CameraState {
    let mut cam = CameraState::default();
    for tw in tweens {
        if t < tw.start || tw.duration <= 0.0 {
            continue;
        }
        let u = ((t - tw.start) / tw.duration).clamp(0.0, 1.0) as f32;
        let e = ease(tw.ease, u);
        if let (Some(a), Some(b)) = (tw.from_x, tw.to_x) {
            cam.x = a + (b - a) * e;
        }
        if let (Some(a), Some(b)) = (tw.from_y, tw.to_y) {
            cam.y = a + (b - a) * e;
        }
        if let (Some(a), Some(b)) = (tw.from_scale, tw.to_scale) {
            cam.scale = a + (b - a) * e;
        }
    }
    for sh in shakes {
        if t >= sh.at && t < sh.at + sh.duration && sh.duration > 0.0 {
            let u = ((t - sh.at) / sh.duration) as f32;
            let e = ease(EaseKind::ElasticOut, u);
            cam.x += sh.amplitude_x * (1.0 - e);
            cam.y += sh.amplitude_y * (1.0 - e);
        }
    }
    cam
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::schema::LayerKind;

    #[test]
    fn easing_endpoints() {
        for k in [
            EaseKind::Linear,
            EaseKind::Power1InOut,
            EaseKind::Power2In,
            EaseKind::Power2Out,
            EaseKind::Power3In,
            EaseKind::Power3Out,
            EaseKind::Power3InOut,
            EaseKind::Power4In,
            EaseKind::Power4Out,
            EaseKind::BackOut,
            EaseKind::ElasticOut,
            EaseKind::SineInOut,
        ] {
            assert!((ease(k, 0.0) - 0.0).abs() < 1e-4, "{k:?} at 0");
            assert!((ease(k, 1.0) - 1.0).abs() < 1e-4, "{k:?} at 1");
        }
        assert!(ease(EaseKind::BackOut, 0.6) > 1.0);
    }

    #[test]
    fn layer_enter_and_exit() {
        let layer = LayerSpec {
            id: "test".into(),
            start: 1.0,
            end: Some(3.0),
            enter: EnterKind::Up,
            enter_duration: Some(0.5),
            exit: ExitKind::Fade,
            exit_duration: Some(0.3),
            rotation_deg: 0.0,
            pulse: None,
            fixed: false,
            kind: LayerKind::Legend {
                label: "9Z".into(),
                dot_color: "#F59E0B".into(),
                x: 0.0,
                y: 0.0,
            },
        };
        assert!(!eval_layer_anim(&layer, 0.5).visible);
        let mid = eval_layer_anim(&layer, 1.25);
        assert!(mid.visible && mid.opacity > 0.5 && mid.dy > 0.0);
        let steady = eval_layer_anim(&layer, 2.0);
        assert!((steady.opacity - 1.0).abs() < 1e-4 && steady.dy.abs() < 1e-4);
        assert!(!eval_layer_anim(&layer, 3.35).visible);
    }
}
