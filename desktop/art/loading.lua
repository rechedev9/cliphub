-- ClipHub Studio boot animation: 64x64 pixel art, 48 frames at 24 fps, one seamless loop.
-- Run: Aseprite.exe -b --script-param out=<folder> --script loading.lua (the Steam build needs Start-Process -Wait).
-- It writes loading.aseprite and loading-sheet.png; loading.html inlines the sheet as a base64 data URL.
local p = app.params
local out = p.out or "."
local S, N = 64, 48
local rgba = app.pixelColor.rgba

local function rgb(h) return tonumber(h:sub(1, 2), 16), tonumber(h:sub(3, 4), 16), tonumber(h:sub(5, 6), 16) end
local function col(r, g, b, a) return rgba(math.floor(r + 0.5), math.floor(g + 0.5), math.floor(b + 0.5), a or 255) end
local function hex(h, a) local r, g, b = rgb(h) return col(r, g, b, a) end

-- Brand ramp of the app icon: orange on the left, purple on the right.
local RAMP = { "ff9a2e", "ff6a1f", "e2504f", "b0469e", "8b3ff0" }
local function ramp(t)
  t = math.max(0, math.min(1, t)) * (#RAMP - 1)
  local i = math.min(math.floor(t), #RAMP - 2)
  local r1, g1, b1 = rgb(RAMP[i + 1])
  local r2, g2, b2 = rgb(RAMP[i + 2])
  local f = t - i
  return r1 + (r2 - r1) * f, g1 + (g2 - g1) * f, b1 + (b2 - b1) * f
end
local bayer = { { 0, 8, 2, 10 }, { 12, 4, 14, 6 }, { 3, 11, 1, 9 }, { 15, 7, 13, 5 } }
local function dither(x, y) return (bayer[y % 4 + 1][x % 4 + 1] + 0.5) / 16 end

-- Pointy-top hexagon like the icon.
local V = { { 32, 2 }, { 58, 17 }, { 58, 47 }, { 32, 62 }, { 6, 47 }, { 6, 17 } }
local function inside(x, y)
  for i = 1, 6 do
    local a, b = V[i], V[i % 6 + 1]
    if (b[1] - a[1]) * (y - a[2]) - (b[2] - a[2]) * (x - a[1]) < 0 then return false end
  end
  return true
end
local rim, onRim = {}, {}
for i = 1, 6 do
  local x0, y0, x1, y1 = V[i][1], V[i][2], V[i % 6 + 1][1], V[i % 6 + 1][2]
  local dx, dy = math.abs(x1 - x0), -math.abs(y1 - y0)
  local sx, sy = x0 < x1 and 1 or -1, y0 < y1 and 1 or -1
  local err = dx + dy
  while not (x0 == x1 and y0 == y1) do
    rim[#rim + 1] = { x0, y0 }
    onRim[y0 * S + x0] = true
    local e2 = 2 * err
    if e2 >= dy then err = err + dy; x0 = x0 + sx end
    if e2 <= dx then err = err + dx; y0 = y0 + sy end
  end
end
local L = #rim

-- Italic block "CH", 12x18 per glyph, rows skewed right toward the top.
local C = {
  "...#########", "..##########", ".###########", "############",
  "####........", "####........", "####........", "####........", "####........",
  "####........", "####........", "####........", "####........", "####........",
  "############", ".###########", "..##########", "...#########",
}
local Hg = {}
for r = 1, 18 do Hg[r] = (r >= 8 and r <= 11) and "############" or "####....####" end
local letter = {}
local function stamp(g, ox, oy)
  for r = 1, 18 do
    local skew = math.floor((18 - r) / 3)
    for c = 1, 12 do
      if g[r]:sub(c, c) == "#" then letter[(oy + r - 1) * S + ox + c - 1 + skew] = true end
    end
  end
end
stamp(C, 14, 19)
stamp(Hg, 29, 19)

-- Play triangle overlapping the foot of the H.
local T = { { 37, 34 }, { 37, 52 }, { 51, 43 } }
local function inTri(x, y)
  local pos, neg = false, false
  for i = 1, 3 do
    local a, b = T[i], T[i % 3 + 1]
    local cr = (b[1] - a[1]) * (y - a[2]) - (b[2] - a[2]) * (x - a[1])
    if cr > 0 then pos = true elseif cr < 0 then neg = true end
  end
  return not (pos and neg)
end

local spr = Sprite(S, S, ColorMode.RGB)
spr.filename = out .. "/loading.aseprite"
local layer = spr.layers[1]
layer.name = "boot"
for f = 2, N do spr:newEmptyFrame() end

local TAIL = 26
local N4 = { { -1, 0 }, { 1, 0 }, { 0, -1 }, { 0, 1 } }
local function innerRim(x, y)
  if onRim[y * S + x] or not inside(x, y) then return false end
  for _, d in ipairs(N4) do if onRim[(y + d[2]) * S + x + d[1]] then return true end end
  return false
end
for f = 0, N - 1 do
  -- Painted back to front so a comet's head is never covered by its own tail.
  local img = Image(S, S, ColorMode.RGB)
  for y = 0, S - 1 do
    for x = 0, S - 1 do
      if onRim[y * S + x] then img:drawPixel(x, y, hex("4a4b53"))
      elseif innerRim(x, y) then img:drawPixel(x, y, hex("2a2b31"))
      elseif inside(x, y) then img:drawPixel(x, y, hex("121317")) end
    end
  end
  -- Two comets run the rim half a lap apart, so half a lap per loop is seamless.
  for c = 0, 1 do
    local head = f / N * L / 2 + c * L / 2
    for k = TAIL - 1, 0, -1 do
      local pt = rim[(math.floor(head) - k) % L + 1]
      local x, y = pt[1], pt[2]
      local r, g, b = ramp((x - 6) / 52)
      local fade = (1 - k / TAIL) ^ 1.2
      if k <= 2 then local w = 0.8 - k * 0.25; r, g, b = r + (255 - r) * w, g + (255 - g) * w, b + (255 - b) * w end
      local br, bg, bb = 74, 75, 83
      img:drawPixel(x, y, col(br + (r - br) * fade, bg + (g - bg) * fade, bb + (b - bb) * fade))
      for _, d in ipairs(N4) do
        local nx, ny = x + d[1], y + d[2]
        if nx >= 0 and ny >= 0 and nx < S and ny < S and not onRim[ny * S + nx] then
          if innerRim(nx, ny) then
            local ir, ig, ib = 42, 43, 49
            img:drawPixel(nx, ny, col(ir + (r - ir) * fade * 0.55, ig + (g - ig) * fade * 0.55, ib + (b - ib) * fade * 0.55))
          elseif not inside(nx, ny) and k <= 8 then
            img:drawPixel(nx, ny, col(r, g, b, math.floor(150 * (1 - k / 9))))
          end
        end
      end
    end
  end
  -- Letters with a dark drop shadow.
  for y = 0, S - 1 do
    for x = 0, S - 1 do
      if letter[y * S + x] then
        img:drawPixel(x, y, hex(letter[(y - 1) * S + x] and "e9e9ee" or "ffffff"))
      elseif letter[(y - 1) * S + x - 1] then
        img:drawPixel(x, y, hex("06060a"))
      end
    end
  end
  -- Triangle: dark outline, dithered brand gradient, and a light sweep in the second half of the loop.
  local sweep = (f - 26) / 14 * 34 + 71
  for y = 0, S - 1 do
    for x = 0, S - 1 do
      if inTri(x, y) then
        local t = ((x - 37) + (y - 34)) / 26
        local q = math.floor(t * 4 + dither(x, y) - 0.5) / 4
        local r, g, b = ramp(q)
        local d = math.abs((x + y) - sweep)
        if f >= 26 and f < 40 and d < 2.5 then
          local w = d < 1 and 0.75 or 0.4
          r, g, b = r + (255 - r) * w, g + (255 - g) * w, b + (255 - b) * w
        end
        img:drawPixel(x, y, col(r, g, b))
      elseif not letter[y * S + x] and (inTri(x - 1, y) or inTri(x + 1, y) or inTri(x, y - 1) or inTri(x, y + 1)) then
        img:drawPixel(x, y, hex("08080b"))
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
