package voicecomms

import "errors"

const SchemaVersion = "1.0"

const (
	FormatNone   = "none"
	FormatOpus   = "opus"
	FormatSteam  = "steam"
	FormatEngine = "engine"
	FormatMixed  = "mixed"
)

var (
	ErrTargetNotFound = errors.New("voicecomms: target steamid not found in demo")
	ErrInvalidTarget  = errors.New("voicecomms: target steamid must be a 64-bit unsigned integer")
)

type Packet struct {
	XUID       uint64
	Tick       int
	Bytes      int
	Format     string
	SampleRate uint32
	Data       []byte
	Offsets    []uint32
}

type Index struct {
	SchemaVersion string  `json:"schema_version"`
	Demo          string  `json:"demo"`
	Tickrate      int     `json:"tickrate"`
	SampleRateHz  uint32  `json:"sample_rate_hz,omitempty"`
	Tracks        []Track `json:"tracks"`
}

type Track struct {
	SteamID64 string `json:"steamid64"`
	Name      string `json:"name,omitempty"`
	Team      string `json:"team,omitempty"`
	Path      string `json:"path"`
	Packets   int    `json:"packets"`
	FirstTick int    `json:"first_tick,omitempty"`
	LastTick  int    `json:"last_tick,omitempty"`
}

type Sighting struct {
	SteamID64 string
	Name      string
	Team      string
	Tick      int
}

type Meta struct {
	Demo          string
	Map           string
	Tickrate      int
	DurationTicks int
}

type Report struct {
	SchemaVersion string        `json:"schema_version"`
	Demo          string        `json:"demo"`
	Map           string        `json:"map,omitempty"`
	Tickrate      int           `json:"tickrate,omitempty"`
	DurationTicks int           `json:"duration_ticks,omitempty"`
	VoicePresent  bool          `json:"voice_present"`
	Format        string        `json:"format"`
	SampleRateHz  uint32        `json:"sample_rate_hz,omitempty"`
	TotalPackets  int           `json:"total_packets"`
	Target        PlayerVoice   `json:"target"`
	Teammates     []PlayerVoice `json:"teammates"`
	Others        OtherVoice    `json:"others"`
	Limitations   []string      `json:"limitations"`
}

type PlayerVoice struct {
	SteamID64       string  `json:"steamid64"`
	Name            string  `json:"name,omitempty"`
	Team            string  `json:"team,omitempty"`
	Packets         int     `json:"packets"`
	Bytes           int     `json:"bytes"`
	FirstTick       int     `json:"first_tick,omitempty"`
	LastTick        int     `json:"last_tick,omitempty"`
	SecondsEstimate float64 `json:"seconds_estimate"`
}

type OtherVoice struct {
	Players int `json:"players"`
	Packets int `json:"packets"`
	Bytes   int `json:"bytes"`
}
