-- ClipHub Studio boot animation: 64x64 pixel art, 48 frames at 24 fps, one seamless loop.
-- Aseprite.exe -b --script-param out=<folder> --script loading.lua (Steam build: Start-Process -Wait) writes
-- loading.aseprite and loading-sheet.png, which loading.html inlines as a base64 data URL.
local p = app.params
local out = p.out or "."
local S, N = 64, 48
local rgba = app.pixelColor.rgba

local function hex(h)
  return rgba(tonumber(h:sub(1, 2), 16), tonumber(h:sub(3, 4), 16), tonumber(h:sub(5, 6), 16), 255)
end
local PAPER, SIGNAL = hex("f2efe8"), hex("ff5a1f")

local K = S / 896
local function art(v) return (v - 52) * K end
local CX, CY = art(500), art(500)
local R_IN, R_OUT = (250 - 66) * K, (250 + 66) * K
local GAP = math.rad(42)

local function inC(x, y)
  local dx, dy = x - CX, y - CY
  local d = math.sqrt(dx * dx + dy * dy)
  if d < R_IN or d > R_OUT then return false end
  return math.abs(math.atan(dy, dx)) >= GAP
end

local T0 = { { art(418), art(352) }, { art(418), art(648) }, { art(700), art(500) } }
local function inTri(x, y, shift)
  local pos, neg = false, false
  for i = 1, 3 do
    local a, b = T0[i], T0[i % 3 + 1]
    local cr = (b[1] - a[1]) * (y - a[2]) - (b[2] - a[2]) * (x - shift - a[1])
    if cr > 0 then pos = true elseif cr < 0 then neg = true end
  end
  return not (pos and neg)
end

local SS = 4
local function coverage(x, y, test)
  local hits = 0
  for sy = 0, SS - 1 do
    for sx = 0, SS - 1 do
      if test(x + (sx + 0.5) / SS, y + (sy + 0.5) / SS) then hits = hits + 1 end
    end
  end
  return hits / (SS * SS)
end

local cmask = {}
for y = 0, S - 1 do
  for x = 0, S - 1 do
    if coverage(x, y, inC) >= 0.5 then cmask[y * S + x] = true end
  end
end

local spr = Sprite(S, S, ColorMode.RGB)
spr.filename = out .. "/loading.aseprite"
local layer = spr.layers[1]
layer.name = "boot"
for f = 2, N do spr:newEmptyFrame() end

local REACH = 6
for f = 0, N - 1 do
  local shift = math.floor(REACH * (1 - math.cos(f / N * 2 * math.pi)) / 2 + 0.5)
  local tri = function(x, y) return inTri(x, y, shift) end
  local img = Image(S, S, ColorMode.RGB)
  for y = 0, S - 1 do
    for x = 0, S - 1 do
      if cmask[y * S + x] then
        img:drawPixel(x, y, PAPER)
      elseif coverage(x, y, tri) >= 0.5 then
        img:drawPixel(x, y, SIGNAL)
      end
    end
  end
  spr:newCel(layer, f + 1, img, Point(0, 0))
  spr.frames[f + 1].duration = 1 / 24
end

spr:saveAs(out .. "/loading.aseprite")
app.command.ExportSpriteSheet {
  ui = false, askOverwrite = false, type = SpriteSheetType.HORIZONTAL,
  textureFilename = out .. "/loading-sheet.png", dataFilename = "",
}
if p.preview == "1" then
  spr:resize(S * 4, S * 4)
  app.command.ExportSpriteSheet {
    ui = false, askOverwrite = false, type = SpriteSheetType.ROWS, columns = 8,
    textureFilename = out .. "/loading-preview.png", dataFilename = "",
  }
  spr:saveCopyAs(out .. "/loading-preview.gif")
end
