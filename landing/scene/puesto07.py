"""Puesto 07: a cybercafe desk built from primitives, rendered with Cycles.

Run: blender -b --factory-startup -P puesto07.py -- [--quick] [--only NAME]
"""
import bpy, bmesh, math, os, sys
from mathutils import Vector

HERE = os.path.dirname(os.path.abspath(__file__))
TEX = os.path.join(HERE, "tex")
OUT = os.path.join(HERE, "renders")
ARGS = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
QUICK = "--quick" in ARGS
ONLY = ARGS[ARGS.index("--only") + 1] if "--only" in ARGS else None
DESK = 0.75

scene = bpy.context.scene
for o in list(bpy.data.objects):
    bpy.data.objects.remove(o, do_unlink=True)


def setin(node, name, value):
    s = node.inputs.get(name)
    if s is not None:
        s.default_value = value


def principled(name, color=(0.8, 0.8, 0.8), rough=0.5, metal=0.0, extra=None):
    m = bpy.data.materials.new(name)
    try:
        m.use_nodes = True
    except Exception:
        pass
    nt = m.node_tree
    p = next(n for n in nt.nodes if n.type == "BSDF_PRINCIPLED")
    setin(p, "Base Color", (*color, 1))
    setin(p, "Roughness", rough)
    setin(p, "Metallic", metal)
    for k, v in (extra or {}).items():
        setin(p, k, v)
    return m, nt, p


def obj_coords(nt):
    tc = nt.nodes.new("ShaderNodeTexCoord")
    return tc.outputs["Object"]


def add_bump(nt, p, scale, strength, distance=0.0004):
    n = nt.nodes.new("ShaderNodeTexNoise")
    n.inputs["Scale"].default_value = scale
    n.inputs["Detail"].default_value = 3
    nt.links.new(obj_coords(nt), n.inputs["Vector"])
    b = nt.nodes.new("ShaderNodeBump")
    b.inputs["Strength"].default_value = strength
    b.inputs["Distance"].default_value = distance
    nt.links.new(n.outputs["Fac"], b.inputs["Height"])
    nt.links.new(b.outputs["Normal"], p.inputs["Normal"])


def vary_rough(nt, p, lo, hi, scale=6):
    n = nt.nodes.new("ShaderNodeTexNoise")
    n.inputs["Scale"].default_value = scale
    nt.links.new(obj_coords(nt), n.inputs["Vector"])
    mr = nt.nodes.new("ShaderNodeMapRange")
    mr.inputs["To Min"].default_value = lo
    mr.inputs["To Max"].default_value = hi
    nt.links.new(n.outputs["Fac"], mr.inputs["Value"])
    nt.links.new(mr.outputs["Result"], p.inputs["Roughness"])


def plastic(name, color, rough=0.42, bump=0.12):
    m, nt, p = principled(name, color, rough)
    add_bump(nt, p, 1400, bump)
    vary_rough(nt, p, rough - 0.08, rough + 0.12)
    return m


def image_mat(name, file, rough=0.6, metal=0.0, emission=0.0):
    m, nt, p = principled(name, (1, 1, 1), rough, metal)
    t = nt.nodes.new("ShaderNodeTexImage")
    t.image = bpy.data.images.load(os.path.join(TEX, file))
    uvn = nt.nodes.new("ShaderNodeUVMap")
    uvn.uv_map = "UVMap"
    nt.links.new(uvn.outputs["UV"], t.inputs["Vector"])
    nt.links.new(t.outputs["Color"], p.inputs["Base Color"])
    if emission:
        nt.links.new(t.outputs["Color"], p.inputs["Emission Color"])
        p.inputs["Emission Strength"].default_value = emission
    return m, nt, p, t


def emissive(name, color, strength):
    m, nt, p = principled(name, (0, 0, 0), 0.4)
    setin(p, "Emission Color", (*color, 1))
    setin(p, "Emission Strength", strength)
    return m


def smooth(o):
    for poly in o.data.polygons:
        poly.use_smooth = True


def bevel(o, width, segments=3):
    md = o.modifiers.new("Bevel", "BEVEL")
    md.width = width
    md.segments = segments
    md.limit_method = "ANGLE"
    md.harden_normals = True
    smooth(o)


def finish(o, name, mat, parent):
    o.name = name
    if mat:
        o.data.materials.append(mat)
    if parent:
        o.parent = parent
    return o


def box(name, size, loc, mat=None, bev=0.0, rot=(0, 0, 0), parent=None, seg=3):
    bpy.ops.mesh.primitive_cube_add(size=1, location=(0, 0, 0))
    o = bpy.context.active_object
    for v in o.data.vertices:
        v.co = Vector((v.co.x * size[0], v.co.y * size[1], v.co.z * size[2]))
    o.location = loc
    o.rotation_euler = rot
    if bev:
        bevel(o, bev, seg)
    return finish(o, name, mat, parent)


def cyl(name, r, depth, loc, mat=None, rot=(0, 0, 0), verts=48, bev=0.0, parent=None):
    bpy.ops.mesh.primitive_cylinder_add(vertices=verts, radius=r, depth=depth, location=(0, 0, 0))
    o = bpy.context.active_object
    o.location = loc
    o.rotation_euler = rot
    if bev:
        bevel(o, bev, 2)
    else:
        smooth_sides(o)
    return finish(o, name, mat, parent)


def smooth_sides(o):
    for poly in o.data.polygons:
        poly.use_smooth = abs(poly.normal.z) < 0.5


def plane(name, w, h, loc, mat, rot=(math.pi / 2, 0, 0), parent=None):
    bpy.ops.mesh.primitive_plane_add(size=1, location=(0, 0, 0))
    o = bpy.context.active_object
    for v in o.data.vertices:
        v.co = Vector((v.co.x * w, v.co.y * h, 0))
    o.location = loc
    o.rotation_euler = rot
    return finish(o, name, mat, parent)


def text(name, body, size, loc, mat, rot=(math.pi / 2, 0, 0), extrude=0.0, parent=None, align="CENTER"):
    cu = bpy.data.curves.new(name, "FONT")
    cu.body = body
    cu.size = size
    cu.extrude = extrude
    cu.align_x = align
    cu.align_y = "CENTER"
    o = bpy.data.objects.new(name, cu)
    scene.collection.objects.link(o)
    o.location = loc
    o.rotation_euler = rot
    cu.materials.append(mat)
    if parent:
        o.parent = parent
    return o


def outward(bm):
    """Point the normals of a lathed or swept mesh away from its middle."""
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
    mid = sum((v.co for v in bm.verts), Vector()) / max(1, len(bm.verts))
    if sum(f.normal.dot(f.calc_center_median() - mid) * f.calc_area() for f in bm.faces) < 0:
        bmesh.ops.reverse_faces(bm, faces=bm.faces)


def mesh_object(name, bm, mat, parent):
    me = bpy.data.meshes.new(name)
    bm.to_mesh(me)
    bm.free()
    o = bpy.data.objects.new(name, me)
    scene.collection.objects.link(o)
    smooth(o)
    return finish(o, name, mat, parent)


def lathe(name, prof, mat=None, parent=None, steps=64):
    """Spin a profile of (radius, z) points around the z axis."""
    bm = bmesh.new()
    vs = [bm.verts.new((r, 0, z)) for r, z in prof]
    es = [bm.edges.new((vs[i], vs[i + 1])) for i in range(len(vs) - 1)]
    bmesh.ops.spin(bm, geom=vs + es, cent=(0, 0, 0), axis=(0, 0, 1), angle=2 * math.pi, steps=steps)
    bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-7)
    outward(bm)
    return mesh_object(name, bm, mat, parent)


def rrect(w, h, r, n=3):
    """Rounded rectangle section, w across the normal and h along it."""
    pts = []
    for cx, cy, a0 in ((w / 2 - r, h / 2 - r, 0), (-w / 2 + r, h / 2 - r, 90), (-w / 2 + r, -h / 2 + r, 180), (w / 2 - r, -h / 2 + r, 270)):
        for i in range(n + 1):
            a = math.radians(a0 + 90 * i / n)
            pts.append((cy + r * math.sin(a), cx + r * math.cos(a)))
    return pts


def sweep(name, path, section, mat=None, parent=None):
    """Sweep a closed section of (along normal, along binormal) points along (point, normal, binormal)."""
    bm = bmesh.new()
    rings = [[bm.verts.new(p + nrm * u + bin_ * v) for u, v in section] for p, nrm, bin_ in path]
    k = len(section)
    for r0, r1 in zip(rings, rings[1:]):
        for i in range(k):
            bm.faces.new((r0[i], r0[(i + 1) % k], r1[(i + 1) % k], r1[i]))
    bm.faces.new(rings[0])
    bm.faces.new(rings[-1])
    outward(bm)
    return mesh_object(name, bm, mat, parent)


def arc_path(a, b, zc, t0, t1, n=48):
    """Half ellipse in the xz plane, from angle t0 to t1, with outward normals and +y binormals."""
    out = []
    for i in range(n + 1):
        t = t0 + (t1 - t0) * i / n
        p = Vector((a * math.cos(t), 0, zc + b * math.sin(t)))
        nrm = Vector((b * math.cos(t), 0, a * math.sin(t))).normalized()
        out.append((p, nrm, Vector((0, 1, 0))))
    return out


def empty(name, loc, rot=(0, 0, 0)):
    o = bpy.data.objects.new(name, None)
    scene.collection.objects.link(o)
    o.location = loc
    o.rotation_euler = rot
    return o


# Materials
beige = plastic("beige", (0.60, 0.55, 0.45))
beige_dark = plastic("beige_dark", (0.47, 0.43, 0.35), 0.5)
key_mat = plastic("key", (0.66, 0.62, 0.53), 0.38, 0.06)
key_worn = plastic("key_worn", (0.55, 0.52, 0.45), 0.16, 0.03)
black_plastic = plastic("black_plastic", (0.025, 0.025, 0.028), 0.5)
foam_m, foam_nt, foam_p = principled("foam", (0.02, 0.02, 0.022), 0.95)
add_bump(foam_nt, foam_p, 600, 0.6)
dark_grey = plastic("dark_grey", (0.06, 0.06, 0.065), 0.45)
legend = principled("legend", (0.08, 0.08, 0.08), 0.5)[0]
marker = principled("marker", (0.01, 0.01, 0.012), 0.35)[0]
alu = principled("alu", (0.8, 0.8, 0.82), 0.22, 1.0)[0]
coin_mat = principled("coin", (0.55, 0.4, 0.17), 0.38, 1.0)[0]
led_green = emissive("led_green", (0.2, 1.0, 0.3), 12)
led_amber = emissive("led_amber", (1.0, 0.55, 0.1), 6)

wall_m, wall_nt, wall_p = principled("wall", (0.13, 0.19, 0.17), 0.85)
add_bump(wall_nt, wall_p, 180, 0.25, 0.002)
melamine_m, mel_nt, mel_p = principled("melamine", (0.30, 0.29, 0.27), 0.5)
add_bump(mel_nt, mel_p, 500, 0.05)
floor_m = principled("floor", (0.05, 0.045, 0.04), 0.6)[0]

desk_m, dnt, dp = principled("desk", (0.5, 0.33, 0.2), 0.42)
wave = dnt.nodes.new("ShaderNodeTexWave")
wave.inputs["Scale"].default_value = 3.0
wave.inputs["Distortion"].default_value = 6.0
wave.inputs["Detail"].default_value = 4.0
mapping = dnt.nodes.new("ShaderNodeMapping")
mapping.inputs["Scale"].default_value = (0.25, 4.0, 1.0)
dnt.links.new(obj_coords(dnt), mapping.inputs["Vector"])
dnt.links.new(mapping.outputs["Vector"], wave.inputs["Vector"])
ramp = dnt.nodes.new("ShaderNodeValToRGB")
ramp.color_ramp.elements[0].color = (0.16, 0.08, 0.04, 1)
ramp.color_ramp.elements[1].color = (0.32, 0.18, 0.09, 1)
dnt.links.new(wave.outputs["Fac"], ramp.inputs["Fac"])
dnt.links.new(ramp.outputs["Color"], dp.inputs["Base Color"])
vary_rough(dnt, dp, 0.28, 0.5, 3)
edge_m = principled("desk_edge", (0.05, 0.035, 0.025), 0.45)[0]

# Room
plane("floor", 6, 5, (0, 0.3, 0), floor_m, rot=(0, 0, 0))
box("desk", (3.4, 0.86, 0.03), (0, -0.07, DESK - 0.015), desk_m, 0.002)
box("desk_edge", (3.4, 0.006, 0.032), (0, -0.503, DESK - 0.015), edge_m, 0.001)
box("desk_back", (3.4, 0.018, 0.09), (0, 0.36, DESK + 0.045), melamine_m, 0.004)
for x in (-0.79, 0.79):
    box("divider", (0.018, 0.84, 0.46), (x, -0.06, DESK + 0.23), melamine_m, 0.006)

# CRT monitor
def make_crt(name, loc, tex, emission=2.6, light_energy=5, light_color=(1.0, 0.82, 0.62), postit=False):
    mon = empty(name, loc)
    cyl("stand_base", 0.12, 0.016, (0, 0.05, DESK + 0.008), beige_dark, bev=0.004, parent=mon)
    box("stand_neck", (0.09, 0.09, 0.05), (0, 0.06, DESK + 0.035), beige_dark, 0.01, parent=mon)
    bz = box("bezel", (0.43, 0.075, 0.40), (0, 0, DESK + 0.255), beige, 0.012, parent=mon)
    cut = box("screen_cut", (0.335, 0.05, 0.255), (0, -0.05, DESK + 0.27), None, parent=mon)
    cut.hide_render = True
    cut.display_type = "WIRE"
    bm = bz.modifiers.new("Recess", "BOOLEAN")
    bm.operation = "DIFFERENCE"
    bm.object = cut
    bz.modifiers.move(1, 0)

    bpy.ops.mesh.primitive_cube_add(size=1, location=(0, 0, 0))
    house = bpy.context.active_object
    for v in house.data.vertices:
        x, y, z = v.co.x * 0.40, v.co.y * 0.32, v.co.z * 0.36
        if y > 0:
            x *= 0.62
            z = z * 0.6 + 0.02
        v.co = Vector((x, y, z))
    house.location = (0, 0.19, DESK + 0.255)
    bevel(house, 0.02, 4)
    finish(house, "housing", beige, mon)
    for i in range(7):
        box("vent", (0.20, 0.003, 0.004), (0, 0.28, DESK + 0.40 - i * 0.012), black_plastic, parent=mon)

    screen_m, snt, sp = principled(name + "_screen", (0.004, 0.005, 0.005), 0.16, extra={"Specular IOR Level": 0.18})
    img = snt.nodes.new("ShaderNodeTexImage")
    img.image = bpy.data.images.load(os.path.join(TEX, tex))
    uv = snt.nodes.new("ShaderNodeTexCoord")
    sep = snt.nodes.new("ShaderNodeSeparateXYZ")
    snt.links.new(uv.outputs["UV"], sep.inputs["Vector"])
    lines = snt.nodes.new("ShaderNodeMath")
    lines.operation = "MULTIPLY"
    lines.inputs[1].default_value = 380 * math.pi * 2
    snt.links.new(sep.outputs["Y"], lines.inputs[0])
    sine = snt.nodes.new("ShaderNodeMath")
    sine.operation = "SINE"
    snt.links.new(lines.outputs[0], sine.inputs[0])
    scan = snt.nodes.new("ShaderNodeMapRange")
    scan.inputs["From Min"].default_value = -1
    scan.inputs["To Min"].default_value = 0.72
    scan.inputs["To Max"].default_value = 1.0
    snt.links.new(sine.outputs[0], scan.inputs["Value"])
    dist = snt.nodes.new("ShaderNodeVectorMath")
    dist.operation = "DISTANCE"
    dist.inputs[1].default_value = (0.5, 0.5, 0)
    snt.links.new(uv.outputs["UV"], dist.inputs[0])
    vig = snt.nodes.new("ShaderNodeMapRange")
    vig.inputs["From Min"].default_value = 0.32
    vig.inputs["From Max"].default_value = 0.72
    vig.inputs["To Min"].default_value = 1.0
    vig.inputs["To Max"].default_value = 0.15
    snt.links.new(dist.outputs["Value"], vig.inputs["Value"])
    mul1 = snt.nodes.new("ShaderNodeVectorMath")
    mul1.operation = "MULTIPLY"
    snt.links.new(img.outputs["Color"], mul1.inputs[0])
    snt.links.new(scan.outputs["Result"], mul1.inputs[1])
    mul2 = snt.nodes.new("ShaderNodeVectorMath")
    mul2.operation = "MULTIPLY"
    snt.links.new(mul1.outputs["Vector"], mul2.inputs[0])
    snt.links.new(vig.outputs["Result"], mul2.inputs[1])
    snt.links.new(mul2.outputs["Vector"], sp.inputs["Emission Color"])
    sp.inputs["Emission Strength"].default_value = emission

    bpy.ops.mesh.primitive_grid_add(x_subdivisions=48, y_subdivisions=36, size=1, location=(0, 0, 0))
    scr = bpy.context.active_object
    for v in scr.data.vertices:
        x, y = v.co.x, v.co.y
        v.co = Vector((x * 0.318, y * 0.24, 0.014 * (1 - (2 * x) ** 2 * 0.6) * (1 - (2 * y) ** 2 * 0.6)))
    scr.location = (0, -0.028, DESK + 0.27)
    scr.rotation_euler = (math.pi / 2, 0, 0)
    smooth(scr)
    finish(scr, "screen", screen_m, mon)
    box("screen_mask", (0.34, 0.004, 0.262), (0, -0.017, DESK + 0.27), black_plastic, parent=mon)

    text("brand", "VISTAMAX 17", 0.013, (-0.13, -0.0385, DESK + 0.085), beige_dark, extrude=0.0008, parent=mon)
    cyl("led", 0.0028, 0.004, (0.175, -0.039, DESK + 0.085), led_green, rot=(math.pi / 2, 0, 0), verts=16, parent=mon)
    for i in range(4):
        box("btn", (0.014, 0.004, 0.007), (0.06 + i * 0.022, -0.039, DESK + 0.085), beige_dark, 0.001, parent=mon)
    if postit:
        postit_m = image_mat("postit", "postit.png", 0.8)[0]
    if postit:
        plane("postit", 0.072, 0.072, (0.175, -0.0395, DESK + 0.405), postit_m, rot=(math.pi / 2, math.radians(-7), 0), parent=mon)

    screen_light = bpy.data.lights.new("screen_light", "AREA")
    screen_light.shape = "RECTANGLE"
    screen_light.size = 0.30
    screen_light.size_y = 0.22
    screen_light.energy = light_energy
    screen_light.color = light_color
    sl = bpy.data.objects.new("screen_light", screen_light)
    scene.collection.objects.link(sl)
    sl.parent = mon
    sl.location = (0, -0.06, DESK + 0.27)
    sl.rotation_euler = (math.pi / 2, 0, 0)
    sl.visible_camera = False
    sl.visible_glossy = False
    sl.visible_volume_scatter = False

    return mon, scr


CRT_TEX = ARGS[ARGS.index("--crt") + 1] if "--crt" in ARGS else "screen-jugada.png"
mon, scr = make_crt("monitor", (0, -0.06, 0), CRT_TEX, 1.6, 6, (1.0, 0.8, 0.58), postit=True)

# The row behind: two players with headsets, backlit by their own CRTs
far_y = 1.55
box("far_desk", (4.0, 0.8, 0.03), (0, far_y + 0.1, DESK - 0.015), desk_m, 0.002)
cloth = principled("cloth", (0.018, 0.018, 0.022), 0.9)[0]
skin, hair_nt, hair_p = principled("hair", (0.016, 0.012, 0.010), 0.55)
add_bump(hair_nt, hair_p, 90, 0.7, 0.004)
for i, (x, tex, col) in enumerate(((-0.92, "screen3.png", (0.65, 0.45, 1.0)), (0.0, "screen2.png", (1.0, 0.8, 0.6)), (0.95, "screen4.png", (0.75, 0.62, 1.0)))):
    make_crt("far_crt_%d" % i, (x, far_y, 0), tex, 1.8, 9, col)
mesh_m = principled("chair_mesh", (0.012, 0.012, 0.014), 0.8)[0]
add_bump(mesh_m.node_tree, next(n for n in mesh_m.node_tree.nodes if n.type == "BSDF_PRINCIPLED"), 900, 0.5)
hand_m = principled("hand_skin", (0.42, 0.26, 0.19), 0.6)[0]


cuff_m = (principled("cuff_a", (0.025, 0.04, 0.09), 0.95)[0], principled("cuff_b", (0.09, 0.03, 0.025), 0.95)[0])


def spline(ctrl, n):
    """Catmull-Rom through the control points, n samples per span."""
    c = [ctrl[0]] + list(ctrl) + [ctrl[-1]]
    out = []
    for k in range(1, len(c) - 2):
        p0, p1, p2, p3 = c[k - 1], c[k], c[k + 1], c[k + 2]
        for j in range(n):
            t = j / n
            out.append(0.5 * (2 * p1 + (p2 - p0) * t + (2 * p0 - 5 * p1 + 4 * p2 - p3) * t * t + (3 * p1 - p0 - 3 * p2 + p3) * t ** 3))
    out.append(ctrl[-1])
    return out


def tube(name, pts, radius, mat, parent, sides=24, flat=1.0):
    """Smooth tube along pts with parallel-transported frames; radius(t) for t in 0..1, flat squashes one axis."""
    bm = bmesh.new()
    tan = [(pts[min(k + 1, len(pts) - 1)] - pts[max(k - 1, 0)]).normalized() for k in range(len(pts))]
    nrm = tan[0].orthogonal().normalized()
    rings = []
    for k, (p, tg) in enumerate(zip(pts, tan)):
        nrm = (nrm - tg * nrm.dot(tg)).normalized()
        bi = tg.cross(nrm)
        r = radius(k / (len(pts) - 1))
        rings.append([bm.verts.new(p + (nrm * math.cos(a) * flat + bi * math.sin(a)) * r) for a in (2 * math.pi * j / sides for j in range(sides))])
    for r0, r1 in zip(rings, rings[1:]):
        for j in range(sides):
            bm.faces.new((r0[j], r0[(j + 1) % sides], r1[(j + 1) % sides], r1[j]))
    bm.faces.new(rings[0])
    bm.faces.new(rings[-1])
    outward(bm)
    o = mesh_object(name, bm, mat, parent)
    smooth(o)
    return o


def make_hand(wrist, d, sx, parent):
    """Palm and four curled fingers reaching along d, thumb on the inner side."""
    side = Vector((0, 0, 1)).cross(d).normalized() * -sx
    down = Vector((0, 0, -1))
    palm = [wrist, wrist + d * 0.045 + down * 0.004, wrist + d * 0.085 + down * 0.01]
    tube("palm", palm, lambda t: 0.026 + 0.008 * math.sin(t * math.pi), hand_m, parent, sides=16, flat=0.45)
    for f, (off, ln) in enumerate(((-0.024, 0.05), (-0.008, 0.058), (0.008, 0.055), (0.023, 0.044))):
        base = palm[-1] + side * off
        tip = base + d * ln * 0.55 + down * ln * 0.65
        tube("finger", spline((base, base + d * ln * 0.45 + down * 0.008, tip), 4), lambda t: 0.0085 - 0.002 * t, hand_m, parent, sides=10)
    tb = wrist + d * 0.03 - side * 0.028 * sx * -1
    tube("thumb", spline((tb, tb + d * 0.035 - side * 0.012 * -sx + down * 0.012, tb + d * 0.06 + down * 0.025), 4), lambda t: 0.0105 - 0.002 * t, hand_m, parent, sides=10)


def ball(name, c, r, mat, parent):
    bpy.ops.mesh.primitive_uv_sphere_add(segments=20, ring_count=10, radius=r, location=c)
    o = bpy.context.active_object
    smooth(o)
    return finish(o, name, mat, parent)


hoodies = (principled("hoodie_a", (0.03, 0.05, 0.11), 0.92)[0], principled("hoodie_b", (0.11, 0.035, 0.03), 0.92)[0])
for h in hoodies:
    add_bump(h.node_tree, next(n for n in h.node_tree.nodes if n.type == "BSDF_PRINCIPLED"), 14, 0.6, 0.006)
for i, x in enumerate((-0.80, 0.80)):
    pl = empty("player", (x, far_y - 0.56, 0), (0, 0, math.radians(4 if i else -5)))
    bpy.ops.mesh.primitive_uv_sphere_add(segments=32, ring_count=16, radius=0.1, location=(0, 0, 0))
    head = bpy.context.active_object
    head.scale = (0.85, 0.95, 1.08)
    head.location = (0, 0.01, 1.18)
    smooth(head)
    finish(head, "head", hand_m, pl)
    # Hair sits a little high and forward, so the nape and neck show skin from behind.
    bpy.ops.mesh.primitive_uv_sphere_add(segments=32, ring_count=16, radius=0.104, location=(0, 0, 0))
    hair = bpy.context.active_object
    hair.scale = (0.87, 0.97, 1.04)
    hair.location = (0, 0.012, 1.19)
    smooth(hair)
    finish(hair, "hair", skin, pl)
    cyl("neck", 0.045, 0.08, (0, 0.02, 1.08), hand_m, verts=24, parent=pl)
    for sx in (-1, 1):
        ear = ball("ear", Vector((sx * 0.086, 0.015, 1.17)), 0.022, hand_m, pl)
        ear.scale = (0.45, 1.0, 1.3)
    bpy.ops.mesh.primitive_torus_add(major_radius=0.1, minor_radius=0.038, major_segments=40, minor_segments=14, location=(0, 0, 0))
    hood = bpy.context.active_object
    hood.location, hood.rotation_euler, hood.scale = (0, -0.03, 1.05), (math.radians(-28), 0, 0), (1.15, 0.9, 1.0)
    smooth(hood)
    finish(hood, "hood", hoodies[i], pl)
    bpy.ops.mesh.primitive_cube_add(size=1, location=(0, 0, 0))
    torso = bpy.context.active_object
    for v in torso.data.vertices:
        top = v.co.z > 0
        v.co = Vector((v.co.x * (0.44 if top else 0.36), v.co.y * (0.22 if top else 0.24), v.co.z * 0.5))
    torso.location = (0, 0.04, 0.82)
    bevel(torso, 0.085, 5)
    finish(torso, "torso", hoodies[i], pl)
    # Elbows stick out past the chair back so the arms read from the camera; hands rest on keyboard and mouse.
    for sx, wrist in ((-1, Vector((-0.09, 0.37, DESK + 0.058))), (1, Vector((0.245, 0.385, DESK + 0.05)))):
        shoulder, elbow = Vector((sx * 0.18, 0.04, 1.02)), Vector((sx * 0.28, 0.13, 0.84))
        pts = spline((shoulder, shoulder + Vector((sx * 0.06, 0.02, -0.06)), elbow, elbow + (wrist - elbow) * 0.5, wrist), 10)
        # Baggy hoodie sleeve: full at the shoulder, narrower past the elbow, ribbed cuff at the wrist.
        sleeve = [0.064, 0.058, 0.052, 0.049, 0.047, 0.044, 0.042, 0.039, 0.036, 0.034]
        tube("sleeve", pts, lambda t: sleeve[min(int(t * 9.99), 9)] * (1 + 0.04 * math.sin(t * 31)), hoodies[i], pl)
        d = (wrist - elbow).normalized()
        cuff = wrist - d * 0.02
        tube("cuff", [cuff - d * 0.03, cuff], lambda t: 0.034, cuff_m[i], pl, sides=18)
        make_hand(wrist - d * 0.01, d, sx, pl)
    box("far_kb", (0.40, 0.14, 0.022), (-0.06, 0.44, DESK + 0.011), black_plastic, 0.006, rot=(0, 0, math.radians(3)), parent=pl)
    box("far_mouse", (0.06, 0.11, 0.035), (0.25, 0.47, DESK + 0.017), black_plastic, 0.016, parent=pl)
    box("far_pad", (0.30, 0.26, 0.003), (0.26, 0.45, DESK + 0.0015), cloth, parent=pl)
    bpy.ops.mesh.primitive_torus_add(major_radius=0.105, minor_radius=0.008, major_segments=48, minor_segments=12, location=(0, 0, 0))
    hb = bpy.context.active_object
    hb.location = (0, 0.01, 1.19)
    hb.rotation_euler = (0, math.pi / 2, 0)
    smooth(hb)
    finish(hb, "headband", black_plastic, pl)
    for sx in (-1, 1):
        cyl("cup", 0.045, 0.03, (sx * 0.095, 0.01, 1.16), black_plastic, rot=(0, math.pi / 2, 0), verts=32, bev=0.006, parent=pl)
    chair = empty("chair", (x, far_y - 0.56, 0), (0, 0, math.radians(-3 if i else 6)))
    bpy.ops.mesh.primitive_cube_add(size=1, location=(0, 0, 0))
    back = bpy.context.active_object
    for v in back.data.vertices:
        w = 0.46 if v.co.z > 0 else 0.40
        v.co = Vector((v.co.x * w, v.co.y * 0.06 + 0.05 * (2 * v.co.x) ** 2, v.co.z * 0.52))
    back.location = (0, -0.15, 0.86)
    bevel(back, 0.025, 4)
    finish(back, "chair_back", mesh_m, chair)
    box("chair_frame", (0.04, 0.03, 0.34), (0, -0.17, 0.52), black_plastic, 0.01, parent=chair)
    box("seat", (0.48, 0.46, 0.08), (0, 0.04, 0.49), mesh_m, 0.03, parent=chair, seg=4)
    cyl("lift", 0.025, 0.36, (0, 0.04, 0.27), alu, verts=24, parent=chair)
    for k in range(5):
        a = k * 2 * math.pi / 5 + 0.3
        box("leg", (0.30, 0.04, 0.03), (math.cos(a) * 0.15, 0.04 + math.sin(a) * 0.15, 0.07), black_plastic, 0.01, rot=(0, 0, a), parent=chair)
        cyl("wheel", 0.025, 0.02, (math.cos(a) * 0.29, 0.04 + math.sin(a) * 0.29, 0.025), black_plastic, rot=(math.pi / 2, 0, a), verts=16, parent=chair)
cam_x, cam_y = 0.95, far_y - 0.02
bpy.ops.mesh.primitive_uv_sphere_add(segments=24, ring_count=12, radius=0.026, location=(cam_x, cam_y, DESK + 0.492))
wc = bpy.context.active_object
smooth(wc)
finish(wc, "webcam", dark_grey, None)
cyl("webcam_base", 0.03, 0.01, (cam_x, cam_y + 0.01, DESK + 0.462), dark_grey, verts=24, bev=0.003)
cyl("webcam_lens", 0.011, 0.006, (cam_x, cam_y - 0.024, DESK + 0.492), black_plastic, rot=(math.pi / 2, 0, 0), verts=24)
cyl("webcam_led", 0.0025, 0.004, (cam_x + 0.017, cam_y - 0.019, DESK + 0.505), emissive("led_red", (1.0, 0.08, 0.05), 14), rot=(math.pi / 2, 0, 0), verts=12)
box("far_wall", (8, 0.02, 3.4), (0, 2.75, 1.7), wall_m)
neon_m = emissive("neon", (1.0, 0.42, 0.12), 18)
text("neon_sign", "CIBER", 0.24, (-1.35, 2.72, 1.8), neon_m, extrude=0.01)
box("neon_back", (1.0, 0.02, 0.34), (-1.35, 2.735, 1.8), black_plastic, 0.01)

# Tower with the CD tray open
tower = empty("tower", (-0.53, 0.08, DESK), (0, 0, math.radians(16)))
box("case", (0.19, 0.44, 0.42), (0, 0, 0.21), beige, 0.006, parent=tower)
box("front", (0.19, 0.012, 0.42), (0, -0.226, 0.21), beige_dark, 0.004, parent=tower)
box("bay_cd", (0.148, 0.008, 0.042), (0, -0.234, 0.375), beige, 0.002, parent=tower)
box("bay_blank", (0.148, 0.006, 0.042), (0, -0.233, 0.327), beige, 0.002, parent=tower)
box("tray", (0.134, 0.15, 0.008), (0, -0.31, 0.37), black_plastic, 0.001, parent=tower)
cd_m = plastic("cd", (0.86, 0.86, 0.83), 0.35, 0.02)
rim_m = principled("cd_rim", (0.85, 0.86, 0.9), 0.12, 1.0, {"Thin Film Thickness": 520.0, "Thin Film IOR": 1.45})[0]
cd = cyl("cd", 0.06, 0.0012, (0, -0.31, 0.3748), cd_m, verts=96, parent=tower)
cyl("cd_rim", 0.0605, 0.001, (0, -0.31, 0.3745), rim_m, verts=96, parent=tower)
cyl("cd_ring", 0.019, 0.0014, (0, -0.31, 0.3751), rim_m, verts=64, parent=tower)
cyl("cd_hub", 0.0075, 0.0016, (0, -0.31, 0.3753), dark_grey, verts=32, parent=tower)
lab = text("cd_label", "FINAL SÁB", 0.02, (0.002, -0.338, 0.3758), marker, rot=(0, 0, math.radians(-8)), parent=tower)
lab.data.font = bpy.data.fonts.load("C:/Windows/Fonts/segoepr.ttf")
box("floppy", (0.10, 0.007, 0.026), (0, -0.233, 0.285), beige, 0.002, parent=tower)
box("floppy_slot", (0.07, 0.004, 0.003), (0, -0.2375, 0.288), black_plastic, parent=tower)
cyl("power", 0.011, 0.008, (0.04, -0.234, 0.12), beige_dark, rot=(math.pi / 2, 0, 0), verts=32, bev=0.002, parent=tower)
cyl("pled", 0.0025, 0.004, (-0.04, -0.2335, 0.15), led_green, rot=(math.pi / 2, 0, 0), verts=16, parent=tower)
cyl("hled", 0.0025, 0.004, (-0.04, -0.2335, 0.135), led_amber, rot=(math.pi / 2, 0, 0), verts=16, parent=tower)
for i in range(10):
    box("grille", (0.12, 0.004, 0.003), (0, -0.234, 0.05 + i * 0.006), black_plastic, parent=tower)

# Keyboard
kb = empty("keyboard", (0.0, -0.335, DESK), (math.radians(3), 0, math.radians(-1.5)))
box("kb_base", (0.46, 0.165, 0.022), (0, 0, 0.011), beige, 0.006, parent=kb)
bpy.ops.mesh.primitive_cube_add(size=1, location=(0, 0, 0))
cap = bpy.context.active_object
for v in cap.data.vertices:
    x, y, z = v.co.x * 0.0175, v.co.y * 0.0175, v.co.z * 0.011
    if z > 0:
        x *= 0.78
        y *= 0.78
    v.co = Vector((x, y, z))
bevel(cap, 0.0018, 2)
finish(cap, "keycap", key_mat, kb)
cap.location = (10, 10, 10)
cap.hide_render = True
worn_data = cap.data.copy()
worn_data.materials.clear()
worn_data.materials.append(key_worn)
PITCH = 0.019
ROWS = [
    [1] * 13 + [2],
    [1.5] + [1] * 12 + [1.5],
    [1.75] + [1] * 11 + [2.25],
    [2.25] + [1] * 10 + [2.75],
    [1.25, 1.25, 1.25, 6.25, 1.25, 1.25, 1.25, 1.25],
]
WASD = {(1, 2): "W", (2, 1): "A", (2, 2): "S", (2, 3): "D"}
for r, row in enumerate(ROWS):
    x = -0.205
    for c, w in enumerate(row):
        k = cap.copy()
        k.hide_render = False
        if (r, c) in WASD:
            k.data = worn_data
        scene.collection.objects.link(k)
        k.parent = kb
        k.location = (x + w * PITCH / 2, 0.046 - r * PITCH, 0.027 + (4 - r) * 0.0012)
        k.scale = ((w * PITCH - 0.0015) / 0.0175, 1, 1)
        if (r, c) in WASD:
            text("leg_" + WASD[(r, c)], WASD[(r, c)], 0.008, (k.location.x, k.location.y, k.location.z + 0.0056), legend, rot=(0, 0, 0), parent=kb)
        x += w * PITCH
    for c in range(4):
        if r == 4:
            continue
        k = cap.copy()
        k.hide_render = False
        scene.collection.objects.link(k)
        k.parent = kb
        k.location = (0.112 + c * PITCH, 0.046 - r * PITCH, 0.027 + (4 - r) * 0.0012)
cyl("kb_led", 0.002, 0.003, (0.19, 0.06, 0.023), led_green, verts=12, parent=kb)

# Mouse and mousepad
pad_m = image_mat("pad", "pad.png", 0.85)[0]
box("pad_base", (0.25, 0.21, 0.003), (0.37, -0.31, DESK + 0.0015), black_plastic, 0.001)
plane("pad_top", 0.248, 0.208, (0.37, -0.31, DESK + 0.0031), pad_m, rot=(0, 0, 0))
bpy.ops.mesh.primitive_uv_sphere_add(segments=48, ring_count=24, radius=1, location=(0, 0, 0))
mouse = bpy.context.active_object
for v in mouse.data.vertices:
    z = v.co.z * 0.02
    y = v.co.y * 0.054
    x = v.co.x * 0.031 * (1 - 0.12 * v.co.y)
    v.co = Vector((x, y, max(z, -0.004)))
mouse.location = (0.40, -0.32, DESK + 0.0072)
mouse.rotation_euler = (0, 0, math.radians(-8))
smooth(mouse)


def mouse_shell():
    """Beige shell with the button seams, the wheel slot and the shell joint drawn as recessed lines."""
    m, nt, p = principled("mouse_shell", (0.60, 0.55, 0.45), 0.42)
    vary_rough(nt, p, 0.34, 0.54)
    sep = nt.nodes.new("ShaderNodeSeparateXYZ")
    nt.links.new(obj_coords(nt), sep.inputs[0])

    def op(kind, a, b=None):
        n = nt.nodes.new("ShaderNodeMath")
        n.operation = kind
        for i, v in enumerate((a, b)):
            if v is None:
                continue
            if isinstance(v, (int, float)):
                n.inputs[i].default_value = v
            else:
                nt.links.new(v, n.inputs[i])
        return n.outputs[0]

    x, y, z = sep.outputs["X"], sep.outputs["Y"], sep.outputs["Z"]
    w = 0.00035
    between = op("MULTIPLY", op("LESS_THAN", op("ABSOLUTE", x), w), op("GREATER_THAN", y, 0.008))
    across = op("MULTIPLY", op("LESS_THAN", op("ABSOLUTE", op("SUBTRACT", y, 0.008)), w), op("GREATER_THAN", z, 0.006))
    joint = op("LESS_THAN", op("ABSOLUTE", op("SUBTRACT", z, 0.0015)), w * 0.8)
    slot = op("MULTIPLY", op("LESS_THAN", op("ABSOLUTE", x), 0.003), op("LESS_THAN", op("ABSOLUTE", op("SUBTRACT", y, 0.026)), 0.0072))
    seam = op("MAXIMUM", op("MAXIMUM", between, across), op("MAXIMUM", joint, slot))
    mix = nt.nodes.new("ShaderNodeMix")
    mix.data_type = "RGBA"
    nt.links.new(seam, mix.inputs["Factor"])
    mix.inputs["A"].default_value = (0.60, 0.55, 0.45, 1)
    mix.inputs["B"].default_value = (0.13, 0.12, 0.10, 1)
    nt.links.new(mix.outputs["Result"], p.inputs["Base Color"])
    bump = nt.nodes.new("ShaderNodeBump")
    bump.inputs["Strength"].default_value = 0.8
    bump.inputs["Distance"].default_value = 0.0005
    nt.links.new(op("SUBTRACT", 1.0, seam), bump.inputs["Height"])
    nt.links.new(bump.outputs["Normal"], p.inputs["Normal"])
    return m


finish(mouse, "mouse", mouse_shell(), None)
rubber = principled("rubber", (0.03, 0.03, 0.032), 0.7)[0]
cyl("mouse_wheel", 0.0062, 0.0042, (0, 0.026, 0.0137), rubber, rot=(0, math.pi / 2, 0), verts=20, parent=mouse)
cable = bpy.data.curves.new("cable", "CURVE")
cable.dimensions = "3D"
cable.bevel_depth = 0.0018
spl = cable.splines.new("BEZIER")
pts = [(0.405, -0.268, DESK + 0.012), (0.40, -0.17, DESK + 0.003), (0.25, -0.10, DESK + 0.002), (0.16, 0.05, DESK + 0.002), (0.13, 0.30, DESK + 0.002)]
spl.bezier_points.add(len(pts) - 1)
for bp, co in zip(spl.bezier_points, pts):
    bp.co = co
    bp.handle_left_type = bp.handle_right_type = "AUTO"
cable_o = bpy.data.objects.new("cable", cable)
scene.collection.objects.link(cable_o)
cable.materials.append(beige_dark)

# Headset lying on the desk
hs = empty("headset", (0.335, 0.065, DESK), (0, 0, math.radians(20)))
leather_m, lnt, lp = principled("leather", (0.03, 0.028, 0.028), 0.48)
add_bump(lnt, lp, 900, 0.3)
fabric_m, fnt, fp = principled("fabric", (0.045, 0.045, 0.05), 0.95)
add_bump(fnt, fp, 2600, 0.6)
CUP_Z, CUP_X = 0.0412, 0.074
# Shell: rounded back, open towards the head; (radius, depth) from the open rim to the back.
shell_prof = [(0.0, 0.031), (0.015, 0.0305), (0.027, 0.0275), (0.0355, 0.0215), (0.0398, 0.013), (0.041, 0.004), (0.0405, 0.0)]
for sx in (-1, 1):
    shell = lathe("cup_shell", shell_prof, black_plastic, hs)
    shell.location = (sx * CUP_X, 0, CUP_Z)
    shell.rotation_euler = (0, sx * math.pi / 2, 0)
    bpy.ops.mesh.primitive_torus_add(major_radius=0.029, minor_radius=0.0105, major_segments=64, minor_segments=20, location=(0, 0, 0))
    cush = bpy.context.active_object
    cush.location = (sx * (CUP_X - 0.0075), 0, CUP_Z)
    cush.rotation_euler = (0, math.pi / 2, 0)
    cush.scale = (1, 1, 0.75)
    smooth(cush)
    finish(cush, "cushion", leather_m, hs)
    cyl("cup_cloth", 0.021, 0.002, (sx * (CUP_X - 0.006), 0, CUP_Z), fabric_m, rot=(0, math.pi / 2, 0), verts=48, parent=hs)
    bpy.ops.mesh.primitive_torus_add(major_radius=0.017, minor_radius=0.0013, major_segments=64, minor_segments=8, location=(0, 0, 0))
    ring = bpy.context.active_object
    ring.location = (sx * (CUP_X + 0.0303), 0, CUP_Z)
    ring.rotation_euler = (0, math.pi / 2, 0)
    smooth(ring)
    finish(ring, "cup_ring", alu, hs)
    # Yoke: a fork over the cup from pivot to pivot, then the slider up into the band.
    bpy.ops.mesh.primitive_torus_add(major_radius=0.046, minor_radius=0.0032, major_segments=48, minor_segments=10, location=(0, 0, 0))
    yoke = bpy.context.active_object
    bmy = bmesh.new()
    bmy.from_mesh(yoke.data)
    bmesh.ops.delete(bmy, geom=[v for v in bmy.verts if v.co.x > 0.0005], context="VERTS")
    bmy.to_mesh(yoke.data)
    bmy.free()
    yoke.location = (sx * (CUP_X + 0.012), 0, CUP_Z)
    yoke.rotation_euler = (0, math.pi / 2, 0)
    smooth(yoke)
    finish(yoke, "yoke", black_plastic, hs)
    for sy in (-1, 1):
        cyl("pivot", 0.0045, 0.012, (sx * (CUP_X + 0.012), sy * 0.046, CUP_Z), dark_grey, rot=(math.pi / 2, 0, 0), verts=24, bev=0.001, parent=hs)
    box("slider", (0.0035, 0.011, 0.024), (sx * (CUP_X + 0.012), 0, CUP_Z + 0.056), alu, 0.001, parent=hs)
# Band over the top, padded underneath.
BAND_A, BAND_B, BAND_Z = CUP_X + 0.012, 0.068, CUP_Z + 0.062
sweep("band", arc_path(BAND_A, BAND_B, BAND_Z, 0, math.pi), rrect(0.026, 0.0075, 0.0034), black_plastic, hs)
sweep("band_pad", arc_path(BAND_A - 0.0082, BAND_B - 0.0082, BAND_Z, 0.5, math.pi - 0.5, 36), rrect(0.021, 0.0095, 0.0045), leather_m, hs)
cord = bpy.data.curves.new("hs_cord", "CURVE")
cord.dimensions = "3D"
cord.bevel_depth = 0.0016
cs = cord.splines.new("BEZIER")
cpts = [(-(CUP_X + 0.02), 0.02, CUP_Z - 0.03), (-(CUP_X + 0.03), 0.06, 0.0017), (-0.07, 0.15, 0.0017), (0.02, 0.24, 0.0017)]
cs.bezier_points.add(len(cpts) - 1)
for bp, co in zip(cs.bezier_points, cpts):
    bp.co = co
    bp.handle_left_type = bp.handle_right_type = "AUTO"
cord_o = bpy.data.objects.new("hs_cord", cord)
scene.collection.objects.link(cord_o)
cord.materials.append(black_plastic)
cord_o.parent = hs
mic = bpy.data.curves.new("mic", "CURVE")
mic.dimensions = "3D"
mic.bevel_depth = 0.0019
ms = mic.splines.new("BEZIER")
mpts = [(-(CUP_X + 0.024), -0.012, CUP_Z - 0.014), (-(CUP_X + 0.04), -0.08, 0.014), (-0.075, -0.15, 0.009)]
ms.bezier_points.add(len(mpts) - 1)
for bp, co in zip(ms.bezier_points, mpts):
    bp.co = co
    bp.handle_left_type = bp.handle_right_type = "AUTO"
mic_o = bpy.data.objects.new("mic", mic)
scene.collection.objects.link(mic_o)
mic.materials.append(black_plastic)
mic_o.parent = hs
cyl("mic_tip", 0.0065, 0.018, (-0.074, -0.154, 0.009), foam_m, rot=(math.pi / 2, 0, 0.5), verts=24, bev=0.003, parent=hs)

# Cola can: a lathed 33 cl can, already opened
can_m = image_mat("can", "can.png", 0.26, 0.55)[0]
CAN_BODY = (0.0135, 0.0985)
can_prof = [(0.0, 0.0055), (0.011, 0.0048), (0.019, 0.0022), (0.0232, 0.0), (0.0262, 0.0006), (0.0302, 0.0042),
            (0.0325, 0.0098), (0.033, CAN_BODY[0]), (0.033, 0.035), (0.033, 0.06), (0.033, 0.08), (0.033, CAN_BODY[1]),
            (0.0326, 0.1015), (0.0302, 0.1068), (0.0276, 0.1108), (0.0271, 0.1128), (0.0277, 0.1143), (0.0279, 0.1153),
            (0.0273, 0.1158), (0.0264, 0.1151), (0.0258, 0.1118), (0.012, 0.1114), (0.0, 0.1112)]
bmc = bmesh.new()
vs = [bmc.verts.new((r, 0, z)) for r, z in can_prof]
es = [bmc.edges.new((vs[i], vs[i + 1])) for i in range(len(vs) - 1)]
bmesh.ops.spin(bmc, geom=vs + es, cent=(0, 0, 0), axis=(0, 0, 1), angle=2 * math.pi, steps=96)
bmesh.ops.remove_doubles(bmc, verts=bmc.verts, dist=1e-7)
outward(bmc)
uvl = bmc.loops.layers.uv.verify()
for f in bmc.faces:
    zc = f.calc_center_median().z
    if not (CAN_BODY[0] - 1e-5 < zc < CAN_BODY[1] + 1e-5):
        f.material_index = 1
        continue
    us = [(math.atan2(l.vert.co.y, l.vert.co.x) / (2 * math.pi)) % 1.0 for l in f.loops]
    if max(us) - min(us) > 0.5:
        us = [u + 1 if u < 0.5 else u for u in us]
    for l, u in zip(f.loops, us):
        l[uvl].uv = (u, (l.vert.co.z - CAN_BODY[0]) / (CAN_BODY[1] - CAN_BODY[0]))
can = mesh_object("can", bmc, can_m, None)
can.data.uv_layers[0].name = "UVMap"
can.data.materials.append(alu)
can.location = (0.48, -0.10, DESK + 0.0003)
can.rotation_euler = (0, 0, math.radians(128))
hole_m = principled("can_hole", (0.004, 0.002, 0.001), 0.15)[0]
box("can_hole", (0.0105, 0.0135, 0.0006), (0, 0.0118, 0.1113), hole_m, 0.0024, parent=can)
tab = empty("can_tab", (0, 0.0005, 0.1121), (math.radians(6), 0, 0))
tab.parent = can
box("can_tab", (0.0115, 0.022, 0.0008), (0, -0.002, 0), alu, 0.0026, parent=tab)
box("can_tab_eye", (0.006, 0.0075, 0.0012), (0, -0.0075, 0.0001), hole_m, 0.0022, parent=tab)
cyl("can_rivet", 0.0021, 0.0012, (0, 0, 0.1118), alu, verts=24, parent=can)

# Ticket, coins, post-it, poster
ticket_m = image_mat("ticket", "ticket.png", 0.75)[0]
bpy.ops.mesh.primitive_grid_add(x_subdivisions=12, y_subdivisions=6, size=1, location=(0, 0, 0))
tk = bpy.context.active_object
for v in tk.data.vertices:
    v.co = Vector((v.co.x * 0.092, v.co.y * 0.0575, 0.0035 * (2 * v.co.x) ** 2))
tk.location = (-0.28, -0.395, DESK + 0.0008)
tk.rotation_euler = (0, 0, math.radians(14))
smooth(tk)
finish(tk, "ticket", ticket_m, None)
for i, (dx, dy) in enumerate(((0, 0), (0.024, 0.01), (0.012, -0.018))):
    cyl("coin", 0.0115, 0.0022, (-0.205 + dx, -0.455 + dy, DESK + 0.0011 + i * 0.0001), coin_mat, verts=48, bev=0.0006)

pad_m2 = image_mat("notepad", "notepad.png", 0.82)[0]
paper_side = principled("paper_side", (0.78, 0.76, 0.70), 0.9)[0]
note = empty("notepad", (-0.30, -0.19, DESK), (0, 0, math.radians(-6)))
box("note_pages", (0.148, 0.21, 0.006), (0, 0, 0.003), paper_side, 0.0008, parent=note)
box("note_back", (0.150, 0.212, 0.0015), (0, 0, 0.0007), principled("cardboard", (0.32, 0.24, 0.15), 0.8)[0], 0.0004, parent=note)
plane("note_top", 0.148, 0.21, (0, 0, 0.0062), pad_m2, rot=(0, 0, 0), parent=note)
wire = principled("spiral", (0.55, 0.55, 0.58), 0.3, 1.0)[0]
for i in range(18):
    bpy.ops.mesh.primitive_torus_add(major_radius=0.0042, minor_radius=0.0006, major_segments=16, minor_segments=6, location=(0, 0, 0))
    ring = bpy.context.active_object
    ring.location = (-0.0655 + i * 0.0077, 0.0975, 0.0062)
    ring.rotation_euler = (0, math.pi / 2, 0)
    smooth(ring)
    finish(ring, "spiral", wire, note)
pen = empty("pen", (-0.255, -0.215, DESK + 0.0105), (0, math.pi / 2, math.radians(58)))
pen_body = principled("pen_body", (0.80, 0.84, 0.88), 0.12, extra={"Transmission Weight": 0.6})[0]
pen_blue = plastic("pen_blue", (0.03, 0.12, 0.5), 0.35)
cyl("pen_shaft", 0.0038, 0.13, (0, 0, 0), pen_body, verts=6, bev=0.0006, parent=pen)
cyl("pen_cap", 0.0042, 0.045, (0, 0, 0.055), pen_blue, verts=6, bev=0.0006, parent=pen)
cyl("pen_tip", 0.0016, 0.012, (0, 0, -0.07), alu, verts=12, parent=pen)
cyl("pen_end", 0.0034, 0.006, (0, 0, -0.064), pen_blue, verts=6, parent=pen)

CX, CY, CZ = 1.62, 2.73, 1.70
clock_rim = plastic("clock_rim", (0.04, 0.04, 0.045), 0.35)
cyl("clock_rim", 0.165, 0.036, (CX, CY, CZ), clock_rim, rot=(math.pi / 2, 0, 0), verts=64, bev=0.008)
face_m = image_mat("clockface", "clockface.png", 0.6)[0]
bpy.ops.mesh.primitive_circle_add(vertices=64, radius=0.15, fill_type="NGON", location=(0, 0, 0))
face = bpy.context.active_object
bmf = bmesh.new()
bmf.from_mesh(face.data)
uvf = bmf.loops.layers.uv.verify()
for f in bmf.faces:
    for l in f.loops:
        l[uvf].uv = (l.vert.co.x / 0.3 + 0.5, l.vert.co.y / 0.3 + 0.5)
bmf.to_mesh(face.data)
bmf.free()
face.data.uv_layers[0].name = "UVMap"
face.location = (CX, CY - 0.0185, CZ)
face.rotation_euler = (math.pi / 2, 0, 0)
finish(face, "clock_face", face_m, None)
hand_dark = principled("hand", (0.02, 0.02, 0.02), 0.4)[0]
hand_red = principled("hand_red", (0.6, 0.03, 0.02), 0.4)[0]
for nm, ln, wd, mat, dy in (("clock_hour", 0.085, 0.009, hand_dark, 0.021), ("clock_min", 0.125, 0.006, hand_dark, 0.0225), ("clock_sec", 0.135, 0.0022, hand_red, 0.024)):
    bpy.ops.mesh.primitive_cube_add(size=1, location=(0, 0, 0))
    hnd = bpy.context.active_object
    for v in hnd.data.vertices:
        v.co = Vector((v.co.x * wd, v.co.y * 0.0015, v.co.z * (ln + 0.025) + (ln + 0.025) / 2 - 0.025))
    hnd.location = (CX, CY - dy, CZ)
    finish(hnd, nm, mat, None)
cyl("clock_pin", 0.006, 0.006, (CX, CY - 0.0255, CZ), hand_red, rot=(math.pi / 2, 0, 0), verts=16)

poster_m = image_mat("poster", "poster.png", 0.7)[0]
PX, PZ, PW, PH = 0.72, 1.58, 0.42, 0.606
plane("poster", PW, PH, (PX, 2.738, PZ), poster_m, rot=(math.pi / 2, math.radians(1.8), 0))
tape_m, tnt, tp = principled("tape", (0.86, 0.83, 0.68), 0.45)
for sx, sz, a in ((-1, 1, -35), (1, 1, 30), (-1, -1, 30), (1, -1, -30)):
    plane("tape", 0.07, 0.02, (PX + sx * PW / 2, 2.736, PZ + sz * PH / 2), tape_m, rot=(math.pi / 2, math.radians(a), 0))
spot = bpy.data.lights.new("poster_spot", "SPOT")
spot.energy = 22
spot.spot_size = math.radians(40)
spot.spot_blend = 0.6
spot.color = (1.0, 0.86, 0.66)
spo = bpy.data.objects.new("poster_spot", spot)
scene.collection.objects.link(spo)
spo.location = (PX, 2.1, 2.35)
spo.rotation_euler = (Vector((PX, 2.74, PZ)) - Vector(spo.location)).to_track_quat("-Z", "Y").to_euler()

# Lights
world = bpy.data.worlds.new("world")
scene.world = world
try:
    world.use_nodes = True
except Exception:
    pass
bg = next(n for n in world.node_tree.nodes if n.type == "BACKGROUND")
bg.inputs["Color"].default_value = (0.004, 0.005, 0.008, 1)


def area(name, size, loc, rot, energy, color, size_y=None):
    lt = bpy.data.lights.new(name, "AREA")
    lt.shape = "RECTANGLE"
    lt.size = size
    lt.size_y = size_y or size
    lt.energy = energy
    lt.color = color
    o = bpy.data.objects.new(name, lt)
    scene.collection.objects.link(o)
    o.location = loc
    o.rotation_euler = rot
    return o


area("fluorescent", 1.2, (0, -0.55, 2.45), (0, 0, 0), 26, (0.84, 0.97, 0.92), 0.08)
area("fluorescent_far", 1.2, (0, 1.2, 2.45), (0, 0, 0), 22, (0.84, 0.97, 0.92), 0.08)
area("neon_right", 0.5, (1.5, -0.9, 1.75), (math.radians(60), 0, math.radians(55)), 18, (1.0, 0.45, 0.15))
nb = bpy.data.lights.new("neighbor", "POINT")
nb.energy = 14
nb.color = (0.55, 0.3, 1.0)
nb.shadow_soft_size = 0.15
nbo = bpy.data.objects.new("neighbor", nb)
scene.collection.objects.link(nbo)
nbo.location = (-1.15, -0.05, 1.12)
nl = bpy.data.lights.new("neon_glow", "POINT")
nl.energy = 25
nl.color = (1.0, 0.42, 0.12)
nl.shadow_soft_size = 0.4
nlo = bpy.data.objects.new("neon_glow", nl)
scene.collection.objects.link(nlo)
nlo.location = (-1.35, 2.55, 1.8)

# Smoke haze: cybercafes in the 2000s still allowed smoking
haze_m = bpy.data.materials.new("haze")
try:
    haze_m.use_nodes = True
except Exception:
    pass
hnt = haze_m.node_tree
for n in list(hnt.nodes):
    if n.type == "BSDF_PRINCIPLED":
        hnt.nodes.remove(n)
vol = hnt.nodes.new("ShaderNodeVolumePrincipled")
vol.inputs["Density"].default_value = 0.016
vol.inputs["Anisotropy"].default_value = 0.35
vol.inputs["Color"].default_value = (0.8, 0.85, 0.9, 1)
mo = next(n for n in hnt.nodes if n.type == "OUTPUT_MATERIAL")
hnt.links.new(vol.outputs["Volume"], mo.inputs["Volume"])
box("haze", (6, 5.5, 3.0), (0, 0.4, 1.5), haze_m)

# Render setup
scene.render.engine = "CYCLES"
try:
    cp = bpy.context.preferences.addons["cycles"].preferences
    for kind in ("OPTIX", "CUDA"):
        cp.compute_device_type = kind
        cp.get_devices()
        devs = [d for d in cp.devices if d.type == kind]
        if devs:
            for d in cp.devices:
                d.use = d.type == kind
            scene.cycles.device = "GPU"
            print("render device", kind, [d.name for d in devs])
            break
except Exception as e:
    print("gpu setup failed", e)
scene.cycles.samples = 48 if QUICK else 320
scene.cycles.use_denoising = True
scene.render.resolution_x = 1440
scene.render.resolution_y = 900
scene.render.resolution_percentage = 50 if QUICK else 100
scene.render.image_settings.file_format = "PNG"
scene.view_settings.view_transform = "Khronos PBR Neutral"
for look in ("AgX - Medium High Contrast", "Medium High Contrast"):
    try:
        scene.view_settings.look = look
        break
    except Exception:
        pass
scene.view_settings.exposure = 0.0

bpy.context.view_layer.update()
cd_world = cd.matrix_world.translation.copy()
screen_world = scr.matrix_world.translation.copy()


def camera(name, loc, target, lens, fstop, focus):
    cam = bpy.data.cameras.new(name)
    cam.lens = lens
    cam.sensor_width = 36
    cam.dof.use_dof = True
    cam.dof.aperture_fstop = fstop
    cam.dof.focus_distance = (Vector(focus) - Vector(loc)).length
    o = bpy.data.objects.new(name, cam)
    scene.collection.objects.link(o)
    o.location = loc
    o.rotation_euler = (Vector(target) - Vector(loc)).to_track_quat("-Z", "Y").to_euler()
    return o


cams = [
    camera("cam_mesa", (0.0, -1.22, 1.26), (0.0, 0.0, 1.02), 28, 7.0, screen_world),
    camera("cam_crt", (0.09, -0.66, 1.05), (0.0, -0.09, 1.0), 45, 3.2, screen_world),
    camera("cam_cd", (-0.20, -0.60, 1.36), tuple(cd_world), 55, 2.4, tuple(cd_world)),
    camera("cam_poster", (0.22, -0.95, 1.32), (PX, 2.738, PZ), 85, 3.5, (PX, 2.738, PZ)),
    camera("cam_ticket", (-0.10, -0.68, 0.98), (-0.25, -0.40, DESK), 55, 2.8, (-0.28, -0.395, DESK)),
    camera("cam_notepad", (-0.20, -0.56, 1.16), (-0.30, -0.19, DESK), 45, 3.2, (-0.30, -0.19, DESK)),
    camera("cam_stream", (1.40, -0.20, 1.44), (0.90, far_y - 0.06, DESK + 0.25), 44, 4.0, (0.95, far_y - 0.06, DESK + 0.27)),
    camera("cam_mesa_v", (0.0, -1.38, 1.62), (0.0, -0.08, 1.12), 29.8, 7.0, screen_world),
]
os.makedirs(OUT, exist_ok=True)
if "--move" in ARGS:
    src, dst = cams[0], cams[1]
    cd_ = bpy.data.cameras.new("move")
    cd_.sensor_width = 36
    cd_.dof.use_dof = True
    cd_.dof.focus_object = scr
    mv = bpy.data.objects.new("cam_move", cd_)
    scene.collection.objects.link(mv)
    mv.rotation_mode = "QUATERNION"
    for frame, c in ((8, src), (64, dst)):
        mv.location = c.location
        mv.rotation_quaternion = c.rotation_euler.to_quaternion()
        cd_.lens = c.data.lens
        cd_.dof.aperture_fstop = c.data.dof.aperture_fstop
        mv.keyframe_insert("location", frame=frame)
        mv.keyframe_insert("rotation_quaternion", frame=frame)
        cd_.keyframe_insert("lens", frame=frame)
        cd_.dof.keyframe_insert("aperture_fstop", frame=frame)
    scene.camera = mv
    scene.frame_start, scene.frame_end = 1, 72
    scene.render.fps = 24
    scene.render.resolution_x, scene.render.resolution_y, scene.render.resolution_percentage = 960, 600, 100
    scene.cycles.samples = 40
    scene.render.filepath = os.path.join(OUT, "move", "f_####")
    bpy.ops.render.render(animation=True)
    print("rendered move")
    cams = []
# Home stills are wider than their frame; landing/components/ciber-stage.tsx uses the same factors.
OVERSCAN = {"cam_mesa": 1.5, "cam_mesa_v": 2.1}
for c in cams:
    if ONLY and c.name[4:] not in ONLY.split(","):
        continue
    scene.camera = c
    base = (1408, 1600) if c.name.endswith("_v") else (1440, 900)
    k = OVERSCAN.get(c.name, 1.0)
    if k != 1.0:
        # Wider frame, same vertical fov and blur: the landing lets the loading still overflow its region.
        tan_v = c.data.sensor_width / 2 / c.data.lens * min(1.0, base[1] / base[0])
        lens = 18 / tan_v
        c.data.dof.aperture_fstop *= lens / c.data.lens
        c.data.sensor_fit, c.data.sensor_height, c.data.lens = "VERTICAL", 36, lens
    scene.render.resolution_x, scene.render.resolution_y = round(base[0] * k), base[1]
    scene.render.filepath = os.path.join(OUT, c.name[4:] + (".quick" if QUICK else "") + ".png")
    bpy.ops.render.render(write_still=True)
    print("rendered", scene.render.filepath)
bpy.ops.wm.save_as_mainfile(filepath=os.path.join(HERE, "puesto07.blend"))
