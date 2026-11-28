use crate::easing::{AnimState, CameraState, ease, eval_camera, eval_layer_anim};
use crate::schema::{
    FontRole, LayerKind, LayerSpec, ProjectSpec, Rgba, TagSpec, TextAlign, ValueKeyframe,
};
use crate::text::{FontRegistry, draw_text_block, measure_line_width};
use crate::video::{FrameClock, MediaPool, active_segment, decode_single_frame};
use anyhow::{Context, Result, anyhow};
use bytemuck::{Pod, Zeroable};
use std::collections::HashMap;
use std::path::Path;
use vello::kurbo::{Affine, Arc as KurboArc, BezPath, Cap, Circle, Line as KurboLine, Point, Rect, RoundedRect, Stroke, Vec2};
use vello::peniko::{Color, Fill, Gradient};
use vello::{AaConfig, AaSupport, RenderParams, Renderer, RendererOptions, Scene, wgpu};

const SHADERS_WGSL: &str = r#"
struct BgUniforms {
    canvas: vec2<f32>,
    desk_enabled: f32,
    mood_strength: f32,
    bg_color: vec4<f32>,
    mood_color: vec4<f32>,
};

@group(0) @binding(0) var<uniform> bg_u: BgUniforms;

struct VsOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
};

@vertex
fn vs_fullscreen(@builtin(vertex_index) idx: u32) -> VsOut {
    var pts = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -3.0),
        vec2<f32>(-1.0,  1.0),
        vec2<f32>( 3.0,  1.0),
    );
    let p = pts[idx];
    var out: VsOut;
    out.pos = vec4<f32>(p, 0.0, 1.0);
    out.uv = vec2<f32>(p.x * 0.5 + 0.5, 0.5 - p.y * 0.5);
    return out;
}

@fragment
fn fs_bg(in: VsOut) -> @location(0) vec4<f32> {
    var col = bg_u.bg_color.rgb;
    if (bg_u.desk_enabled > 0.5) {
        let d = (in.uv - vec2<f32>(0.50, 0.30)) / vec2<f32>(0.78, 0.72);
        let dist = clamp(length(d), 0.0, 1.0);
        let glow = vec3<f32>(0.082, 0.110, 0.168); // #151C2B
        col = mix(glow, col, smoothstep(0.0, 1.0, dist));
        if (bg_u.mood_strength > 0.001) {
            let md = (in.uv - vec2<f32>(0.50, 0.34)) / vec2<f32>(0.75, 0.65);
            let mfac = (1.0 - smoothstep(0.0, 1.0, clamp(length(md), 0.0, 1.0))) * bg_u.mood_strength;
            col = mix(col, bg_u.mood_color.rgb, mfac);
        }
        let px = in.uv * bg_u.canvas;
        // Subtle 64px architectural blueprint grid
        let gx = px.x - floor(px.x / 64.0) * 64.0;
        let gy = px.y - floor(px.y / 64.0) * 64.0;
        if (gx < 1.0 || gy < 1.0) {
            col = mix(col, vec3<f32>(1.0, 1.0, 1.0), 0.025);
        }
    }
    return vec4<f32>(col, 1.0);
}

struct BoxUniforms {
    canvas: vec2<f32>,
    box_pos: vec2<f32>,
    box_size: vec2<f32>,
    radius: f32,
    border_width: f32,
    opacity: f32,
    shadow_enabled: f32,
    zoom: f32,
    contrast: f32,
    border_color: vec4<f32>,
};

@group(0) @binding(0) var<uniform> box_u: BoxUniforms;
@group(0) @binding(1) var box_tex: texture_2d<f32>;
@group(0) @binding(2) var box_samp: sampler;

fn sd_rounded_rect(p: vec2<f32>, half_size: vec2<f32>, r: f32) -> f32 {
    let q = abs(p) - half_size + vec2<f32>(r, r);
    return length(max(q, vec2<f32>(0.0))) + min(max(q.x, q.y), 0.0) - r;
}

@fragment
fn fs_video_box(in: VsOut) -> @location(0) vec4<f32> {
    let px = in.uv * box_u.canvas;
    let half = box_u.box_size * 0.5;
    let center = box_u.box_pos + half;
    let p = px - center;
    let r = min(box_u.radius, min(half.x, half.y));
    let d = sd_rounded_rect(p, half, r);

    var out_col = vec4<f32>(0.0, 0.0, 0.0, 0.0);

    // Soft drop shadow outside box
    if (box_u.shadow_enabled > 0.5 && d > -2.0) {
        let sp = p - vec2<f32>(0.0, 26.0);
        let sd = sd_rounded_rect(sp, half, r);
        let shadow_a = exp(-max(sd, 0.0) * 0.032) * 0.58 * box_u.opacity;
        out_col = vec4<f32>(0.0, 0.0, 0.0, shadow_a);
    }

    // Inside rounded box (with 1.2px AA feather)
    let mask = 1.0 - smoothstep(-1.0, 0.8, d);
    if (mask > 0.001) {
        let raw_uv = (px - box_u.box_pos) / box_u.box_size;
        let z = max(box_u.zoom, 0.1);
        let local_uv = clamp((raw_uv - vec2<f32>(0.5, 0.5)) / z + vec2<f32>(0.5, 0.5), vec2<f32>(0.0), vec2<f32>(1.0));
        var sample_col = textureSample(box_tex, box_samp, local_uv);
        if (abs(box_u.contrast - 1.0) > 0.005) {
            let lifted = (sample_col.rgb - vec3<f32>(0.45)) * box_u.contrast + vec3<f32>(0.48);
            sample_col = vec4<f32>(clamp(lifted, vec3<f32>(0.0), vec3<f32>(1.0)), 1.0);
        } else {
            sample_col.a = 1.0;
        }

        // Inner border stroke
        if (box_u.border_width > 0.0) {
            let border_t = smoothstep(-box_u.border_width - 0.8, -box_u.border_width + 0.8, d);
            let b_rgb = mix(sample_col.rgb, box_u.border_color.rgb, box_u.border_color.a * border_t);
            sample_col = vec4<f32>(b_rgb, 1.0);
        }

        let box_alpha = mask * box_u.opacity;
        // SrcOver box over shadow
        let final_a = box_alpha + out_col.a * (1.0 - box_alpha);
        if (final_a > 0.0001) {
            let final_rgb = (sample_col.rgb * box_alpha + out_col.rgb * out_col.a * (1.0 - box_alpha)) / final_a;
            out_col = vec4<f32>(final_rgb, final_a);
        }
    }

    return out_col;
}

struct CompositeUniforms {
    vignette: f32,
    _pad0: f32,
    _pad1: f32,
    _pad2: f32,
};

@group(0) @binding(0) var<uniform> comp_u: CompositeUniforms;
@group(0) @binding(1) var base_tex: texture_2d<f32>;
@group(0) @binding(2) var overlay_tex: texture_2d<f32>;
@group(0) @binding(3) var comp_samp: sampler;

@fragment
fn fs_composite(in: VsOut) -> @location(0) vec4<f32> {
    let base = textureSample(base_tex, comp_samp, in.uv);
    let ov = textureSample(overlay_tex, comp_samp, in.uv);
    // Vello writes straight (unpremultiplied) RGBA into overlay_tex
    let rgb = mix(base.rgb, ov.rgb, ov.a);

    // Vignette
    var final_rgb = rgb;
    if (comp_u.vignette > 0.001) {
        let d = length((in.uv - vec2<f32>(0.5, 0.5)) * 2.0);
        let vig = smoothstep(0.68, 1.42, d) * comp_u.vignette;
        final_rgb = final_rgb * (1.0 - vig);
    }

    return vec4<f32>(clamp(final_rgb, vec3<f32>(0.0), vec3<f32>(1.0)), 1.0);
}
"#;

#[repr(C)]
#[derive(Copy, Clone, Pod, Zeroable)]
struct BgUniforms {
    canvas: [f32; 2],
    desk_enabled: f32,
    mood_strength: f32,
    bg_color: [f32; 4],
    mood_color: [f32; 4],
}

#[repr(C)]
#[derive(Copy, Clone, Pod, Zeroable)]
struct BoxUniforms {
    canvas: [f32; 2],
    box_pos: [f32; 2],
    box_size: [f32; 2],
    radius: f32,
    border_width: f32,
    opacity: f32,
    shadow_enabled: f32,
    zoom: f32,
    contrast: f32,
    border_color: [f32; 4],
}

#[repr(C)]
#[derive(Copy, Clone, Pod, Zeroable)]
struct CompositeUniforms {
    vignette: f32,
    _pad: [f32; 3],
}

struct CachedTexture {
    width: u32,
    height: u32,
    texture: wgpu::Texture,
    bind_group: wgpu::BindGroup,
    uniform_buf: wgpu::Buffer,
    static_uploaded: bool,
}

pub struct GpuInfo {
    pub adapter_name: String,
    pub backend: String,
    pub device_type: String,
    pub is_discrete_gpu: bool,
}

struct PendingReadback {
    slot: usize,
    rx: std::sync::mpsc::Receiver<Result<(), wgpu::BufferAsyncError>>,
}

pub struct GpuCompositor {
    pub info: GpuInfo,
    width: u32,
    height: u32,
    device: wgpu::Device,
    queue: wgpu::Queue,
    vello: Renderer,
    scene: Scene,

    // Textures
    _base_tex: wgpu::Texture,
    base_view: wgpu::TextureView,
    _vello_tex: wgpu::Texture,
    vello_view: wgpu::TextureView,
    final_tex: wgpu::Texture,
    final_view: wgpu::TextureView,
    readback_bufs: [wgpu::Buffer; 2],
    next_slot: usize,
    pending: Option<PendingReadback>,
    padded_bytes_per_row: u32,

    // Pipelines & bindings
    bg_pipeline: wgpu::RenderPipeline,
    bg_uniform_buf: wgpu::Buffer,
    bg_bind_group: wgpu::BindGroup,

    box_pipeline: wgpu::RenderPipeline,
    box_bgl: wgpu::BindGroupLayout,
    sampler: wgpu::Sampler,
    media_textures: HashMap<String, CachedTexture>,

    comp_pipeline: wgpu::RenderPipeline,
    comp_uniform_buf: wgpu::Buffer,
    comp_bind_group: wgpu::BindGroup,
}

pub fn probe_gpu() -> Result<GpuInfo> {
    pollster::block_on(async {
        let instance = wgpu::Instance::new(wgpu::InstanceDescriptor::new_without_display_handle());
        let adapter = instance
            .request_adapter(&wgpu::RequestAdapterOptions {
                power_preference: wgpu::PowerPreference::HighPerformance,
                compatible_surface: None,
                force_fallback_adapter: false,
                ..Default::default()
            })
            .await
            .map_err(|e| anyhow!("no compatible GPU adapter found: {e}"))?;
        let ai = adapter.get_info();
        Ok(GpuInfo {
            adapter_name: ai.name,
            backend: format!("{:?}", ai.backend),
            device_type: format!("{:?}", ai.device_type),
            is_discrete_gpu: ai.device_type == wgpu::DeviceType::DiscreteGpu,
        })
    })
}

impl GpuCompositor {
    pub fn new(width: u32, height: u32, require_discrete_gpu: bool) -> Result<Self> {
        pollster::block_on(Self::new_async(width, height, require_discrete_gpu))
    }

    async fn new_async(width: u32, height: u32, require_discrete_gpu: bool) -> Result<Self> {
        let instance = wgpu::Instance::new(wgpu::InstanceDescriptor::new_without_display_handle());
        let adapter = instance
            .request_adapter(&wgpu::RequestAdapterOptions {
                power_preference: wgpu::PowerPreference::HighPerformance,
                compatible_surface: None,
                force_fallback_adapter: false,
                ..Default::default()
            })
            .await
            .map_err(|e| anyhow!("failed requesting high-performance GPU adapter: {e}"))?;

        let ai = adapter.get_info();
        let is_discrete_gpu = ai.device_type == wgpu::DeviceType::DiscreteGpu;
        if require_discrete_gpu && !is_discrete_gpu {
            anyhow::bail!(
                "--require-gpu rejected adapter '{}' ({:?} on {:?}): expected DiscreteGpu",
                ai.name,
                ai.device_type,
                ai.backend
            );
        }
        let info = GpuInfo {
            adapter_name: ai.name,
            backend: format!("{:?}", ai.backend),
            device_type: format!("{:?}", ai.device_type),
            is_discrete_gpu,
        };

        let (device, queue) = adapter
            .request_device(&wgpu::DeviceDescriptor {
                label: Some("cliphub-render GPU"),
                required_features: wgpu::Features::empty(),
                required_limits: wgpu::Limits::default(),
                ..Default::default()
            })
            .await
            .map_err(|e| anyhow!("failed creating wgpu device: {e}"))?;

        let vello = Renderer::new(
            &device,
            RendererOptions {
                use_cpu: false,
                antialiasing_support: AaSupport::area_only(),
                num_init_threads: None,
                pipeline_cache: None,
            },
        )
        .map_err(|e| anyhow!("failed initializing Vello renderer: {e}"))?;

        let tex_size = wgpu::Extent3d {
            width,
            height,
            depth_or_array_layers: 1,
        };

        let base_tex = device.create_texture(&wgpu::TextureDescriptor {
            label: Some("base_canvas"),
            size: tex_size,
            mip_level_count: 1,
            sample_count: 1,
            dimension: wgpu::TextureDimension::D2,
            format: wgpu::TextureFormat::Rgba8Unorm,
            usage: wgpu::TextureUsages::RENDER_ATTACHMENT | wgpu::TextureUsages::TEXTURE_BINDING,
            view_formats: &[],
        });
        let base_view = base_tex.create_view(&wgpu::TextureViewDescriptor::default());

        let vello_tex = device.create_texture(&wgpu::TextureDescriptor {
            label: Some("vello_overlay"),
            size: tex_size,
            mip_level_count: 1,
            sample_count: 1,
            dimension: wgpu::TextureDimension::D2,
            format: wgpu::TextureFormat::Rgba8Unorm,
            usage: wgpu::TextureUsages::STORAGE_BINDING
                | wgpu::TextureUsages::TEXTURE_BINDING
                | wgpu::TextureUsages::RENDER_ATTACHMENT,
            view_formats: &[],
        });
        let vello_view = vello_tex.create_view(&wgpu::TextureViewDescriptor::default());

        let final_tex = device.create_texture(&wgpu::TextureDescriptor {
            label: Some("final_frame"),
            size: tex_size,
            mip_level_count: 1,
            sample_count: 1,
            dimension: wgpu::TextureDimension::D2,
            format: wgpu::TextureFormat::Rgba8Unorm,
            usage: wgpu::TextureUsages::RENDER_ATTACHMENT | wgpu::TextureUsages::COPY_SRC,
            view_formats: &[],
        });
        let final_view = final_tex.create_view(&wgpu::TextureViewDescriptor::default());

        let unpadded = width * 4;
        let align = wgpu::COPY_BYTES_PER_ROW_ALIGNMENT;
        let padded_bytes_per_row = unpadded.div_ceil(align) * align;
        let rb_size = (padded_bytes_per_row as u64) * (height as u64);
        let readback_bufs = [
            device.create_buffer(&wgpu::BufferDescriptor {
                label: Some("frame_readback_0"),
                size: rb_size,
                usage: wgpu::BufferUsages::COPY_DST | wgpu::BufferUsages::MAP_READ,
                mapped_at_creation: false,
            }),
            device.create_buffer(&wgpu::BufferDescriptor {
                label: Some("frame_readback_1"),
                size: rb_size,
                usage: wgpu::BufferUsages::COPY_DST | wgpu::BufferUsages::MAP_READ,
                mapped_at_creation: false,
            }),
        ];

        let shader = device.create_shader_module(wgpu::ShaderModuleDescriptor {
            label: Some("compositor_wgsl"),
            source: wgpu::ShaderSource::Wgsl(SHADERS_WGSL.into()),
        });

        let sampler = device.create_sampler(&wgpu::SamplerDescriptor {
            label: Some("linear_sampler"),
            mag_filter: wgpu::FilterMode::Linear,
            min_filter: wgpu::FilterMode::Linear,
            address_mode_u: wgpu::AddressMode::ClampToEdge,
            address_mode_v: wgpu::AddressMode::ClampToEdge,
            ..Default::default()
        });

        // 1. Background pipeline
        let bg_bgl = device.create_bind_group_layout(&wgpu::BindGroupLayoutDescriptor {
            label: Some("bg_bgl"),
            entries: &[wgpu::BindGroupLayoutEntry {
                binding: 0,
                visibility: wgpu::ShaderStages::FRAGMENT,
                ty: wgpu::BindingType::Buffer {
                    ty: wgpu::BufferBindingType::Uniform,
                    has_dynamic_offset: false,
                    min_binding_size: None,
                },
                count: None,
            }],
        });
        let bg_uniform_buf = device.create_buffer(&wgpu::BufferDescriptor {
            label: Some("bg_u"),
            size: std::mem::size_of::<BgUniforms>() as u64,
            usage: wgpu::BufferUsages::UNIFORM | wgpu::BufferUsages::COPY_DST,
            mapped_at_creation: false,
        });
        let bg_bind_group = device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("bg_bg"),
            layout: &bg_bgl,
            entries: &[wgpu::BindGroupEntry {
                binding: 0,
                resource: bg_uniform_buf.as_entire_binding(),
            }],
        });
        let bg_layout = device.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
            label: Some("bg_layout"),
            bind_group_layouts: &[Some(&bg_bgl)],
            immediate_size: 0,
        });
        let bg_pipeline = device.create_render_pipeline(&wgpu::RenderPipelineDescriptor {
            label: Some("bg_pipeline"),
            layout: Some(&bg_layout),
            vertex: wgpu::VertexState {
                module: &shader,
                entry_point: Some("vs_fullscreen"),
                buffers: &[],
                compilation_options: Default::default(),
            },
            fragment: Some(wgpu::FragmentState {
                module: &shader,
                entry_point: Some("fs_bg"),
                targets: &[Some(wgpu::ColorTargetState {
                    format: wgpu::TextureFormat::Rgba8Unorm,
                    blend: None,
                    write_mask: wgpu::ColorWrites::ALL,
                })],
                compilation_options: Default::default(),
            }),
            primitive: wgpu::PrimitiveState::default(),
            depth_stencil: None,
            multisample: wgpu::MultisampleState::default(),
            multiview_mask: None,
            cache: None,
        });

        // 2. Video/Image rounded-box pipeline
        let box_bgl = device.create_bind_group_layout(&wgpu::BindGroupLayoutDescriptor {
            label: Some("box_bgl"),
            entries: &[
                wgpu::BindGroupLayoutEntry {
                    binding: 0,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Buffer {
                        ty: wgpu::BufferBindingType::Uniform,
                        has_dynamic_offset: false,
                        min_binding_size: None,
                    },
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 1,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Texture {
                        sample_type: wgpu::TextureSampleType::Float { filterable: true },
                        view_dimension: wgpu::TextureViewDimension::D2,
                        multisampled: false,
                    },
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 2,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Sampler(wgpu::SamplerBindingType::Filtering),
                    count: None,
                },
            ],
        });
        let box_layout = device.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
            label: Some("box_layout"),
            bind_group_layouts: &[Some(&box_bgl)],
            immediate_size: 0,
        });
        let box_pipeline = device.create_render_pipeline(&wgpu::RenderPipelineDescriptor {
            label: Some("box_pipeline"),
            layout: Some(&box_layout),
            vertex: wgpu::VertexState {
                module: &shader,
                entry_point: Some("vs_fullscreen"),
                buffers: &[],
                compilation_options: Default::default(),
            },
            fragment: Some(wgpu::FragmentState {
                module: &shader,
                entry_point: Some("fs_video_box"),
                targets: &[Some(wgpu::ColorTargetState {
                    format: wgpu::TextureFormat::Rgba8Unorm,
                    blend: Some(wgpu::BlendState::ALPHA_BLENDING),
                    write_mask: wgpu::ColorWrites::ALL,
                })],
                compilation_options: Default::default(),
            }),
            primitive: wgpu::PrimitiveState::default(),
            depth_stencil: None,
            multisample: wgpu::MultisampleState::default(),
            multiview_mask: None,
            cache: None,
        });

        // 3. Final composite + vignette pipeline
        let comp_bgl = device.create_bind_group_layout(&wgpu::BindGroupLayoutDescriptor {
            label: Some("comp_bgl"),
            entries: &[
                wgpu::BindGroupLayoutEntry {
                    binding: 0,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Buffer {
                        ty: wgpu::BufferBindingType::Uniform,
                        has_dynamic_offset: false,
                        min_binding_size: None,
                    },
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 1,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Texture {
                        sample_type: wgpu::TextureSampleType::Float { filterable: true },
                        view_dimension: wgpu::TextureViewDimension::D2,
                        multisampled: false,
                    },
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 2,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Texture {
                        sample_type: wgpu::TextureSampleType::Float { filterable: true },
                        view_dimension: wgpu::TextureViewDimension::D2,
                        multisampled: false,
                    },
                    count: None,
                },
                wgpu::BindGroupLayoutEntry {
                    binding: 3,
                    visibility: wgpu::ShaderStages::FRAGMENT,
                    ty: wgpu::BindingType::Sampler(wgpu::SamplerBindingType::Filtering),
                    count: None,
                },
            ],
        });
        let comp_uniform_buf = device.create_buffer(&wgpu::BufferDescriptor {
            label: Some("comp_u"),
            size: std::mem::size_of::<CompositeUniforms>() as u64,
            usage: wgpu::BufferUsages::UNIFORM | wgpu::BufferUsages::COPY_DST,
            mapped_at_creation: false,
        });
        let comp_bind_group = device.create_bind_group(&wgpu::BindGroupDescriptor {
            label: Some("comp_bg"),
            layout: &comp_bgl,
            entries: &[
                wgpu::BindGroupEntry {
                    binding: 0,
                    resource: comp_uniform_buf.as_entire_binding(),
                },
                wgpu::BindGroupEntry {
                    binding: 1,
                    resource: wgpu::BindingResource::TextureView(&base_view),
                },
                wgpu::BindGroupEntry {
                    binding: 2,
                    resource: wgpu::BindingResource::TextureView(&vello_view),
                },
                wgpu::BindGroupEntry {
                    binding: 3,
                    resource: wgpu::BindingResource::Sampler(&sampler),
                },
            ],
        });
        let comp_layout = device.create_pipeline_layout(&wgpu::PipelineLayoutDescriptor {
            label: Some("comp_layout"),
            bind_group_layouts: &[Some(&comp_bgl)],
            immediate_size: 0,
        });
        let comp_pipeline = device.create_render_pipeline(&wgpu::RenderPipelineDescriptor {
            label: Some("comp_pipeline"),
            layout: Some(&comp_layout),
            vertex: wgpu::VertexState {
                module: &shader,
                entry_point: Some("vs_fullscreen"),
                buffers: &[],
                compilation_options: Default::default(),
            },
            fragment: Some(wgpu::FragmentState {
                module: &shader,
                entry_point: Some("fs_composite"),
                targets: &[Some(wgpu::ColorTargetState {
                    format: wgpu::TextureFormat::Rgba8Unorm,
                    blend: None,
                    write_mask: wgpu::ColorWrites::ALL,
                })],
                compilation_options: Default::default(),
            }),
            primitive: wgpu::PrimitiveState::default(),
            depth_stencil: None,
            multisample: wgpu::MultisampleState::default(),
            multiview_mask: None,
            cache: None,
        });

        Ok(Self {
            info,
            width,
            height,
            device,
            queue,
            vello,
            scene: Scene::new(),
            _base_tex: base_tex,
            base_view,
            _vello_tex: vello_tex,
            vello_view,
            final_tex,
            final_view,
            readback_bufs,
            next_slot: 0,
            pending: None,
            padded_bytes_per_row,
            bg_pipeline,
            bg_uniform_buf,
            bg_bind_group,
            box_pipeline,
            box_bgl,
            sampler,
            media_textures: HashMap::new(),
            comp_pipeline,
            comp_uniform_buf,
            comp_bind_group,
        })
    }

    pub fn clear_media_cache(&mut self) {
        self.media_textures.clear();
    }

    fn ensure_media_texture(&mut self, key: &str, w: u32, h: u32) {
        let needs_create = match self.media_textures.get(key) {
            Some(c) => c.width != w || c.height != h,
            None => true,
        };
        if needs_create {
            let texture = self.device.create_texture(&wgpu::TextureDescriptor {
                label: Some(key),
                size: wgpu::Extent3d {
                    width: w,
                    height: h,
                    depth_or_array_layers: 1,
                },
                mip_level_count: 1,
                sample_count: 1,
                dimension: wgpu::TextureDimension::D2,
                format: wgpu::TextureFormat::Rgba8Unorm,
                usage: wgpu::TextureUsages::TEXTURE_BINDING | wgpu::TextureUsages::COPY_DST,
                view_formats: &[],
            });
            let view = texture.create_view(&wgpu::TextureViewDescriptor::default());
            let uniform_buf = self.device.create_buffer(&wgpu::BufferDescriptor {
                label: Some("box_u"),
                size: std::mem::size_of::<BoxUniforms>() as u64,
                usage: wgpu::BufferUsages::UNIFORM | wgpu::BufferUsages::COPY_DST,
                mapped_at_creation: false,
            });
            let bind_group = self.device.create_bind_group(&wgpu::BindGroupDescriptor {
                label: Some("box_bg"),
                layout: &self.box_bgl,
                entries: &[
                    wgpu::BindGroupEntry {
                        binding: 0,
                        resource: uniform_buf.as_entire_binding(),
                    },
                    wgpu::BindGroupEntry {
                        binding: 1,
                        resource: wgpu::BindingResource::TextureView(&view),
                    },
                    wgpu::BindGroupEntry {
                        binding: 2,
                        resource: wgpu::BindingResource::Sampler(&self.sampler),
                    },
                ],
            });
            self.media_textures.insert(
                key.to_string(),
                CachedTexture {
                    width: w,
                    height: h,
                    texture,
                    bind_group,
                    uniform_buf,
                    static_uploaded: false,
                },
            );
        }
    }

    /// Render a single frame synchronously (used by `snapshot` mode).
    #[allow(clippy::too_many_arguments)]
    pub fn render_frame_sync(
        &mut self,
        spec: &ProjectSpec,
        base_dir: &Path,
        fonts: &FontRegistry,
        media: &mut MediaPool,
        t: f64,
        snapshot_mode: bool,
        out_rgba: &mut Vec<u8>,
    ) -> Result<()> {
        let _ = self.submit_and_collect(
            spec,
            base_dir,
            fonts,
            media,
            t,
            snapshot_mode,
            out_rgba,
        )?;
        self.flush_pending(out_rgba)?;
        Ok(())
    }

    /// Ping-pong pipelined GPU frame submission:
    /// Submits frame `t` to slot `S` and schedules non-blocking `map_async`, then while the GPU
    /// renders slot `S`, collects the previous frame from slot `1-S` into `out_rgba`.
    #[allow(clippy::too_many_arguments)]
    pub fn submit_and_collect(
        &mut self,
        spec: &ProjectSpec,
        base_dir: &Path,
        fonts: &FontRegistry,
        media: &mut MediaPool,
        t: f64,
        snapshot_mode: bool,
        out_rgba: &mut Vec<u8>,
    ) -> Result<bool> {
        let cam = eval_camera(&spec.camera, &spec.camera_shakes, t);
        let bg_rgba = Rgba::parse(&spec.background)?;

        // Update background uniform
        let (mood_color, mood_strength) = match spec.bg_mood.as_deref() {
            Some("red") => ([0.843, 0.149, 0.239, 1.0], 0.16),
            Some("cyan") => ([0.0, 0.831, 1.0, 1.0], 0.13),
            Some("gold") => ([0.980, 0.800, 0.082, 1.0], 0.12),
            Some(hex) => (Rgba::parse(hex)?.to_f32_array(), 0.14),
            None => ([0.0, 0.0, 0.0, 0.0], 0.0),
        };
        let bg_u = BgUniforms {
            canvas: [self.width as f32, self.height as f32],
            desk_enabled: if spec.desk_pattern { 1.0 } else { 0.0 },
            mood_strength,
            bg_color: bg_rgba.to_f32_array(),
            mood_color,
        };
        self.queue
            .write_buffer(&self.bg_uniform_buf, 0, bytemuck::bytes_of(&bg_u));

        // Gather active video/image boxes to draw in Pass 1 (and upload their pixels)
        let mut active_media_keys: Vec<String> = Vec::new();
        for layer in &spec.layers {
            let anim = eval_layer_anim(layer, t);
            if !anim.visible {
                // Release finished video decoders eagerly
                if let LayerKind::VideoBox { segments, .. } = &layer.kind
                    && let Some(end) = layer.end
                    && t > end + 0.5
                {
                    for (si, _) in segments.iter().enumerate() {
                        media.release_stream(&format!("{}#seg{si}", layer.id));
                    }
                }
                continue;
            }
            match &layer.kind {
                LayerKind::ImageBox {
                    src,
                    x,
                    y,
                    width,
                    height,
                    radius,
                    border_width,
                    border_color,
                    shadow,
                    zoom,
                    contrast,
                    ..
                } => {
                    let iw = (*width as u32).max(2);
                    let ih = (*height as u32).max(2);
                    let key = format!("img:{}", layer.id);
                    let path = spec.resolve_asset(base_dir, src);
                    let need_upload = self
                        .media_textures
                        .get(&key)
                        .map(|c| !c.static_uploaded)
                        .unwrap_or(true);
                    if need_upload {
                        let frame = media.get_image(&path, iw, ih)?;
                        self.ensure_media_texture(&key, iw, ih);
                        let entry = self.media_textures.get_mut(&key).unwrap();
                        self.queue.write_texture(
                            wgpu::TexelCopyTextureInfo {
                                texture: &entry.texture,
                                mip_level: 0,
                                origin: wgpu::Origin3d::ZERO,
                                aspect: wgpu::TextureAspect::All,
                            },
                            &frame.rgba,
                            wgpu::TexelCopyBufferLayout {
                                offset: 0,
                                bytes_per_row: Some(iw * 4),
                                rows_per_image: Some(ih),
                            },
                            wgpu::Extent3d {
                                width: iw,
                                height: ih,
                                depth_or_array_layers: 1,
                            },
                        );
                        entry.static_uploaded = true;
                    }
                    let eff_cam = if layer.fixed {
                        CameraState::default()
                    } else {
                        cam
                    };
                    let (bx, by, bw, bh) = apply_box_transform(
                        *x,
                        *y,
                        *width,
                        *height,
                        anim,
                        eff_cam,
                        self.width as f32,
                        self.height as f32,
                    );
                    let bc = Rgba::parse(border_color)?.to_f32_array();
                    let u = BoxUniforms {
                        canvas: [self.width as f32, self.height as f32],
                        box_pos: [bx, by],
                        box_size: [bw, bh],
                        radius: *radius * anim.scale * eff_cam.scale,
                        border_width: *border_width * anim.scale * eff_cam.scale,
                        opacity: anim.opacity,
                        shadow_enabled: if *shadow { 1.0 } else { 0.0 },
                        zoom: *zoom,
                        contrast: *contrast,
                        border_color: bc,
                    };
                    let entry = self.media_textures.get(&key).unwrap();
                    self.queue
                        .write_buffer(&entry.uniform_buf, 0, bytemuck::bytes_of(&u));
                    active_media_keys.push(key);
                }
                LayerKind::VideoBox {
                    x,
                    y,
                    width,
                    height,
                    radius,
                    border_width,
                    border_color,
                    shadow,
                    zoom,
                    contrast,
                    segments,
                    ..
                } => {
                    let chosen = active_segment(segments, t);
                    for (si, seg) in segments.iter().enumerate() {
                        if t > seg.start + seg.duration + 0.25 {
                            media.release_stream(&format!("{}#seg{si}", layer.id));
                        }
                    }
                    let Some((si, seg)) = chosen else {
                        continue;
                    };

                    let vw = ((*width as u32).max(2) / 2) * 2;
                    let vh = ((*height as u32).max(2) / 2) * 2;
                    let key = format!("vid:{}", layer.id);
                    let path = spec.resolve_asset(base_dir, &seg.src);

                    let pixels_cow: Option<std::borrow::Cow<'_, [u8]>> = if snapshot_mode {
                        let local_t = (t - seg.start).max(0.0);
                        let media_t = seg.media_start + local_t * seg.playback_rate;
                        Some(std::borrow::Cow::Owned(
                            decode_single_frame(&path, media_t, vw, vh)?.rgba,
                        ))
                    } else {
                        let stream_key = format!("{}#seg{si}", layer.id);
                        Some(std::borrow::Cow::Borrowed(media.video_frame(
                            &stream_key,
                            &path,
                            seg,
                            FrameClock { t, fps: spec.fps },
                            (vw, vh),
                        )?))
                    };

                    if let Some(rgba) = pixels_cow {
                        self.ensure_media_texture(&key, vw, vh);
                        let entry = self.media_textures.get(&key).unwrap();
                        self.queue.write_texture(
                            wgpu::TexelCopyTextureInfo {
                                texture: &entry.texture,
                                mip_level: 0,
                                origin: wgpu::Origin3d::ZERO,
                                aspect: wgpu::TextureAspect::All,
                            },
                            &rgba,
                            wgpu::TexelCopyBufferLayout {
                                offset: 0,
                                bytes_per_row: Some(vw * 4),
                                rows_per_image: Some(vh),
                            },
                            wgpu::Extent3d {
                                width: vw,
                                height: vh,
                                depth_or_array_layers: 1,
                            },
                        );
                        let eff_cam = if layer.fixed {
                            CameraState::default()
                        } else {
                            cam
                        };
                        let (bx, by, bw, bh) = apply_box_transform(
                            *x,
                            *y,
                            *width,
                            *height,
                            anim,
                            eff_cam,
                            self.width as f32,
                            self.height as f32,
                        );
                        let bc = Rgba::parse(border_color)?.to_f32_array();
                        let u = BoxUniforms {
                            canvas: [self.width as f32, self.height as f32],
                            box_pos: [bx, by],
                            box_size: [bw, bh],
                            radius: *radius * anim.scale * eff_cam.scale,
                            border_width: *border_width * anim.scale * eff_cam.scale,
                            opacity: anim.opacity,
                            shadow_enabled: if *shadow { 1.0 } else { 0.0 },
                            zoom: *zoom,
                            contrast: *contrast,
                            border_color: bc,
                        };
                        self.queue
                            .write_buffer(&entry.uniform_buf, 0, bytemuck::bytes_of(&u));
                        active_media_keys.push(key);
                    }
                }
                _ => {}
            }
        }

        // Pass 1: Render background + all video/image boxes to base_tex
        let mut encoder = self
            .device
            .create_command_encoder(&wgpu::CommandEncoderDescriptor {
                label: Some("frame_encoder"),
            });
        {
            let mut rpass = encoder.begin_render_pass(&wgpu::RenderPassDescriptor {
                label: Some("base_pass"),
                color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                    view: &self.base_view,
                    resolve_target: None,
                    ops: wgpu::Operations {
                        load: wgpu::LoadOp::Clear(wgpu::Color::BLACK),
                        store: wgpu::StoreOp::Store,
                    },
                    depth_slice: None,
                })],
                depth_stencil_attachment: None,
                timestamp_writes: None,
                occlusion_query_set: None,
                multiview_mask: None,
            });
            rpass.set_pipeline(&self.bg_pipeline);
            rpass.set_bind_group(0, &self.bg_bind_group, &[]);
            rpass.draw(0..3, 0..1);

            rpass.set_pipeline(&self.box_pipeline);
            for key in &active_media_keys {
                if let Some(entry) = self.media_textures.get(key) {
                    rpass.set_bind_group(0, &entry.bind_group, &[]);
                    rpass.draw(0..3, 0..1);
                }
            }
        }
        self.queue.submit(Some(encoder.finish()));

        // Pass 2: Build Vello 2D vector scene
        self.scene.reset();
        let cam_affine = camera_affine(cam, self.width as f64, self.height as f64);
        for layer in &spec.layers {
            let anim = eval_layer_anim(layer, t);
            if !anim.visible {
                continue;
            }
            let eff_cam_aff = if layer.fixed {
                Affine::IDENTITY
            } else {
                cam_affine
            };
            draw_vector_layer(
                &mut self.scene,
                layer,
                anim,
                eff_cam_aff,
                fonts,
                t,
                self.width as f32,
            )?;
        }

        self.vello
            .render_to_texture(
                &self.device,
                &self.queue,
                &self.scene,
                &self.vello_view,
                &RenderParams {
                    base_color: Color::TRANSPARENT,
                    width: self.width,
                    height: self.height,
                    antialiasing_method: AaConfig::Area,
                },
            )
            .map_err(|e| anyhow!("Vello render_to_texture failed: {e}"))?;

        // Pass 3: Composite Vello overlay + vignette + copy to current ping-pong slot
        let comp_u = CompositeUniforms {
            vignette: spec.vignette,
            _pad: [0.0; 3],
        };
        self.queue
            .write_buffer(&self.comp_uniform_buf, 0, bytemuck::bytes_of(&comp_u));

        let slot = self.next_slot;
        self.next_slot = 1 - slot;

        let mut encoder2 = self
            .device
            .create_command_encoder(&wgpu::CommandEncoderDescriptor {
                label: Some("composite_encoder"),
            });
        {
            let mut rpass = encoder2.begin_render_pass(&wgpu::RenderPassDescriptor {
                label: Some("composite_pass"),
                color_attachments: &[Some(wgpu::RenderPassColorAttachment {
                    view: &self.final_view,
                    resolve_target: None,
                    ops: wgpu::Operations {
                        load: wgpu::LoadOp::Clear(wgpu::Color::BLACK),
                        store: wgpu::StoreOp::Store,
                    },
                    depth_slice: None,
                })],
                depth_stencil_attachment: None,
                timestamp_writes: None,
                occlusion_query_set: None,
                multiview_mask: None,
            });
            rpass.set_pipeline(&self.comp_pipeline);
            rpass.set_bind_group(0, &self.comp_bind_group, &[]);
            rpass.draw(0..3, 0..1);
        }

        encoder2.copy_texture_to_buffer(
            wgpu::TexelCopyTextureInfo {
                texture: &self.final_tex,
                mip_level: 0,
                origin: wgpu::Origin3d::ZERO,
                aspect: wgpu::TextureAspect::All,
            },
            wgpu::TexelCopyBufferInfo {
                buffer: &self.readback_bufs[slot],
                layout: wgpu::TexelCopyBufferLayout {
                    offset: 0,
                    bytes_per_row: Some(self.padded_bytes_per_row),
                    rows_per_image: Some(self.height),
                },
            },
            wgpu::Extent3d {
                width: self.width,
                height: self.height,
                depth_or_array_layers: 1,
            },
        );
        self.queue.submit(Some(encoder2.finish()));

        // Kick off non-blocking map_async on newly submitted slot
        let (tx, rx) = std::sync::mpsc::channel();
        self.readback_bufs[slot]
            .slice(..)
            .map_async(wgpu::MapMode::Read, move |res| {
                let _ = tx.send(res);
            });
        let prev = self.pending.replace(PendingReadback { slot, rx });

        // Drain PREVIOUS slot while GPU executes `slot`
        if let Some(p) = prev {
            self.finish_readback(p, out_rgba)?;
            Ok(true)
        } else {
            Ok(false)
        }
    }

    pub fn flush_pending(&mut self, out_rgba: &mut Vec<u8>) -> Result<bool> {
        if let Some(p) = self.pending.take() {
            self.finish_readback(p, out_rgba)?;
            Ok(true)
        } else {
            Ok(false)
        }
    }

    fn finish_readback(&self, p: PendingReadback, out_rgba: &mut Vec<u8>) -> Result<()> {
        let _ = self.device.poll(wgpu::PollType::wait_indefinitely());
        p.rx.recv()
            .context("readback map channel closed")?
            .map_err(|e| anyhow!("buffer map_async failed: {e}"))?;

        let buf = &self.readback_bufs[p.slot];
        let slice = buf.slice(..);
        let mapped = slice
            .get_mapped_range()
            .map_err(|e| anyhow!("get_mapped_range failed: {e}"))?;
        let unpadded = (self.width * 4) as usize;
        let padded = self.padded_bytes_per_row as usize;
        let total = unpadded * (self.height as usize);
        out_rgba.resize(total, 0);
        if padded == unpadded {
            out_rgba.copy_from_slice(&mapped[..total]);
        } else {
            for row in 0..(self.height as usize) {
                let src_off = row * padded;
                let dst_off = row * unpadded;
                out_rgba[dst_off..dst_off + unpadded]
                    .copy_from_slice(&mapped[src_off..src_off + unpadded]);
            }
        }
        drop(mapped);
        buf.unmap();
        Ok(())
    }
}

fn to_vello_color(c: Rgba, opacity: f32) -> Color {
    let c = c.with_alpha_mul(opacity);
    Color::from_rgba8(c.r, c.g, c.b, c.a)
}

fn camera_affine(cam: CameraState, w: f64, h: f64) -> Affine {
    let cx = w * 0.5;
    let cy = h * 0.5;
    Affine::translate((cx + cam.x as f64, cy + cam.y as f64))
        * Affine::scale(cam.scale as f64)
        * Affine::translate((-cx, -cy))
}

fn layer_affine(cam_aff: Affine, anim: AnimState, pivot_x: f64, pivot_y: f64) -> Affine {
    let rad = (anim.rotation_deg as f64).to_radians();
    cam_aff
        * Affine::translate((pivot_x + anim.dx as f64, pivot_y + anim.dy as f64))
        * Affine::rotate(rad)
        * Affine::scale(anim.scale as f64)
        * Affine::translate((-pivot_x, -pivot_y))
}

#[allow(clippy::too_many_arguments)]
fn apply_box_transform(
    x: f32,
    y: f32,
    w: f32,
    h: f32,
    anim: AnimState,
    cam: CameraState,
    canvas_w: f32,
    canvas_h: f32,
) -> (f32, f32, f32, f32) {
    let bw = w * anim.scale;
    let bh = h * anim.scale;
    let cx_box = x + w * 0.5 + anim.dx;
    let cy_box = y + h * 0.5 + anim.dy;

    let canvas_cx = canvas_w * 0.5;
    let canvas_cy = canvas_h * 0.5;
    let world_cx = (cx_box - canvas_cx) * cam.scale + canvas_cx + cam.x;
    let world_cy = (cy_box - canvas_cy) * cam.scale + canvas_cy + cam.y;
    let final_w = bw * cam.scale;
    let final_h = bh * cam.scale;
    (
        world_cx - final_w * 0.5,
        world_cy - final_h * 0.5,
        final_w,
        final_h,
    )
}

fn eval_value_keyframes<'a>(
    default_text: &'a str,
    default_color: &'a str,
    keyframes: &'a [ValueKeyframe],
    t: f64,
) -> (&'a str, &'a str, f32) {
    let mut text = default_text;
    let mut color = default_color;
    let mut jump_scale = 1.0_f32;
    for kf in keyframes {
        if t >= kf.at {
            text = kf.text.as_str();
            if let Some(c) = &kf.color {
                color = c.as_str();
            }
            let elapsed = t - kf.at;
            if (0.0..0.30).contains(&elapsed) {
                let u = (elapsed / 0.30) as f32;
                let e = ease(crate::schema::EaseKind::Power4Out, u);
                jump_scale = kf.jump_scale + (1.0 - kf.jump_scale) * e;
            }
        }
    }
    (text, color, jump_scale)
}

fn draw_tag_badge(
    scene: &mut Scene,
    tag: &TagSpec,
    box_x: f32,
    box_y: f32,
    aff: Affine,
    opacity: f32,
    fonts: &FontRegistry,
) -> Result<()> {
    let font = &fonts.mono.font_data;
    let tw = measure_line_width(font, &tag.text, tag.font_size, 1.0);
    let pad_x = 14.0_f32;
    let pad_y = 6.0_f32;
    let tx = box_x + 14.0;
    let ty = box_y + 12.0;
    let rw = tw + pad_x * 2.0;
    let rh = tag.font_size * 1.18 + pad_y * 2.0;
    let rrect = RoundedRect::new(tx as f64, ty as f64, (tx + rw) as f64, (ty + rh) as f64, 6.0);
    let bg = to_vello_color(Rgba::parse(&tag.bg)?, opacity);
    let fg = to_vello_color(Rgba::parse(&tag.fg)?, opacity);
    scene.fill(Fill::NonZero, aff, bg, None, &rrect);
    draw_text_block(
        scene,
        font,
        &tag.text,
        tag.font_size,
        tx + pad_x,
        ty + pad_y + 1.0,
        tw + 20.0,
        TextAlign::Left,
        fg,
        aff,
        1.0,
        1.05,
    );
    Ok(())
}

fn draw_vector_layer(
    scene: &mut Scene,
    layer: &LayerSpec,
    anim: AnimState,
    cam_aff: Affine,
    fonts: &FontRegistry,
    t: f64,
    canvas_w: f32,
) -> Result<()> {
    match &layer.kind {
        LayerKind::VideoBox {
            x,
            y,
            width,
            height,
            tag: Some(tag),
            ..
        }
        | LayerKind::ImageBox {
            x,
            y,
            width,
            height,
            tag: Some(tag),
            ..
        } => {
            let aff = layer_affine(cam_aff, anim, (*x + *width * 0.5) as f64, (*y + *height * 0.5) as f64);
            draw_tag_badge(scene, tag, *x, *y, aff, anim.opacity, fonts)?;
        }
        LayerKind::VideoBox { tag: None, .. } | LayerKind::ImageBox { tag: None, .. } => {}
        LayerKind::Rect {
            x,
            y,
            width,
            height,
            radius,
            fill,
            border_width,
            border_color,
            text,
            text_color,
            text_size,
            value_keyframes,
            fill_keyframes,
        } => {
            let aff = layer_affine(cam_aff, anim, (*x + *width * 0.5) as f64, (*y + *height * 0.5) as f64);
            let rr = RoundedRect::new(
                *x as f64,
                *y as f64,
                (*x + *width) as f64,
                (*y + *height) as f64,
                *radius as f64,
            );
            let mut cur_fill = fill.as_str();
            let mut cur_border = border_color.as_deref();
            for kf in fill_keyframes {
                if t >= kf.at {
                    cur_fill = kf.fill.as_str();
                    if let Some(bc) = &kf.border_color {
                        cur_border = Some(bc.as_str());
                    }
                }
            }
            scene.fill(
                Fill::NonZero,
                aff,
                to_vello_color(Rgba::parse(cur_fill)?, anim.opacity),
                None,
                &rr,
            );
            if *border_width > 0.0
                && let Some(bc) = cur_border
            {
                scene.stroke(
                    &Stroke::new(*border_width as f64),
                    aff,
                    to_vello_color(Rgba::parse(bc)?, anim.opacity),
                    None,
                    &rr,
                );
            }
            let base_txt = text.as_deref().unwrap_or("");
            let base_col = text_color.as_deref().unwrap_or("#FFFFFF");
            let (cur_txt, cur_col, jump_scale) =
                eval_value_keyframes(base_txt, base_col, value_keyframes, t);
            if !cur_txt.is_empty() {
                let sz = text_size.unwrap_or(44.0);
                let txt_aff = if (jump_scale - 1.0).abs() > 0.001 {
                    let px = (*x + *width * 0.5) as f64;
                    let py = (*y + *height * 0.5) as f64;
                    aff * Affine::translate((px, py))
                        * Affine::scale(jump_scale as f64)
                        * Affine::translate((-px, -py))
                } else {
                    aff
                };
                draw_text_block(
                    scene,
                    &fonts.display.font_data,
                    cur_txt,
                    sz,
                    *x,
                    *y + (*height - sz) * 0.45,
                    *width,
                    TextAlign::Center,
                    to_vello_color(Rgba::parse(cur_col)?, anim.opacity),
                    txt_aff,
                    0.0,
                    1.0,
                );
            }
        }
        LayerKind::Panel {
            x,
            y,
            width,
            height,
            radius,
            bg,
            border_color,
            accent_top,
            shadow,
        } => {
            let aff = layer_affine(cam_aff, anim, (*x + *width * 0.5) as f64, (*y + *height * 0.5) as f64);
            let rect = Rect::new(*x as f64, *y as f64, (*x + *width) as f64, (*y + *height) as f64);
            if *shadow {
                let srect = Rect::new(
                    *x as f64,
                    (*y + 20.0) as f64,
                    (*x + *width) as f64,
                    (*y + *height + 20.0) as f64,
                );
                scene.draw_blurred_rounded_rect(
                    aff,
                    srect,
                    Color::from_rgba8(0, 0, 0, (125.0 * anim.opacity) as u8),
                    *radius as f64,
                    20.0,
                );
            }
            let rr = RoundedRect::from_rect(rect, *radius as f64);
            scene.fill(
                Fill::NonZero,
                aff,
                to_vello_color(Rgba::parse(bg)?, anim.opacity),
                None,
                &rr,
            );
            scene.stroke(
                &Stroke::new(1.5),
                aff,
                to_vello_color(Rgba::parse(border_color)?, anim.opacity),
                None,
                &rr,
            );
            if let Some(acc) = accent_top {
                let bar = RoundedRect::new(
                    (*x + 2.0) as f64,
                    *y as f64,
                    (*x + *width - 2.0) as f64,
                    (*y + 5.0) as f64,
                    3.0,
                );
                scene.fill(
                    Fill::NonZero,
                    aff,
                    to_vello_color(Rgba::parse(acc)?, anim.opacity),
                    None,
                    &bar,
                );
            }
        }
        LayerKind::Pedestal {
            x,
            y,
            width,
            max_height,
            fraction,
            color,
            top_text,
            bottom_text,
            sub_text,
            anim_duration,
        } => {
            let aff = layer_affine(
                cam_aff,
                anim,
                (*x + *width * 0.5) as f64,
                (*y + *max_height * 0.5) as f64,
            );
            let u = if *anim_duration > 0.0 {
                ((t - layer.start - 0.06) / *anim_duration).clamp(0.0, 1.0) as f32
            } else {
                1.0
            };
            let grow = ease(crate::schema::EaseKind::Power3Out, u);
            let target_h = (*max_height * fraction.clamp(0.0, 1.0)).max(16.0);
            let cur_h = (target_h * grow).max(6.0);
            let rgba = Rgba::parse(color)?;
            let base_y = *y + *max_height;

            // Dark architectural reference track (100% ceiling)
            let track = RoundedRect::new(*x as f64, *y as f64, (*x + *width) as f64, base_y as f64, 14.0);
            scene.fill(
                Fill::NonZero,
                aff,
                Color::from_rgba8(255, 255, 255, (10.0 * anim.opacity) as u8),
                None,
                &track,
            );
            scene.stroke(
                &Stroke::new(1.5),
                aff,
                Color::from_rgba8(255, 255, 255, (22.0 * anim.opacity) as u8),
                None,
                &track,
            );

            // Soft glow behind bar
            let bar_top = base_y - cur_h;
            let bar_rect = Rect::new(*x as f64, bar_top as f64, (*x + *width) as f64, base_y as f64);
            scene.draw_blurred_rounded_rect(
                aff,
                bar_rect,
                to_vello_color(rgba, anim.opacity * 0.25),
                10.0,
                22.0,
            );

            // Vertical gradient column fill
            let bar_rr = RoundedRect::from_rect(bar_rect, 10.0);
            let grad = Gradient::new_linear(
                (*x, bar_top),
                (*x, base_y),
            )
            .with_stops([
                (0.0_f32, to_vello_color(rgba, anim.opacity)),
                (0.55_f32, to_vello_color(rgba, anim.opacity * 0.78)),
                (1.0_f32, to_vello_color(rgba, anim.opacity * 0.30)),
            ]);
            scene.fill(Fill::NonZero, aff, &grad, None, &bar_rr);

            // Crisp top highlight cap
            let cap = RoundedRect::new(
                (*x + 4.0) as f64,
                bar_top as f64,
                (*x + *width - 4.0) as f64,
                (bar_top + 4.0).min(base_y) as f64,
                2.0,
            );
            scene.fill(
                Fill::NonZero,
                aff,
                Color::from_rgba8(255, 255, 255, (130.0 * anim.opacity) as u8),
                None,
                &cap,
            );

            // Baseline horizontal rule
            let line = KurboLine::new(
                ((*x - 22.0) as f64, base_y as f64),
                ((*x + *width + 22.0) as f64, base_y as f64),
            );
            scene.stroke(
                &Stroke::new(3.0),
                aff,
                Color::from_rgba8(255, 255, 255, (64.0 * anim.opacity) as u8),
                None,
                &line,
            );

            // Big value text above column
            let top_y = (base_y - target_h - 126.0).max(*y - 120.0);
            draw_text_block(
                scene,
                &fonts.display.font_data,
                top_text,
                96.0,
                *x - 20.0,
                top_y,
                *width + 40.0,
                TextAlign::Center,
                to_vello_color(rgba, anim.opacity),
                aff,
                1.0,
                1.0,
            );
            // Bottom label
            draw_text_block(
                scene,
                &fonts.mono.font_data,
                bottom_text,
                28.0,
                *x - 30.0,
                base_y + 20.0,
                *width + 60.0,
                TextAlign::Center,
                Color::from_rgba8(237, 242, 247, (255.0 * anim.opacity) as u8),
                aff,
                1.0,
                1.1,
            );
            if let Some(sub) = sub_text {
                draw_text_block(
                    scene,
                    &fonts.mono.font_data,
                    sub,
                    24.0,
                    *x - 30.0,
                    base_y + 58.0,
                    *width + 60.0,
                    TextAlign::Center,
                    Color::from_rgba8(148, 160, 178, (255.0 * anim.opacity) as u8),
                    aff,
                    0.8,
                    1.1,
                );
            }
        }
        LayerKind::Donut {
            cx,
            cy,
            radius,
            stroke_width,
            fraction,
            color,
            track_color,
            value_text,
            label,
            ref_fraction,
            ref_color,
            anim_duration,
        } => {
            let aff = layer_affine(cam_aff, anim, *cx as f64, *cy as f64);
            let u = if *anim_duration > 0.0 {
                ((t - layer.start) / *anim_duration).clamp(0.0, 1.0) as f32
            } else {
                1.0
            };
            let cur_frac = fraction.clamp(0.0, 1.0) * ease(crate::schema::EaseKind::Power3Out, u);
            let r = *radius as f64;
            let track_circle = Circle::new((*cx as f64, *cy as f64), r);
            scene.stroke(
                &Stroke::new(*stroke_width as f64),
                aff,
                to_vello_color(Rgba::parse(track_color)?, anim.opacity),
                None,
                &track_circle,
            );
            if cur_frac > 0.002 {
                let sweep = std::f64::consts::TAU * (cur_frac as f64);
                let arc = KurboArc::new(
                    Point::new(*cx as f64, *cy as f64),
                    Vec2::new(r, r),
                    -std::f64::consts::FRAC_PI_2,
                    sweep,
                    0.0,
                );
                let rgba = Rgba::parse(color)?;
                let st = Stroke::new(*stroke_width as f64).with_caps(Cap::Round);
                let glow_st = Stroke::new((*stroke_width as f64) * 1.65).with_caps(Cap::Round);
                scene.stroke(&glow_st, aff, to_vello_color(rgba, anim.opacity * 0.22), None, &arc);
                scene.stroke(&st, aff, to_vello_color(rgba, anim.opacity), None, &arc);
            }
            // Optional reference tick (e.g. 50% pro mean)
            if let Some(rf) = ref_fraction {
                let ang = -std::f64::consts::FRAC_PI_2 + std::f64::consts::TAU * (*rf as f64);
                let inner_r = r - (*stroke_width as f64) * 0.85;
                let outer_r = r + (*stroke_width as f64) * 0.85;
                let p1 = Point::new(*cx as f64 + inner_r * ang.cos(), *cy as f64 + inner_r * ang.sin());
                let p2 = Point::new(*cx as f64 + outer_r * ang.cos(), *cy as f64 + outer_r * ang.sin());
                let rcol = ref_color.as_deref().unwrap_or("#FACC15");
                scene.stroke(
                    &Stroke::new(7.0).with_caps(Cap::Round),
                    aff,
                    to_vello_color(Rgba::parse(rcol)?, anim.opacity),
                    None,
                    &KurboLine::new(p1, p2),
                );
            }
            // Centered numeric value (counting up during sweep)
            let shown_val = if u < 0.999 && value_text.ends_with('%') {
                let pct = cur_frac * 100.0;
                format!("{pct:.1}%").replace('.', ",")
            } else {
                value_text.clone()
            };
            draw_text_block(
                scene,
                &fonts.display.font_data,
                &shown_val,
                88.0,
                *cx - *radius + 24.0,
                *cy - 82.0,
                (*radius - 24.0) * 2.0,
                TextAlign::Center,
                Color::from_rgba8(237, 242, 247, (255.0 * anim.opacity) as u8),
                aff,
                1.0,
                1.0,
            );
            if let Some(lbl) = label {
                draw_text_block(
                    scene,
                    &fonts.mono.font_data,
                    lbl,
                    24.0,
                    *cx - *radius + 40.0,
                    *cy + 46.0,
                    (*radius - 40.0) * 2.0,
                    TextAlign::Center,
                    Color::from_rgba8(148, 160, 178, (255.0 * anim.opacity) as u8),
                    aff,
                    0.8,
                    1.15,
                );
            }
        }
        LayerKind::Door {
            x,
            y,
            width,
            height,
            open_at,
            open_duration,
            frame_color,
            door_color,
            glow_color,
        } => {
            let aff = layer_affine(cam_aff, anim, (*x + *width * 0.5) as f64, (*y + *height * 0.5) as f64);
            let u = if *open_duration > 0.0 {
                ((t - *open_at) / *open_duration).clamp(0.0, 1.0) as f32
            } else if t >= *open_at {
                1.0
            } else {
                0.0
            };
            let open = ease(crate::schema::EaseKind::Power3InOut, u);
            let g_rgba = Rgba::parse(glow_color)?;
            let f_rgba = Rgba::parse(frame_color)?;
            let d_rgba = Rgba::parse(door_color)?;

            let x0 = *x as f64;
            let y0 = *y as f64;
            let w = *width as f64;
            let h = *height as f64;

            // Volumetric floor light beam spilling forward as door opens
            if open > 0.02 {
                let spread = w * 1.25 * (open as f64);
                let mut beam = BezPath::new();
                beam.move_to(Point::new(x0 + 8.0, y0 + h));
                beam.line_to(Point::new(x0 + w - 8.0, y0 + h));
                beam.line_to(Point::new(x0 + w + spread, y0 + h + 135.0));
                beam.line_to(Point::new(x0 - spread, y0 + h + 135.0));
                beam.close_path();
                let beam_grad = Gradient::new_linear(
                    (*x + *width * 0.5, *y + *height),
                    (*x + *width * 0.5, *y + *height + 135.0),
                )
                .with_stops([
                    (0.0_f32, to_vello_color(g_rgba, anim.opacity * 0.58 * open)),
                    (0.5_f32, to_vello_color(g_rgba, anim.opacity * 0.22 * open)),
                    (1.0_f32, to_vello_color(g_rgba, 0.0)),
                ]);
                scene.fill(Fill::NonZero, aff, &beam_grad, None, &beam);

                // Warm bright interior doorway glow
                let inner_rect = Rect::new(x0 + 8.0, y0 + 8.0, x0 + w - 8.0, y0 + h);
                let inner_grad = Gradient::new_linear(
                    (*x, *y),
                    (*x, *y + *height),
                )
                .with_stops([
                    (0.0_f32, Color::from_rgba8(255, 248, 220, (245.0 * anim.opacity * open) as u8)),
                    (1.0_f32, to_vello_color(g_rgba, anim.opacity * 0.78 * open)),
                ]);
                scene.fill(Fill::NonZero, aff, &inner_grad, None, &inner_rect);
            }

            // Door leaf swinging inward/outward with crisp perspective trapezoid
            let leaf_w = (w - 16.0) * (1.0 - 0.84 * (open as f64));
            let skew_y = 26.0 * (open as f64);
            let mut leaf = BezPath::new();
            leaf.move_to(Point::new(x0 + 8.0, y0 + 8.0));
            leaf.line_to(Point::new(x0 + 8.0 + leaf_w, y0 + 8.0 + skew_y));
            leaf.line_to(Point::new(x0 + 8.0 + leaf_w, y0 + h - skew_y * 0.5));
            leaf.line_to(Point::new(x0 + 8.0, y0 + h));
            leaf.close_path();
            scene.fill(Fill::NonZero, aff, to_vello_color(d_rgba, anim.opacity), None, &leaf);
            scene.stroke(
                &Stroke::new(2.5),
                aff,
                Color::from_rgba8(62, 74, 98, (220.0 * anim.opacity) as u8),
                None,
                &leaf,
            );
            // Door handle
            if leaf_w > 22.0 {
                let hx = x0 + 8.0 + leaf_w * 0.84;
                let hy = y0 + h * 0.52 + skew_y * 0.2;
                scene.fill(
                    Fill::NonZero,
                    aff,
                    to_vello_color(f_rgba, anim.opacity),
                    None,
                    &Circle::new((hx, hy), 6.5),
                );
            }

            // Architectural outer portal frame (top + sides) + threshold floor line
            let mut frame = BezPath::new();
            frame.move_to(Point::new(x0, y0 + h));
            frame.line_to(Point::new(x0, y0));
            frame.line_to(Point::new(x0 + w, y0));
            frame.line_to(Point::new(x0 + w, y0 + h));
            scene.stroke(
                &Stroke::new(16.0),
                aff,
                to_vello_color(f_rgba, anim.opacity * 0.22),
                None,
                &frame,
            );
            scene.stroke(
                &Stroke::new(9.0),
                aff,
                to_vello_color(f_rgba, anim.opacity),
                None,
                &frame,
            );
            let floor = KurboLine::new((x0 - 150.0, y0 + h + 2.0), (x0 + w + 150.0, y0 + h + 2.0));
            scene.stroke(
                &Stroke::new(2.5),
                aff,
                Color::from_rgba8(255, 255, 255, (60.0 * anim.opacity) as u8),
                None,
                &floor,
            );
        }
        LayerKind::Line {
            x1,
            y1,
            x2,
            y2,
            stroke_width,
            color,
            anim_duration,
        } => {
            let aff = layer_affine(cam_aff, anim, ((*x1 + *x2) * 0.5) as f64, ((*y1 + *y2) * 0.5) as f64);
            let u = if *anim_duration > 0.0 {
                ((t - layer.start) / *anim_duration).clamp(0.0, 1.0) as f32
            } else {
                1.0
            };
            let prog = ease(crate::schema::EaseKind::Power3Out, u);
            let ex = *x1 + (*x2 - *x1) * prog;
            let ey = *y1 + (*y2 - *y1) * prog;
            let line = KurboLine::new((*x1 as f64, *y1 as f64), (ex as f64, ey as f64));
            scene.stroke(
                &Stroke::new(*stroke_width as f64).with_caps(Cap::Round),
                aff,
                to_vello_color(Rgba::parse(color)?, anim.opacity),
                None,
                &line,
            );
        }
        LayerKind::Card {
            x,
            y,
            width,
            height,
            bg,
            accent_top,
            fold,
            kicker,
            title,
            subtitle,
        } => {
            let aff = layer_affine(cam_aff, anim, (*x + *width * 0.5) as f64, (*y + *height * 0.5) as f64);
            let rect = Rect::new(*x as f64, *y as f64, (*x + *width) as f64, (*y + *height) as f64);
            // Soft shadow
            let shadow_rect = Rect::new(
                *x as f64,
                (*y + 24.0) as f64,
                (*x + *width) as f64,
                (*y + *height + 24.0) as f64,
            );
            scene.draw_blurred_rounded_rect(
                aff,
                shadow_rect,
                Color::from_rgba8(0, 0, 0, (140.0 * anim.opacity) as u8),
                8.0,
                22.0,
            );
            let rr = RoundedRect::from_rect(rect, 6.0);
            scene.fill(
                Fill::NonZero,
                aff,
                to_vello_color(Rgba::parse(bg)?, anim.opacity),
                None,
                &rr,
            );
            if let Some(acc) = accent_top {
                let top_bar = Rect::new(*x as f64, *y as f64, (*x + *width) as f64, (*y + 10.0) as f64);
                scene.fill(
                    Fill::NonZero,
                    aff,
                    to_vello_color(Rgba::parse(acc)?, anim.opacity),
                    None,
                    &top_bar,
                );
            }
            if *fold {
                let fsz = 44.0_f64;
                let mut tri = BezPath::new();
                tri.move_to(Point::new((*x + *width) as f64 - fsz, *y as f64));
                tri.line_to(Point::new((*x + *width) as f64, *y as f64));
                tri.line_to(Point::new((*x + *width) as f64, *y as f64 + fsz));
                tri.close_path();
                scene.fill(
                    Fill::NonZero,
                    aff,
                    Color::from_rgba8(7, 9, 14, (255.0 * anim.opacity) as u8),
                    None,
                    &tri,
                );
                let mut crease = BezPath::new();
                crease.move_to(Point::new((*x + *width) as f64 - fsz, *y as f64));
                crease.line_to(Point::new((*x + *width) as f64 - fsz, *y as f64 + fsz));
                crease.line_to(Point::new((*x + *width) as f64, *y as f64 + fsz));
                crease.close_path();
                scene.fill(
                    Fill::NonZero,
                    aff,
                    Color::from_rgba8(207, 198, 179, (255.0 * anim.opacity) as u8),
                    None,
                    &crease,
                );
            }
            let ink = Color::from_rgba8(21, 23, 28, (255.0 * anim.opacity) as u8);
            let mut cy = *y + 34.0;
            if let Some(k) = kicker {
                let m = draw_text_block(
                    scene,
                    &fonts.mono.font_data,
                    k,
                    26.0,
                    *x + 34.0,
                    cy,
                    *width - 80.0,
                    TextAlign::Left,
                    Color::from_rgba8(21, 23, 28, (180.0 * anim.opacity) as u8),
                    aff,
                    1.2,
                    1.1,
                );
                cy += m.height + 10.0;
            }
            if let Some(tt) = title {
                let m = draw_text_block(
                    scene,
                    &fonts.display.font_data,
                    tt,
                    68.0,
                    *x + 34.0,
                    cy,
                    *width - 70.0,
                    TextAlign::Left,
                    ink,
                    aff,
                    0.5,
                    1.02,
                );
                cy += m.height + 12.0;
            }
            if let Some(sub) = subtitle {
                draw_text_block(
                    scene,
                    &fonts.mono.font_data,
                    sub,
                    26.0,
                    *x + 34.0,
                    cy,
                    *width - 70.0,
                    TextAlign::Left,
                    ink,
                    aff,
                    0.8,
                    1.2,
                );
            }
        }
        LayerKind::Text {
            text,
            x,
            y,
            width,
            size,
            color,
            font,
            align,
            counter,
            value_keyframes,
        } => {
            let owned_counter: String;
            let (cur_text, cur_color, jump_scale) = if let Some(ctr) = counter {
                let u = if ctr.duration > 0.0 {
                    ((t - ctr.start) / ctr.duration).clamp(0.0, 1.0) as f32
                } else if t >= ctr.start {
                    1.0
                } else {
                    0.0
                };
                let val = ctr.from + (ctr.to - ctr.from) * (ease(ctr.ease, u) as f64);
                owned_counter = format!("{:.*}", ctr.decimals, val).replace('.', ",") + &ctr.suffix;
                let (kf_txt, kf_col, kf_jump) = eval_value_keyframes(&owned_counter, color, value_keyframes, t);
                let final_txt = if value_keyframes.iter().any(|k| t >= k.at && !k.text.is_empty()) {
                    kf_txt
                } else {
                    owned_counter.as_str()
                };
                (final_txt, kf_col, kf_jump)
            } else {
                eval_value_keyframes(text, color, value_keyframes, t)
            };

            let loaded = fonts.get(*font);
            let letter_spacing = match font {
                FontRole::Mono => *size * 0.04,
                FontRole::Display => *size * 0.01,
                FontRole::Sans => 0.0,
            };
            let pivot_x = match align {
                TextAlign::Left => *x + (*width * 0.25).min(240.0),
                TextAlign::Center => *x + *width * 0.5,
                TextAlign::Right => *x + *width * 0.75,
            };
            let pivot_y = *y + *size * 0.5;
            let mut local_anim = anim;
            local_anim.scale *= jump_scale;
            let aff = layer_affine(cam_aff, local_anim, pivot_x as f64, pivot_y as f64);
            draw_text_block(
                scene,
                &loaded.font_data,
                cur_text,
                *size,
                *x,
                *y,
                *width,
                *align,
                to_vello_color(Rgba::parse(cur_color)?, anim.opacity),
                aff,
                letter_spacing,
                1.05,
            );
        }
        LayerKind::Chip {
            text,
            x,
            y,
            bg,
            fg,
            font_size,
        } => {
            let font = &fonts.mono.font_data;
            let spacing = *font_size * 0.04;
            let tw = measure_line_width(font, text, *font_size, spacing);
            let pad_x = 20.0_f32;
            let pad_y = 10.0_f32;
            let rw = tw + pad_x * 2.0;
            let rh = *font_size * 1.18 + pad_y * 2.0;
            let aff = layer_affine(cam_aff, anim, (*x + rw * 0.5) as f64, (*y + rh * 0.5) as f64);
            let rrect = RoundedRect::new(
                *x as f64,
                *y as f64,
                (*x + rw) as f64,
                (*y + rh) as f64,
                8.0,
            );
            scene.fill(
                Fill::NonZero,
                aff,
                to_vello_color(Rgba::parse(bg)?, anim.opacity),
                None,
                &rrect,
            );
            draw_text_block(
                scene,
                font,
                text,
                *font_size,
                *x + pad_x,
                *y + pad_y + 1.0,
                tw + 20.0,
                TextAlign::Left,
                to_vello_color(Rgba::parse(fg)?, anim.opacity),
                aff,
                spacing,
                1.05,
            );
        }
        LayerKind::HudBox {
            label,
            value,
            x,
            y,
            width,
            value_color,
            value_size,
            accent_top,
            value_keyframes,
            label_keyframes,
        } => {
            let height = 148.0_f32;
            let aff = layer_affine(cam_aff, anim, (*x + *width * 0.5) as f64, (*y + height * 0.5) as f64);
            let rr = RoundedRect::new(
                *x as f64,
                *y as f64,
                (*x + *width) as f64,
                (*y + height) as f64,
                14.0,
            );
            scene.fill(
                Fill::NonZero,
                aff,
                Color::from_rgba8(13, 18, 29, (226.0 * anim.opacity) as u8),
                None,
                &rr,
            );
            scene.stroke(
                &Stroke::new(1.5),
                aff,
                Color::from_rgba8(255, 255, 255, (38.0 * anim.opacity) as u8),
                None,
                &rr,
            );
            let (cur_val, cur_col, jump_scale) =
                eval_value_keyframes(value, value_color, value_keyframes, t);
            let acc_str = accent_top.as_deref().unwrap_or(cur_col);
            let acc_bar = RoundedRect::new(
                (*x + 2.0) as f64,
                *y as f64,
                (*x + *width - 2.0) as f64,
                (*y + 4.5) as f64,
                3.0,
            );
            scene.fill(
                Fill::NonZero,
                aff,
                to_vello_color(Rgba::parse(acc_str)?, anim.opacity),
                None,
                &acc_bar,
            );
            let (cur_lbl, _, _) = eval_value_keyframes(label, "#94A0B2", label_keyframes, t);
            draw_text_block(
                scene,
                &fonts.mono.font_data,
                cur_lbl,
                24.0,
                *x + 24.0,
                *y + 18.0,
                *width - 48.0,
                TextAlign::Left,
                Color::from_rgba8(148, 160, 178, (255.0 * anim.opacity) as u8),
                aff,
                1.0,
                1.05,
            );
            let val_x = *x + 24.0;
            let val_y = *y + 54.0;
            let val_aff = if (jump_scale - 1.0).abs() > 0.001 {
                let px = (val_x + 70.0) as f64;
                let py = (val_y + *value_size * 0.5) as f64;
                aff * Affine::translate((px, py))
                    * Affine::scale(jump_scale as f64)
                    * Affine::translate((-px, -py))
            } else {
                aff
            };
            draw_text_block(
                scene,
                &fonts.display.font_data,
                cur_val,
                *value_size,
                val_x,
                val_y,
                *width - 48.0,
                TextAlign::Left,
                to_vello_color(Rgba::parse(cur_col)?, anim.opacity),
                val_aff,
                0.5,
                1.0,
            );
        }
        LayerKind::Ring {
            cx,
            cy,
            diameter,
            color,
            stroke_width,
            glow,
        } => {
            let aff = layer_affine(cam_aff, anim, *cx as f64, *cy as f64);
            let r = (*diameter * 0.5) as f64;
            let rgba = Rgba::parse(color)?;
            let circle = Circle::new((*cx as f64, *cy as f64), r);
            if *glow {
                // Multi-ring soft glow halo
                for (mult, alpha) in [(3.4, 0.14), (2.2, 0.25), (1.45, 0.42)] {
                    scene.stroke(
                        &Stroke::new((*stroke_width as f64) * mult),
                        aff,
                        to_vello_color(rgba, anim.opacity * alpha),
                        None,
                        &circle,
                    );
                }
            }
            scene.stroke(
                &Stroke::new(*stroke_width as f64),
                aff,
                to_vello_color(rgba, anim.opacity),
                None,
                &circle,
            );
        }
        LayerKind::Stamp {
            text,
            x,
            y,
            color,
            font_size,
        } => {
            let font = &fonts.display.font_data;
            let tw = measure_line_width(font, text, *font_size, 1.5);
            let pad_x = 30.0_f32;
            let pad_y = 18.0_f32;
            let rw = tw + pad_x * 2.0;
            let rh = *font_size * 0.96 + pad_y * 2.0;
            let aff = layer_affine(cam_aff, anim, (*x + rw * 0.5) as f64, (*y + rh * 0.5) as f64);
            let rgba = Rgba::parse(color)?;
            let col = to_vello_color(rgba, anim.opacity);
            // Outer + inner double border (matches CSS `border: 8px double`)
            let outer = RoundedRect::new(
                *x as f64,
                *y as f64,
                (*x + rw) as f64,
                (*y + rh) as f64,
                10.0,
            );
            let inner = RoundedRect::new(
                (*x + 7.0) as f64,
                (*y + 7.0) as f64,
                (*x + rw - 7.0) as f64,
                (*y + rh - 7.0) as f64,
                6.0,
            );
            scene.fill(
                Fill::NonZero,
                aff,
                Color::from_rgba8(7, 9, 14, (218.0 * anim.opacity) as u8),
                None,
                &outer,
            );
            scene.stroke(&Stroke::new(4.0), aff, col, None, &outer);
            scene.stroke(&Stroke::new(2.5), aff, col, None, &inner);
            draw_text_block(
                scene,
                font,
                text,
                *font_size,
                *x + pad_x,
                *y + pad_y - *font_size * 0.14,
                tw + 20.0,
                TextAlign::Left,
                col,
                aff,
                1.5,
                1.0,
            );
        }
        LayerKind::Legend {
            label,
            dot_color,
            x,
            y,
        } => {
            let font = &fonts.mono.font_data;
            let fs = 28.0_f32;
            let tw = measure_line_width(font, label, fs, 1.0);
            let rw = 14.0 + 22.0 + 12.0 + tw + 16.0;
            let rh = 48.0_f32;
            let aff = layer_affine(cam_aff, anim, (*x + rw * 0.5) as f64, (*y + rh * 0.5) as f64);
            let rr = RoundedRect::new(
                *x as f64,
                *y as f64,
                (*x + rw) as f64,
                (*y + rh) as f64,
                8.0,
            );
            scene.fill(
                Fill::NonZero,
                aff,
                Color::from_rgba8(7, 9, 14, (204.0 * anim.opacity) as u8),
                None,
                &rr,
            );
            let dot = Circle::new((*x as f64 + 25.0, *y as f64 + rh as f64 * 0.5), 11.0);
            scene.fill(
                Fill::NonZero,
                aff,
                to_vello_color(Rgba::parse(dot_color)?, anim.opacity),
                None,
                &dot,
            );
            draw_text_block(
                scene,
                font,
                label,
                fs,
                *x + 48.0,
                *y + 9.0,
                tw + 20.0,
                TextAlign::Left,
                Color::from_rgba8(232, 238, 245, (255.0 * anim.opacity) as u8),
                aff,
                1.0,
                1.0,
            );
        }
        LayerKind::ProgressBar {
            label,
            value_text,
            fraction,
            x,
            y,
            bar_width,
            height,
            label_width,
            color,
            label_color,
            font_size,
            value_size,
            anim_duration,
        } => {
            let u = if *anim_duration > 0.0 {
                ((t - layer.start) / *anim_duration).clamp(0.0, 1.0) as f32
            } else {
                1.0
            };
            let fill_frac = fraction.clamp(0.0, 1.0) * ease(crate::schema::EaseKind::Power3Out, u);
            let aff = layer_affine(
                cam_aff,
                anim,
                (*x + (*label_width + *bar_width) * 0.5) as f64,
                (*y + *height * 0.5) as f64,
            );
            let lbl_rgba = label_color
                .as_deref()
                .map(Rgba::parse)
                .transpose()?
                .unwrap_or(Rgba { r: 237, g: 242, b: 247, a: 255 });
            draw_text_block(
                scene,
                &fonts.sans.font_data,
                label,
                *font_size,
                *x,
                *y + (*height - *font_size) * 0.42,
                *label_width - 18.0,
                TextAlign::Left,
                to_vello_color(lbl_rgba, anim.opacity),
                aff,
                0.0,
                1.0,
            );
            let track_x = *x + *label_width;
            let track = RoundedRect::new(
                track_x as f64,
                *y as f64,
                (track_x + *bar_width) as f64,
                (*y + *height) as f64,
                10.0,
            );
            scene.fill(
                Fill::NonZero,
                aff,
                Color::from_rgba8(255, 255, 255, (16.0 * anim.opacity) as u8),
                None,
                &track,
            );
            let fill_w = (*bar_width * fill_frac).max(6.0);
            let bar = RoundedRect::new(
                track_x as f64,
                *y as f64,
                (track_x + fill_w) as f64,
                (*y + *height) as f64,
                10.0,
            );
            let rgba = Rgba::parse(color)?;
            let col = to_vello_color(rgba, anim.opacity);
            let bgrad = Gradient::new_linear((track_x, *y), (track_x + fill_w, *y)).with_stops([
                (0.0_f32, to_vello_color(rgba, anim.opacity * 0.72)),
                (1.0_f32, col),
            ]);
            scene.fill(Fill::NonZero, aff, &bgrad, None, &bar);
            draw_text_block(
                scene,
                &fonts.display.font_data,
                value_text,
                *value_size,
                track_x + *bar_width * fraction.clamp(0.0, 1.0) + 22.0,
                *y + (*height - *value_size) * 0.34,
                240.0,
                TextAlign::Left,
                col,
                aff,
                0.5,
                1.0,
            );
        }
        LayerKind::Captions {
            items,
            y,
            font_size,
        } => {
            if let Some(item) = items.iter().find(|it| t >= it.start && t <= it.end) {
                let font = &fonts.sans.font_data;
                let pad_x = 28.0_f32;
                let pad_y = 11.0_f32;
                let space_w = measure_line_width(font, " ", *font_size, 0.4);
                let total_tw = if item.words.is_empty() {
                    measure_line_width(font, &item.text, *font_size, 0.4)
                } else {
                    let mut wsum = 0.0_f32;
                    for (wi, wd) in item.words.iter().enumerate() {
                        if wi > 0 {
                            wsum += space_w;
                        }
                        wsum += measure_line_width(font, &wd.text, *font_size, 0.4);
                    }
                    wsum
                };
                let rw = (total_tw + pad_x * 2.0).min(canvas_w - 120.0);
                let rh = *font_size * 1.24 + pad_y * 2.0;
                let rx = (canvas_w - rw) * 0.5;
                let rr = RoundedRect::new(
                    rx as f64,
                    *y as f64,
                    (rx + rw) as f64,
                    (*y + rh) as f64,
                    14.0,
                );
                scene.fill(
                    Fill::NonZero,
                    Affine::IDENTITY,
                    Color::from_rgba8(7, 9, 14, (224.0 * anim.opacity) as u8),
                    None,
                    &rr,
                );
                scene.stroke(
                    &Stroke::new(1.2),
                    Affine::IDENTITY,
                    Color::from_rgba8(255, 255, 255, (28.0 * anim.opacity) as u8),
                    None,
                    &rr,
                );
                if item.words.is_empty() || total_tw > canvas_w - 180.0 {
                    draw_text_block(
                        scene,
                        font,
                        &item.text,
                        *font_size,
                        rx + pad_x,
                        *y + pad_y,
                        rw - pad_x * 2.0,
                        TextAlign::Center,
                        Color::from_rgba8(255, 255, 255, (255.0 * anim.opacity) as u8),
                        Affine::IDENTITY,
                        0.4,
                        1.15,
                    );
                } else {
                    let mut pen_x = (canvas_w - total_tw) * 0.5;
                    for (wi, wd) in item.words.iter().enumerate() {
                        if wi > 0 {
                            pen_x += space_w;
                        }
                        let w_width = measure_line_width(font, &wd.text, *font_size, 0.4);
                        let is_active = t >= wd.start && t <= wd.end + 0.04;
                        let is_spoken = t > wd.end + 0.04;
                        let col = if is_active {
                            Color::from_rgba8(250, 204, 21, (255.0 * anim.opacity) as u8)
                        } else if is_spoken {
                            Color::from_rgba8(255, 255, 255, (255.0 * anim.opacity) as u8)
                        } else {
                            Color::from_rgba8(255, 255, 255, (168.0 * anim.opacity) as u8)
                        };
                        draw_text_block(
                            scene,
                            font,
                            &wd.text,
                            *font_size,
                            pen_x,
                            *y + pad_y,
                            w_width + 20.0,
                            TextAlign::Left,
                            col,
                            Affine::IDENTITY,
                            0.4,
                            1.15,
                        );
                        pen_x += w_width;
                    }
                }
            }
        }
        LayerKind::Arrow {
            x1,
            y1,
            x2,
            y2,
            stroke_width,
            head_size,
            color,
            anim_duration,
        } => {
            let u = if *anim_duration > 0.0 {
                ((t - layer.start) / *anim_duration).clamp(0.0, 1.0) as f32
            } else {
                1.0
            };
            let p = ease(crate::schema::EaseKind::Power3Out, u);
            if p > 0.01 {
                let cx = ((*x1 + *x2) * 0.5) as f64;
                let cy = ((*y1 + *y2) * 0.5) as f64;
                let aff = layer_affine(cam_aff, anim, cx, cy);
                let dx = (*x2 - *x1) as f64;
                let dy = (*y2 - *y1) as f64;
                let len = dx.hypot(dy).max(1.0);
                let ux = dx / len;
                let uy = dy / len;
                let cur_x = *x1 as f64 + dx * p as f64;
                let cur_y = *y1 as f64 + dy * p as f64;
                let head_len = (*head_size as f64).min(len * p as f64 * 0.65);
                let shaft_end_x = cur_x - ux * head_len * 0.72;
                let shaft_end_y = cur_y - uy * head_len * 0.72;
                let col = to_vello_color(Rgba::parse(color)?, anim.opacity);
                let line = KurboLine::new((*x1 as f64, *y1 as f64), (shaft_end_x, shaft_end_y));
                scene.stroke(
                    &Stroke::new(*stroke_width as f64).with_caps(Cap::Round),
                    aff,
                    col,
                    None,
                    &line,
                );
                let px_perp = -uy;
                let py_perp = ux;
                let base_x = cur_x - ux * head_len;
                let base_y = cur_y - uy * head_len;
                let half_w = head_len * 0.58;
                let mut tri = BezPath::new();
                tri.move_to(Point::new(cur_x, cur_y));
                tri.line_to(Point::new(
                    base_x + px_perp * half_w,
                    base_y + py_perp * half_w,
                ));
                tri.line_to(Point::new(
                    base_x - px_perp * half_w,
                    base_y - py_perp * half_w,
                ));
                tri.close_path();
                scene.fill(Fill::NonZero, aff, col, None, &tri);
            }
        }
        LayerKind::FloorGlow {
            cx,
            cy,
            rx,
            ry,
            color,
            intensity,
        } => {
            let aff = layer_affine(cam_aff, anim, *cx as f64, *cy as f64);
            let rgba = Rgba::parse(color)?;
            let steps = 5;
            for i in 0..steps {
                let frac = 1.0 - (i as f32 / steps as f32) * 0.68;
                let alpha = anim.opacity * (*intensity) * (0.28 + 0.18 * (i as f32 / steps as f32));
                let ell_aff = aff
                    * Affine::translate((*cx as f64, *cy as f64))
                    * Affine::scale_non_uniform((*rx * frac) as f64, (*ry * frac) as f64);
                let unit_circle = Circle::new((0.0, 0.0), 1.0);
                scene.fill(
                    Fill::NonZero,
                    ell_aff,
                    to_vello_color(rgba, alpha),
                    None,
                    &unit_circle,
                );
            }
        }
    }
    Ok(())
}
