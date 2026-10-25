-- Shorts thumbnail: pixel art at 180x320, exported x6 to 1080x1920.
-- Params (--script-param k=v): out, name, top, big, label, sub, score, result (win|loss), map
local W, H, SCALE = 180, 320, 6
-- The channel grid shows Shorts at about 2:3, cutting 26 art px (154 out px) top and bottom.
local CROP = 26
local p = app.params
local out = p.out or "."
local name = p.name or "thumb"
local spr = Sprite(W, H, ColorMode.RGB)
local img = spr.cels[1].image
local rgba = app.pixelColor.rgba

local function hex(h)
  return rgba(tonumber(h:sub(1, 2), 16), tonumber(h:sub(3, 4), 16), tonumber(h:sub(5, 6), 16), 255)
end
local function px(x, y, c)
  if x >= 0 and y >= 0 and x < W and y < H then img:drawPixel(x, y, c) end
end
local function rect(x, y, w, h, c)
  for j = y, y + h - 1 do for i = x, x + w - 1 do px(i, j, c) end end
end

local bayer = { { 0, 8, 2, 10 }, { 12, 4, 14, 6 }, { 3, 11, 1, 9 }, { 15, 7, 13, 5 } }
local function dither(x, y) return (bayer[y % 4 + 1][x % 4 + 1] + 0.5) / 16 end

-- Sunset sky
local sky = { "1b1030", "2e1748", "4a1f5c", "7a2b5e", "b53f50", "e0663f", "f29b45", "f7c25c" }
local GROUND = 230
for y = 0, GROUND - 1 do
  local t = y / GROUND * (#sky - 1)
  local i = math.floor(t)
  for x = 0, W - 1 do
    px(x, y, hex(sky[(t - i > dither(x, y)) and math.min(i + 2, #sky) or (i + 1)]))
  end
end
math.randomseed(1113)
for _ = 1, 40 do
  px(math.random(0, W - 1), math.random(CROP, 90), hex(math.random() < 0.3 and "ffe9a8" or "8a7bb0"))
end

-- Sun with retro stripes
local cx, cy = 90, 175
for y = cy - 48, cy + 48 do
  for x = cx - 48, cx + 48 do
    local d = math.sqrt((x - cx) ^ 2 + (y - cy) ^ 2)
    if d < 44 then px(x, y, hex(d < 38 and "ffe27a" or "ffc94d"))
    elseif d < 47 and (x + y) % 2 == 0 then px(x, y, hex("ffb347")) end
  end
end
for _, sy in ipairs({ 190, 198, 205, 211, 216 }) do
  for x = cx - 44, cx + 44 do
    if (x - cx) ^ 2 + (sy - cy) ^ 2 < 44 ^ 2 then px(x, sy, hex("f29b45")) end
  end
end

-- Pyramids and sand
local function pyramid(x0, base, half, height, light, dark)
  for j = 0, height - 1 do
    local w = math.floor(half * j / height)
    for x = x0 - w, x0 + w do px(x, base - height + j, hex(x < x0 and light or dark)) end
  end
end
pyramid(40, GROUND + 10, 55, 60, "5a2a3a", "3d1c2c")
pyramid(150, GROUND + 10, 45, 48, "5a2a3a", "3d1c2c")
pyramid(100, GROUND + 15, 38, 30, "74343f", "4e2333")
for y = GROUND, H - 1 do
  for x = 0, W - 1 do
    px(x, y, hex((y - GROUND) / (H - GROUND) > dither(x, y) and "8a5a34" or "c48a4c"))
  end
end
rect(0, GROUND, W, 1, hex("e8b56a"))

-- 5x7 bitmap font; characters outside it render as spaces
local G = {
  A = { "01110", "10001", "10001", "11111", "10001", "10001", "10001" },
  B = { "11110", "10001", "10001", "11110", "10001", "10001", "11110" },
  C = { "01111", "10000", "10000", "10000", "10000", "10000", "01111" },
  D = { "11110", "10001", "10001", "10001", "10001", "10001", "11110" },
  E = { "11111", "10000", "10000", "11110", "10000", "10000", "11111" },
  F = { "11111", "10000", "10000", "11110", "10000", "10000", "10000" },
  G = { "01111", "10000", "10000", "10011", "10001", "10001", "01111" },
  H = { "10001", "10001", "10001", "11111", "10001", "10001", "10001" },
  I = { "01110", "00100", "00100", "00100", "00100", "00100", "01110" },
  J = { "00111", "00010", "00010", "00010", "00010", "10010", "01100" },
  K = { "10001", "10010", "10100", "11000", "10100", "10010", "10001" },
  L = { "10000", "10000", "10000", "10000", "10000", "10000", "11111" },
  M = { "10001", "11011", "10101", "10101", "10001", "10001", "10001" },
  N = { "10001", "11001", "10101", "10011", "10001", "10001", "10001" },
  O = { "01110", "10001", "10001", "10001", "10001", "10001", "01110" },
  P = { "11110", "10001", "10001", "11110", "10000", "10000", "10000" },
  Q = { "01110", "10001", "10001", "10001", "10101", "10010", "01101" },
  R = { "11110", "10001", "10001", "11110", "10100", "10010", "10001" },
  S = { "01111", "10000", "10000", "01110", "00001", "00001", "11110" },
  T = { "11111", "00100", "00100", "00100", "00100", "00100", "00100" },
  U = { "10001", "10001", "10001", "10001", "10001", "10001", "01110" },
  V = { "10001", "10001", "10001", "10001", "10001", "01010", "00100" },
  W = { "10001", "10001", "10001", "10101", "10101", "11011", "10001" },
  X = { "10001", "10001", "01010", "00100", "01010", "10001", "10001" },
  Y = { "10001", "10001", "01010", "00100", "00100", "00100", "00100" },
  Z = { "11111", "00001", "00010", "00100", "01000", "10000", "11111" },
  ["0"] = { "01110", "10001", "10011", "10101", "11001", "10001", "01110" },
  ["1"] = { "00100", "01100", "00100", "00100", "00100", "00100", "01110" },
  ["2"] = { "01110", "10001", "00001", "00010", "00100", "01000", "11111" },
  ["3"] = { "11110", "00001", "00001", "01110", "00001", "00001", "11110" },
  ["4"] = { "10010", "10010", "10010", "11111", "00010", "00010", "00010" },
  ["5"] = { "11111", "10000", "11110", "00001", "00001", "10001", "01110" },
  ["6"] = { "00110", "01000", "10000", "11110", "10001", "10001", "01110" },
  ["7"] = { "11111", "00001", "00010", "00100", "01000", "01000", "01000" },
  ["8"] = { "01110", "10001", "10001", "01110", "10001", "10001", "01110" },
  ["9"] = { "01110", "10001", "10001", "01111", "00001", "00010", "01100" },
  ["-"] = { "00000", "00000", "00000", "11111", "00000", "00000", "00000" },
  ["%"] = { "11001", "11010", "00010", "00100", "01000", "01011", "10011" },
  ["!"] = { "00100", "00100", "00100", "00100", "00100", "00000", "00100" },
  ["?"] = { "01110", "10001", "00001", "00010", "00100", "00000", "00100" },
  ["."] = { "00000", "00000", "00000", "00000", "00000", "01100", "01100" },
}

local function width(s, sc) return #s * 6 * sc - sc end

local function eachPixel(s, x0, y0, sc, fn)
  for k = 1, #s do
    local g = G[s:sub(k, k)]
    if g then
      for r = 1, 7 do
        for c = 1, 5 do
          if g[r]:sub(c, c) == "1" then fn(x0 + ((k - 1) * 6 + c - 1) * sc, y0 + (r - 1) * sc) end
        end
      end
    end
  end
end

-- Draws s centered in a slot of 7*maxSc rows at y, shrinking the scale until it fits the width.
local function text(o)
  local s = string.upper(o.s or "")
  if s == "" then return end
  local ol, sh = o.ol or 1, o.shadow or 0
  local sc = o.sc
  while sc > 1 and width(s, sc) + 2 * ol + sh > W - 12 do sc = sc - 1 end
  local x0 = math.floor((W - width(s, sc)) / 2)
  local y = o.y + math.floor((o.sc - sc) * 7 / 2)
  local function block(c, dx)
    return function(x, yy) rect(x - ol + dx, yy - ol + dx, sc + 2 * ol, sc + 2 * ol, hex(c)) end
  end
  if sh > 0 then eachPixel(s, x0, y, sc, block("120818", sh)) end
  eachPixel(s, x0, y, sc, block(o.outline or "120818", 0))
  local mid = y + math.floor(7 * sc / 2)
  eachPixel(s, x0, y, sc, function(x, yy)
    for j = 0, sc - 1 do rect(x, yy + j, sc, 1, hex(yy + j < mid and o.top or o.bottom)) end
  end)
end

-- Every line sits inside the grid-safe band y = CROP .. H - CROP
text { s = p.top, y = 36, sc = 3, top = "ffffff", bottom = "d8d0e8", shadow = 2 }
text { s = p.big, y = 64, sc = 9, top = "fff27a", bottom = "ffb020", ol = 2, shadow = 3 }
text { s = p.label, y = 134, sc = 4, top = "ffffff", bottom = "ffd9a0", ol = 2, shadow = 2 }
text { s = p.sub, y = 198, sc = 2, top = "ffffff", bottom = "ffe0c0", shadow = 1 }
if p.score and p.score ~= "" then
  local win = p.result == "win"
  local by = 226
  rect(28, by, 124, 40, hex("120818"))
  rect(30, by + 2, 120, 36, hex(win and "2f9e5a" or "c0283a"))
  rect(30, by + 2, 120, 3, hex(win and "5fd18a" or "e8505c"))
  text { s = p.score, y = by + 6, sc = 4, top = "ffffff", bottom = win and "d0ffe0" or "ffd0d0",
    outline = win and "0f3d22" or "5a0f1c" }
end
text { s = p.map, y = 274, sc = 2, top = "3d1c2c", bottom = "3d1c2c", outline = "e8b56a" }

spr:saveAs(out .. "/" .. name .. ".aseprite")
app.command.SpriteSize { width = W * SCALE, height = H * SCALE, method = "nearest" }
spr:saveCopyAs(out .. "/" .. name .. "_1080x1920.png")
local full = spr.cels[1].image
Image(full, Rectangle(0, CROP * SCALE, W * SCALE, (H - 2 * CROP) * SCALE)):saveAs(out .. "/" .. name .. "_grid-preview.png")
