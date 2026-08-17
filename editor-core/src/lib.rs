use serde::{Deserialize, Serialize};

pub const SCHEMA_VERSION: &str = "1.0";
const DEFAULT_FONT_SIZE: i32 = 64;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Canvas {
    pub width: i32,
    pub height: i32,
    pub fps: i32,
}

#[derive(Debug, Clone, Deserialize, Serialize, Default)]
pub struct Transform {
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
    #[serde(default)]
    pub opacity: f64,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Item {
    pub id: String,
    pub asset_id: String,
    #[serde(default)]
    pub timeline_start: f64,
    pub source_in: f64,
    pub source_out: f64,
    #[serde(default)]
    pub speed: f64,
    #[serde(default)]
    pub fade_in: f64,
    #[serde(default)]
    pub fade_out: f64,
    #[serde(default)]
    pub transform: Option<Transform>,
    #[serde(default)]
    pub filter: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Track {
    pub id: String,
    pub kind: String,
    #[serde(default)]
    pub items: Vec<Item>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct TextOverlay {
    pub id: String,
    pub text: String,
    pub position_y: f64,
    pub start_seconds: f64,
    #[serde(default)]
    pub end_seconds: Option<f64>,
    #[serde(default)]
    pub font_size: i32,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Document {
    #[serde(default)]
    pub schema_version: String,
    pub canvas: Canvas,
    pub tracks: Vec<Track>,
    #[serde(default)]
    pub overlays: Vec<TextOverlay>,
}

#[derive(Debug, Clone, Serialize)]
pub struct Layer {
    pub item_id: String,
    pub track_id: String,
    pub asset_id: String,
    pub source_time: f64,
    pub transform: Transform,
    pub opacity: f64,
    pub filter: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct TextSample {
    pub id: String,
    pub text: String,
    pub position_y: f64,
    pub font_size: i32,
}

#[derive(Debug, Clone, Serialize)]
pub struct Sample {
    pub time: f64,
    pub duration: f64,
    pub layers: Vec<Layer>,
    pub texts: Vec<TextSample>,
}

impl Item {
    fn speed(&self) -> f64 {
        if self.speed == 0.0 {
            1.0
        } else {
            self.speed
        }
    }

    fn output_duration(&self) -> f64 {
        (self.source_out - self.source_in) / self.speed()
    }

    fn timeline_end(&self) -> f64 {
        self.timeline_start + self.output_duration()
    }

    fn resolved_transform(&self) -> Transform {
        match &self.transform {
            Some(tf) => Transform {
                x: tf.x,
                y: tf.y,
                width: tf.width,
                height: tf.height,
                opacity: if tf.opacity == 0.0 { 1.0 } else { tf.opacity },
            },
            None => Transform {
                x: 0.0,
                y: 0.0,
                width: 1.0,
                height: 1.0,
                opacity: 1.0,
            },
        }
    }
}

impl Document {
    pub fn duration(&self) -> f64 {
        self.tracks
            .iter()
            .flat_map(|track| track.items.iter())
            .map(Item::timeline_end)
            .fold(0.0, f64::max)
    }
}

pub fn evaluate(doc: &Document, t: f64) -> Sample {
    let duration = doc.duration();
    let mut sample = Sample {
        time: t,
        duration,
        layers: Vec::new(),
        texts: Vec::new(),
    };
    if t < 0.0 || t > duration {
        return sample;
    }
    for track in &doc.tracks {
        if track.kind != "video" {
            continue;
        }
        for item in &track.items {
            if t < item.timeline_start || t >= item.timeline_end() {
                continue;
            }
            let local = t - item.timeline_start;
            let base = item.resolved_transform();
            let opacity = base.opacity * fade_opacity(local, item.output_duration(), item.fade_in, item.fade_out);
            if opacity <= 0.0 {
                continue;
            }
            sample.layers.push(Layer {
                item_id: item.id.clone(),
                track_id: track.id.clone(),
                asset_id: item.asset_id.clone(),
                source_time: item.source_in + local * item.speed(),
                transform: Transform {
                    opacity,
                    ..base
                },
                opacity,
                filter: item.filter.clone(),
            });
        }
    }
    for overlay in &doc.overlays {
        let end = overlay.end_seconds.unwrap_or(duration);
        if t < overlay.start_seconds || t >= end {
            continue;
        }
        sample.texts.push(TextSample {
            id: overlay.id.clone(),
            text: overlay.text.clone(),
            position_y: overlay.position_y,
            font_size: if overlay.font_size == 0 {
                DEFAULT_FONT_SIZE
            } else {
                overlay.font_size
            },
        });
    }
    sample
}

pub fn evaluate_json(doc_json: &str, t: f64) -> Result<String, String> {
    let doc: Document = serde_json::from_str(doc_json).map_err(|err| err.to_string())?;
    serde_json::to_string(&evaluate(&doc, t)).map_err(|err| err.to_string())
}

fn fade_opacity(local: f64, duration: f64, fade_in: f64, fade_out: f64) -> f64 {
    let mut opacity = 1.0;
    if fade_in > 0.0 && local < fade_in {
        opacity = local / fade_in;
    }
    if fade_out > 0.0 && local > duration - fade_out {
        let tail = (duration - local) / fade_out;
        if tail < opacity {
            opacity = tail;
        }
    }
    opacity.clamp(0.0, 1.0)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn evaluate_table() {
        let doc = Document {
            schema_version: SCHEMA_VERSION.to_string(),
            canvas: Canvas {
                width: 1080,
                height: 1920,
                fps: 60,
            },
            tracks: vec![
                Track {
                    id: "v1".into(),
                    kind: "video".into(),
                    items: vec![Item {
                        id: "base".into(),
                        asset_id: "11111111-1111-1111-1111-111111111111".into(),
                        timeline_start: 0.0,
                        source_in: 1.0,
                        source_out: 5.0,
                        speed: 0.0,
                        fade_in: 0.5,
                        fade_out: 0.0,
                        transform: None,
                        filter: String::new(),
                    }],
                },
                Track {
                    id: "v2".into(),
                    kind: "video".into(),
                    items: vec![Item {
                        id: "pip".into(),
                        asset_id: "22222222-2222-2222-2222-222222222222".into(),
                        timeline_start: 1.0,
                        source_in: 0.0,
                        source_out: 1.0,
                        speed: 2.0,
                        fade_in: 0.0,
                        fade_out: 0.0,
                        transform: Some(Transform {
                            x: 0.6,
                            y: 0.05,
                            width: 0.35,
                            height: 0.25,
                            opacity: 0.8,
                        }),
                        filter: String::new(),
                    }],
                },
            ],
            overlays: vec![TextOverlay {
                id: "title".into(),
                text: "ACE".into(),
                position_y: 0.1,
                start_seconds: 0.2,
                end_seconds: Some(4.0),
                font_size: 72,
            }],
        };
        let fade = evaluate(&doc, 0.25);
        assert_eq!(fade.layers.len(), 1);
        assert!((fade.layers[0].opacity - 0.5).abs() < 1e-9);
        assert!((fade.layers[0].source_time - 1.25).abs() < 1e-9);
        let stacked = evaluate(&doc, 1.1);
        assert_eq!(stacked.layers.len(), 2);
        assert_eq!(stacked.texts.len(), 1);
        let after = evaluate(&doc, 1.6);
        assert_eq!(after.layers.len(), 1);
        assert_eq!(after.layers[0].item_id, "base");
    }
}
