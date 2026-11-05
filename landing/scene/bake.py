"""Bake puesto07.blend into texture atlases and export a GLB for three.js.

Run: blender -b puesto07.blend -P bake.py -- [--samples N] [--only atlas,atlas] [--scale 0.5] [--export]
"""
import bpy, bmesh, math, os, sys
from mathutils import Vector

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "bake")
ARGS = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []


def arg(name, default):
    return ARGS[ARGS.index(name) + 1] if name in ARGS else default


SAMPLES = int(arg("--samples", "512"))
ONLY = arg("--only", None)
SCALE = float(arg("--scale", "1"))
os.makedirs(OUT, exist_ok=True)
scene = bpy.context.scene

ATLAS_SIZE = {"room": 4096, "desk": 2048, "desktop": 2048, "monitor": 4096, "far": 2048, "tower": 2048, "cd": 1024,
              "keyboard": 2048, "props": 2048, "paper": 2048, "poster": 1024}
UNBAKED_PREFIX = ("led", "pled", "hled", "kb_led", "webcam_led", "neon_sign", "clock_hour", "clock_min", "clock_sec", "clock_pin")


def root_of(o):
    while o.parent is not None:
        o = o.parent
    return o.name


for o in list(bpy.data.objects):
    if o.name.startswith("haze") or o.type == "CAMERA" or (o.type == "MESH" and o.hide_render and not o.name.startswith("screen_cut")):
        bpy.data.objects.remove(o, do_unlink=True)

roots = {o.name: root_of(o) for o in bpy.data.objects}
base = {o.name: o.name.split(".")[0] for o in bpy.data.objects}
bpy.ops.object.select_all(action="DESELECT")
for o in bpy.data.objects:
    if o.type in {"MESH", "CURVE", "FONT"}:
        o.select_set(True)
bpy.context.view_layer.objects.active = next(o for o in bpy.data.objects if o.type == "MESH")
bpy.ops.object.make_single_user(object=True, obdata=True)
bpy.ops.object.convert(target="MESH")
info = {}
for o in bpy.data.objects:
    info[o] = (roots.get(o.name, o.name), base.get(o.name, o.name))
for o in [o for o in bpy.data.objects if o.name.startswith("screen_cut")]:
    bpy.data.objects.remove(o, do_unlink=True)
bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.parent_clear(type="CLEAR_KEEP_TRANSFORM")
for o in [o for o in bpy.data.objects if o.type == "EMPTY"]:
    bpy.data.objects.remove(o, do_unlink=True)

# Every camera sits in front of the desk (y < -0.15) and between z 0.95 and 1.5, so faces turned
# away from that box never show; dropping them gives the visible faces more texels.
for o in [o for o in bpy.data.objects if o.type == "MESH"]:
    name = info[o][1]
    if name == "screen" or name.startswith(UNBAKED_PREFIX):
        continue
    bm = bmesh.new()
    bm.from_mesh(o.data)
    mw = o.matrix_world
    rot = mw.to_3x3().normalized()
    dead = []
    for f in bm.faces:
        n = rot @ f.normal
        c = mw @ f.calc_center_median()
        if (n.z < -0.5 and c.z < 0.95) or (n.y > 0.6 and c.y > -0.15) or (n.z > 0.6 and c.z > 1.55):
            dead.append(f)
    if dead and len(dead) < len(bm.faces):
        bmesh.ops.delete(bm, geom=dead, context="FACES")
        bm.to_mesh(o.data)
    bm.free()

# The desk top inside the booth gets its own atlas with a flat top-down UV.
DESKTOP = (-0.82, 0.82, -0.505, 0.365)
desk = next(o for o in bpy.data.objects if info.get(o, ("", ""))[1] == "desk")
bm = bmesh.new()
bm.from_mesh(desk.data)
loc = desk.matrix_world.translation
for x in (DESKTOP[0], DESKTOP[1]):
    bmesh.ops.bisect_plane(bm, geom=bm.verts[:] + bm.edges[:] + bm.faces[:], plane_co=(x - loc.x, 0, 0), plane_no=(1, 0, 0))
top = [f for f in bm.faces if f.normal.z > 0.9 and abs(f.calc_center_median().x + loc.x) < DESKTOP[1]]
top_mesh = bpy.data.meshes.new("desktop")
bm_top = bmesh.new()
vmap = {}
for f in top:
    vs = []
    for v in f.verts:
        if v not in vmap:
            vmap[v] = bm_top.verts.new(v.co)
        vs.append(vmap[v])
    bm_top.faces.new(vs)
uvl = bm_top.loops.layers.uv.new("UVMap")
for f in bm_top.faces:
    for l in f.loops:
        wx, wy = l.vert.co.x + loc.x, l.vert.co.y + loc.y
        l[uvl].uv = ((wx - DESKTOP[0]) / (DESKTOP[1] - DESKTOP[0]), (wy - DESKTOP[2]) / (DESKTOP[3] - DESKTOP[2]))
bm_top.to_mesh(top_mesh)
bm_top.free()
bmesh.ops.delete(bm, geom=top, context="FACES")
bm.to_mesh(desk.data)
bm.free()
desktop = bpy.data.objects.new("desktop", top_mesh)
scene.collection.objects.link(desktop)
desktop.matrix_world = desk.matrix_world.copy()
for m in desk.data.materials:
    top_mesh.materials.append(m)
info[desktop] = ("desk", "desktop")


def classify(o):
    root, name = info[o]
    if name.startswith(UNBAKED_PREFIX):
        return "unbaked"
    if name == "screen":
        return "screen"
    if root == "monitor":
        return "monitor"
    if root.startswith(("far_crt", "player", "chair")) or name.startswith("webcam"):
        return "far"
    if root == "tower":
        return "cd" if name.startswith(("cd", "tray")) else "tower"
    if root == "keyboard":
        return "keyboard"
    if root in ("notepad", "pen") or name in ("ticket", "coin"):
        return "paper"
    if root == "headset" or name in ("mouse", "pad_base", "pad_top", "cable", "can", "can_top"):
        return "props"
    if name == "desktop":
        return "desktop"
    if name in ("desk", "desk_edge", "desk_back", "divider"):
        return "desk"
    if name == "poster":
        return "poster"
    return "room"


groups = {}
for o in bpy.data.objects:
    if o.type == "MESH":
        groups.setdefault(classify(o), []).append(o)
for k, v in sorted(groups.items()):
    print("group", k, len(v))


def join(objs, name):
    bpy.ops.object.select_all(action="DESELECT")
    for o in objs:
        o.select_set(True)
    bpy.context.view_layer.objects.active = objs[0]
    if len(objs) > 1:
        bpy.ops.object.join()
    j = bpy.context.view_layer.objects.active
    j.name = name
    j.data.name = name
    return j


# Cycles bakes each selected object in its own pass, so one joined mesh per atlas is far faster.
# The paper atlas keeps the notepad and the ticket apart so each can light up on hover.
for g, objs in list(groups.items()):
    if g in ("unbaked", "screen"):
        continue
    if g == "paper":
        note = [o for o in objs if info[o][0] in ("notepad", "pen")]
        tick = [o for o in objs if o not in note]
        groups[g] = [join(note, "notepad"), join(tick, "ticket")]
    else:
        groups[g] = [join(objs, g)]

scene.render.engine = "CYCLES"
cp = bpy.context.preferences.addons["cycles"].preferences
cp.compute_device_type = "OPTIX"
cp.get_devices()
for d in cp.devices:
    d.use = d.type == "OPTIX"
scene.cycles.device = "GPU"
scene.cycles.samples = SAMPLES
scene.render.bake.margin = 12
scene.render.bake.margin_type = "EXTEND"
try:
    scene.render.bake.view_from = "ACTIVE_CAMERA"
except Exception as e:
    print("view_from unsupported", e)
cam = bpy.data.cameras.new("bake_view")
cam_o = bpy.data.objects.new("bake_view", cam)
scene.collection.objects.link(cam_o)
cam_o.location = (0.0, -1.22, 1.26)
cam_o.rotation_euler = (Vector((0, 0, 1.02)) - Vector(cam_o.location)).to_track_quat("-Z", "Y").to_euler()
scene.camera = cam_o


def unwrap(objs, use_existing):
    bpy.ops.object.select_all(action="DESELECT")
    for o in objs:
        o.select_set(True)
        me = o.data
        lay = me.uv_layers.get("bake")
        if lay is None:
            src = me.uv_layers.get("UVMap")
            lay = me.uv_layers.new(name="bake")
            if use_existing and src is not None:
                for a, b in zip(lay.data, src.data):
                    a.uv = b.uv
        me.uv_layers.active = lay
        if me.uv_layers.get("UVMap") is not None:
            me.uv_layers["UVMap"].active_render = True
    bpy.context.view_layer.objects.active = objs[0]
    if use_existing:
        return
    bpy.ops.object.mode_set(mode="EDIT")
    bpy.ops.mesh.select_all(action="SELECT")
    bpy.ops.uv.smart_project(angle_limit=math.radians(60), island_margin=0.0025, area_weight=0.0, scale_to_bounds=False)
    bpy.ops.uv.pack_islands(margin=0.0025, rotate=True)
    bpy.ops.object.mode_set(mode="OBJECT")


def bake(objs, size, out_name, w=None):
    w = w or size
    img = bpy.data.images.new("bake_" + out_name, int(w * SCALE), int(size * SCALE), float_buffer=True, alpha=False)
    img.colorspace_settings.name = "Linear Rec.709"
    mats = {s.material for o in objs for s in o.material_slots if s.material}
    for m in mats:
        nt = m.node_tree
        node = nt.nodes.get("BakeTarget") or nt.nodes.new("ShaderNodeTexImage")
        node.name = "BakeTarget"
        node.image = img
        nt.nodes.active = node
    bpy.ops.object.select_all(action="DESELECT")
    for o in objs:
        o.select_set(True)
    bpy.context.view_layer.objects.active = objs[0]
    bpy.ops.object.bake(type="COMBINED", use_clear=True, margin=12)
    img.filepath_raw = os.path.join(OUT, out_name + ".exr")
    img.file_format = "OPEN_EXR"
    img.save()
    scene.render.image_settings.file_format = "PNG"
    scene.render.image_settings.color_depth = "8"
    scene.view_settings.view_transform = "Khronos PBR Neutral"
    img.save_render(os.path.join(OUT, out_name + ".png"), scene=scene)
    print("baked", out_name, tuple(img.size))


for g in ATLAS_SIZE:
    if g in groups:
        unwrap(groups[g], use_existing=(g in ("poster", "desktop")))
        uvsum = sum(d.uv.x * 3.1 + d.uv.y for o in groups[g] for d in o.data.uv_layers["bake"].data)
        print("uvsum", g, round(uvsum, 4))
todo = [g for g in ATLAS_SIZE if g in groups and (ONLY is None or g in ONLY.split(","))]
for g in todo:
    objs = groups[g]
    if g == "poster":
        pm = objs[0].material_slots[0].material
        tex = next(n for n in pm.node_tree.nodes if n.type == "TEX_IMAGE" and n.name != "BakeTarget")
        for i, f in enumerate(("poster.png", "poster2.png", "poster3.png")):
            tex.image = bpy.data.images.load(os.path.join(HERE, "tex", f))
            bake(objs, 1024, "poster-%d" % (i + 1), 720)
        continue
    bake(objs, ATLAS_SIZE[g], g, 4096 if g == "desktop" else None)

if "--export" not in ARGS:
    sys.exit(0)


final = [(o, g) for g, objs in groups.items() if g not in ("unbaked", "screen") for o in objs]
for o in groups.get("screen", []):
    root = info[o][0]
    o.name = "screen_main" if root == "monitor" else "screen_" + root.replace("far_crt_", "far")
for o in groups.get("unbaked", []):
    o.name = info[o][1]
for o, g in final:
    keep = "bake"
    for lay in [l for l in o.data.uv_layers if l.name != keep]:
        o.data.uv_layers.remove(lay)
    o.data.materials.clear()
    m = bpy.data.materials.get("atlas_" + g) or bpy.data.materials.new("atlas_" + g)
    o.data.materials.append(m)
for o in [o for o in bpy.data.objects if o.type in ("LIGHT", "CAMERA")]:
    bpy.data.objects.remove(o, do_unlink=True)
bpy.ops.export_scene.gltf(filepath=os.path.join(OUT, "puesto07.glb"), export_format="GLB", export_texcoords=True,
                          export_normals=False, export_image_format="NONE", export_cameras=False, export_lights=False,
                          export_yup=True, export_apply=True)
print("exported glb")
