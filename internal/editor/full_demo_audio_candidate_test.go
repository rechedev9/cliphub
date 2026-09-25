package editor

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rechedev9/cliphub/internal/filecommit"
	"github.com/rechedev9/cliphub/internal/recapplan"
)

const (
	// 1818 frames = 30.3s = 1,454,400 samples: not a loudnorm 3s window and
	// not an AAC packet boundary (remainder 320). 1819 leaves a 96-sample
	// tail that Ubuntu apt FFmpeg 6.x native AAC shortens by 32 samples.
	fullDemoAudioTestFrames = 1818
	// A 30-second program with a deliberately quiet + transient mix that the
	// native single-pass master must retarget or recover.
	fullDemoAudioTestTransient = "aevalsrc=(0.03+0.22*gte(mod(t\\,30)\\,15))*sin(2*PI*440*t)+0.05*sin(2*PI*2311*t)*lt(mod(t\\,1)\\,0.03):s=48000"
	fullDemoAudioTestSteady    = "aevalsrc=0.3*sin(2*PI*440*t):s=48000"
	fullDemoAudioTestSilent    = "anullsrc=r=48000:cl=stereo"
)

type fullDemoAudioPacket struct {
	PTS      int64
	Duration int64
}

// fullDemoAudioTestProgram generates real H.264 + PCM media of exactly frames
// frames using the bundled FFmpeg's local filters. No capture or user media.
func fullDemoAudioTestProgram(t *testing.T, ctx context.Context, ffmpeg, dir string, frames int, audio, size string) (string, float64) {
	t.Helper()
	duration := float64(frames) / recapplan.OutputFPS
	path := filepath.Join(dir, "program.nut")
	command := []string{
		ffmpeg, "-y", "-v", "error",
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=navy:s=%s:r=60:d=%s", size, decimal(duration)),
		"-f", "lavfi", "-i", audio + ":d=" + decimal(duration),
		"-map", "0:v", "-map", "1:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-bf", "0",
		"-c:a", "pcm_f32le", "-ac", "2", "-t", decimal(duration), path,
	}
	if _, err := runFFmpegOutput(ctx, command, "generate audio-only candidate program"); err != nil {
		t.Fatal(err)
	}
	return path, duration
}

func fullDemoTestFFprobe(t *testing.T) string {
	t.Helper()
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("FFprobe is required for the Full Demo audio media canary:", err)
	}
	return ffprobe
}

func fullDemoTestVideoHash(t *testing.T, ctx context.Context, ffmpeg, path string) string {
	t.Helper()
	out, err := runFFmpegOutput(ctx, []string{ffmpeg, "-v", "error", "-i", path, "-map", "0:v:0", "-f", "hash", "-hash", "sha256", "-"}, "decoded video hash")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func fullDemoTestAudioMD5(t *testing.T, ctx context.Context, ffmpeg, path string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", path, "-map", "0:a:0", "-f", "s32le", "-ac", "2", "-ar", "48000", "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("decode audio %s: %v: %s", path, err, stderr.String())
	}
	sum := md5.Sum(stdout.Bytes())
	return fmt.Sprintf("%x:%d", sum, stdout.Len())
}

func fullDemoTestVideoFrames(t *testing.T, ctx context.Context, ffprobe, path string) int64 {
	t.Helper()
	out, err := runFFmpegOutput(ctx, []string{ffprobe, "-v", "error", "-count_frames", "-select_streams", "v:0", "-show_entries", "stream=nb_read_frames", "-of", "default=nw=1:nk=1", path}, "decoded frame count")
	if err != nil {
		t.Fatal(err)
	}
	frames, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		t.Fatalf("frame count %q: %v", out, err)
	}
	return frames
}

func fullDemoTestAudioPackets(t *testing.T, ctx context.Context, ffprobe, path string) []fullDemoAudioPacket {
	t.Helper()
	body, err := runFFmpegOutput(ctx, []string{ffprobe, "-v", "error", "-select_streams", "a:0", "-show_entries", "packet=pts,duration", "-of", "json", path}, "audio packet clock")
	if err != nil {
		t.Fatal(err)
	}
	var packets struct {
		Packets []fullDemoAudioPacket `json:"packets"`
	}
	if err := json.Unmarshal([]byte(body), &packets); err != nil {
		t.Fatal(err)
	}
	return packets.Packets
}

// fullDemoTestPacketClock requires contiguous packets and returns the first and
// final timestamp. A negative start is legitimate AAC priming, not a gap; the
// final timestamp is the sample the playable timeline must reach. Native AAC
// may legally merge several access units into one MP4 sample, so a packet
// duration above 1024 samples is not itself a gap.
func fullDemoTestPacketClock(t *testing.T, packets []fullDemoAudioPacket, name string) (int64, int64) {
	t.Helper()
	if len(packets) == 0 {
		t.Fatalf("%s: no audio packets", name)
	}
	expected := packets[0].PTS
	for i, packet := range packets {
		if packet.Duration <= 0 {
			t.Fatalf("%s: packet %d has an invalid duration: %+v", name, i, packet)
		}
		if packet.PTS != expected {
			t.Fatalf("%s: packet clock has a gap at %d: %+v, expected %d", name, i, packet, expected)
		}
		expected += packet.Duration
	}
	return packets[0].PTS, expected
}

func fullDemoTestStreams(t *testing.T, ctx context.Context, ffprobe, path string) []struct {
	CodecType  string `json:"codec_type"`
	CodecName  string `json:"codec_name"`
	SampleRate string `json:"sample_rate"`
	Channels   int    `json:"channels"`
} {
	t.Helper()
	body, err := runFFmpegOutput(ctx, []string{ffprobe, "-v", "error", "-show_entries", "stream=codec_type,codec_name,sample_rate,channels", "-of", "json", path}, "stream probe")
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
		} `json:"streams"`
	}
	if err := json.Unmarshal([]byte(body), &probe); err != nil {
		t.Fatal(err)
	}
	return probe.Streams
}

func fullDemoCommandHas(command []string, want string) bool {
	for _, arg := range command {
		if arg == want {
			return true
		}
	}
	return false
}

func fullDemoTestNoTemporaryAudioFiles(t *testing.T, dir string) {
	t.Helper()
	for _, pattern := range []string{".*attempt*", "full-demo-audio-candidate*"} {
		paths, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 0 {
			t.Fatalf("temporary audio files leaked in %s: %v", dir, paths)
		}
	}
}

func fullDemoTestAssertOutputUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("previous good output was replaced: %q", got)
	}
}

func TestFullDemoAudioCandidateCommandsExcludeProgramVideo(t *testing.T) {
	const samples = int64(1455200)
	nativeFilter := "loudnorm=I=-14.000000:TP=-1.500000:LRA=11.000000"
	native := fullDemoNativeCandidateCommand("ffmpeg", "program.nut", "candidate.m4a", nativeFilter, samples, 30.316667)
	for _, forbidden := range []string{"0:v:0", "-c:v", "copy"} {
		if fullDemoCommandHas(native, forbidden) {
			t.Fatalf("native candidate command copies program video: %q", native)
		}
	}
	if !fullDemoCommandHas(native, "0:a:0") || !fullDemoCommandHas(native, "aac") {
		t.Fatalf("native candidate does not encode program audio: %q", native)
	}
	wantFilter := nativeFilter + fmt.Sprintf(",aresample=48000,aformat=channel_layouts=stereo,apad=whole_len=%d,atrim=end_sample=%d", samples, samples)
	if !fullDemoCommandHas(native, wantFilter) {
		t.Fatalf("native candidate filter changed: %q", native)
	}
	if !fullDemoCommandHas(native, "-t") || native[len(native)-1] != "candidate.m4a" {
		t.Fatalf("native candidate lost its -t bound or destination: %q", native)
	}

	recoveryMaster := ProgramAACFallbackMaster{Encoder: "aac_mf", GainDB: 3, CeilingDBFS: -5}
	recoveryFilter := recoveryMaster.filter("loudnorm=I=-14.000000:TP=-1.800000") + ",apad=whole_len=1455200,atrim=end_sample=1455200,asetpts=N/SR/TB"
	packetDuration := "setts=duration=min(DURATION\\,max(0\\,1455200/48000/TB-PTS))"
	recovery := fullDemoRecoveryCandidateCommand("ffmpeg", "program.nut", "candidate.m4a", recoveryFilter, recoveryMaster.Encoder, packetDuration)
	for _, forbidden := range []string{"0:v:0", "-c:v", "copy"} {
		if fullDemoCommandHas(recovery, forbidden) {
			t.Fatalf("recovery candidate command copies program video: %q", recovery)
		}
	}
	if !fullDemoCommandHas(recovery, recoveryFilter) {
		t.Fatalf("recovery candidate filter chain changed: %q", recovery)
	}
	if !fullDemoCommandHas(recovery, packetDuration) {
		t.Fatalf("recovery candidate lost its setts packet-duration correction: %q", recovery)
	}
	if !fullDemoCommandHas(recovery, "aac_mf") || !fullDemoCommandHas(recovery, "0:a:0") {
		t.Fatalf("recovery candidate lost its encoder or audio map: %q", recovery)
	}
	if fullDemoCommandHas(recovery, "-t") {
		t.Fatalf("recovery candidate must rely on setts, not -t: %q", recovery)
	}

	mux := fullDemoFinalMuxCommand("ffmpeg", "program.nut", "candidate.m4a", "final.mp4")
	for _, want := range []string{"0:v:0", "1:a:0", "copy", "+faststart"} {
		if !fullDemoCommandHas(mux, want) {
			t.Fatalf("final mux command is missing %q: %q", want, mux)
		}
	}
	if mux[len(mux)-1] != "final.mp4" {
		t.Fatalf("final mux does not target the output: %q", mux)
	}
}

func TestFullDemoAudioOnlyCandidateMuxIsBitExact(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe := fullDemoTestFFprobe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	const frames = fullDemoAudioTestFrames
	input, duration := fullDemoAudioTestProgram(t, ctx, ffmpeg, dir, frames, fullDemoAudioTestSteady, "320x180")
	samples := int64(math.Round(duration * recapplan.SampleRate))
	target := recapplan.DefaultOptions().Audio.Loudness
	measurement, err := measureLoudness(ctx, ffmpeg, input, target, "", duration, nil)
	if err != nil {
		t.Fatal(err)
	}
	attemptTarget := aacHeadroomTarget(target)
	filter, err := measuredLoudnessFilter(attemptTarget, measurement)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "final.mp4")
	candidate, cleanup, err := fullDemoAudioCandidatePath(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(candidate, ".m4a") {
		t.Fatalf("candidate container is not audio MP4: %q", candidate)
	}
	if err := runFFmpegWithOptionalLogAndProgress(ctx, fullDemoNativeCandidateCommand(ffmpeg, input, candidate, filter, samples, duration), "audio candidate", "", duration, nil); err != nil {
		cleanup()
		t.Fatal(err)
	}
	// The candidate is audio only, so no video packet is ever copied for it.
	streams := fullDemoTestStreams(t, ctx, ffprobe, candidate)
	if len(streams) != 1 || streams[0].CodecType != "audio" || streams[0].CodecName != "aac" || streams[0].SampleRate != "48000" || streams[0].Channels != 2 {
		t.Fatalf("candidate is not a single stereo AAC stream: %+v", streams)
	}
	inputInfo, err := os.Stat(input)
	if err != nil {
		t.Fatal(err)
	}
	candidateInfo, err := os.Stat(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if candidateInfo.Size() >= inputInfo.Size() {
		t.Fatalf("audio candidate copied the program video: candidate=%d input=%d", candidateInfo.Size(), inputInfo.Size())
	}
	candidatePackets := fullDemoTestAudioPackets(t, ctx, ffprobe, candidate)
	candidateStart, candidateEnd := fullDemoTestPacketClock(t, candidatePackets, "candidate")
	if candidateStart != -1024 {
		t.Fatalf("native candidate lost AAC priming: first packet %d", candidateStart)
	}
	if candidateEnd != samples {
		t.Fatalf("candidate reaches sample %d, want %d", candidateEnd, samples)
	}
	if last := candidatePackets[len(candidatePackets)-1]; last.Duration >= 1024 {
		t.Fatalf("candidate did not exercise a short final packet: %+v", last)
	}
	candidatePCM := fullDemoTestAudioMD5(t, ctx, ffmpeg, candidate)

	evidence, err := deliverFullDemoAACCandidate(ctx, ffmpeg, input, candidate, output, filepath.Join(dir, "logs"), target, false, duration, ProgramLoudnessEvidence{Policy: target.PolicyVersion, MasterTargets: []recapplan.LoudnessOptions{}, DecodedAAC: []LoudnessMeasurement{}, Status: "unverified"}, nil)
	cleanup()
	if err != nil {
		t.Fatalf("final mux or certification failed: %v; evidence: %+v", err, evidence)
	}
	if evidence.Status != "verified-decoded-aac" || evidence.FinalMuxedAAC == nil {
		t.Fatalf("final muxed AAC was not certified: %+v", evidence)
	}

	streams = fullDemoTestStreams(t, ctx, ffprobe, output)
	video, audio := false, false
	for _, stream := range streams {
		switch stream.CodecType {
		case "video":
			video = stream.CodecName == "h264"
		case "audio":
			audio = stream.CodecName == "aac" && stream.SampleRate == "48000" && stream.Channels == 2
		}
	}
	if !video || !audio || len(streams) != 2 {
		t.Fatalf("muxed output is not H.264 + stereo AAC: %+v", streams)
	}
	if got := fullDemoTestVideoHash(t, ctx, ffmpeg, output); got != fullDemoTestVideoHash(t, ctx, ffmpeg, input) {
		t.Fatalf("muxing changed the copied video: %s", got)
	}
	if got := fullDemoTestVideoFrames(t, ctx, ffprobe, output); got != frames {
		t.Fatalf("muxed output has %d frames, want %d", got, frames)
	}
	if got := fullDemoTestAudioMD5(t, ctx, ffmpeg, output); got != candidatePCM {
		t.Fatalf("muxed PCM differs from the candidate PCM: %s versus %s", got, candidatePCM)
	}
	outputPackets := fullDemoTestAudioPackets(t, ctx, ffprobe, output)
	if !reflect.DeepEqual(outputPackets, candidatePackets) {
		t.Fatalf("muxing altered the candidate packet clock:\n%+v\n%+v", outputPackets, candidatePackets)
	}
	if last := outputPackets[len(outputPackets)-1]; last.Duration >= 1024 {
		t.Fatalf("muxed output did not keep the short final packet: %+v", last)
	}
	fullDemoTestNoTemporaryAudioFiles(t, dir)
}

func TestFullDemoAudioOnlyMuxMatchesLegacySinglePass(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe := fullDemoTestFFprobe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	const frames = fullDemoAudioTestFrames
	input, duration := fullDemoAudioTestProgram(t, ctx, ffmpeg, dir, frames, fullDemoAudioTestSteady, "320x180")
	samples := int64(math.Round(duration * recapplan.SampleRate))
	target := recapplan.DefaultOptions().Audio.Loudness
	output := filepath.Join(dir, "final.mp4")
	evidence, err := masterFullDemoProgram(ctx, ffmpeg, input, output, filepath.Join(dir, "logs"), target, false, duration, nil)
	if err != nil {
		t.Fatalf("master: %v; evidence: %+v", err, evidence)
	}
	if evidence.Status != "verified-decoded-aac" || evidence.FinalMuxedAAC == nil {
		t.Fatalf("accepted status or final certification changed: %+v", evidence)
	}
	if len(evidence.FallbackMasters) != 0 {
		t.Fatalf("previously passing program used recovery: %+v", evidence.FallbackMasters)
	}
	if len(evidence.MasterTargets) != 1 || len(evidence.DecodedAAC) != 1 {
		t.Fatalf("native attempt sequence changed: targets=%d decoded=%d", len(evidence.MasterTargets), len(evidence.DecodedAAC))
	}
	wantTarget := target
	wantTarget.TargetTPDBTP -= 0.3
	if evidence.MasterTargets[0] != wantTarget {
		t.Fatalf("first native target changed: %+v, want %+v", evidence.MasterTargets[0], wantTarget)
	}
	last := evidence.DecodedAAC[0]
	if last.IntegratedLUFS == nil || last.TruePeakDBTP == nil || math.Abs(*last.IntegratedLUFS-target.TargetILUFS) > .5 || *last.TruePeakDBTP > target.TargetTPDBTP {
		t.Fatalf("decoded acceptance changed: %+v", last)
	}

	measurement, err := measureLoudness(ctx, ffmpeg, input, target, "", duration, nil)
	if err != nil {
		t.Fatal(err)
	}
	filter, err := measuredLoudnessFilter(wantTarget, measurement)
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, "legacy.mp4")
	legacyCommand := []string{ffmpeg, "-y", "-hide_banner", "-nostats", "-v", "info", "-i", input, "-map", "0:v:0", "-map", "0:a:0", "-c:v", "copy", "-af", filter + fmt.Sprintf(",aresample=48000,aformat=channel_layouts=stereo,apad=whole_len=%d,atrim=end_sample=%d", samples, samples), "-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-ac", "2", "-t", decimal(duration), "-movflags", "+faststart", legacy}
	if _, err := runFFmpegOutput(ctx, legacyCommand, "legacy single-pass master"); err != nil {
		t.Fatal(err)
	}
	if got, want := fullDemoTestAudioMD5(t, ctx, ffmpeg, output), fullDemoTestAudioMD5(t, ctx, ffmpeg, legacy); got != want {
		t.Fatalf("audio-only candidate and mux changed the delivered PCM: %s versus legacy %s", got, want)
	}
	inputVideo := fullDemoTestVideoHash(t, ctx, ffmpeg, input)
	if got := fullDemoTestVideoHash(t, ctx, ffmpeg, output); got != inputVideo {
		t.Fatalf("muxing changed the copied video: %s", got)
	}
	if got := fullDemoTestVideoHash(t, ctx, ffmpeg, legacy); got != inputVideo {
		t.Fatalf("legacy reference changed the copied video: %s", got)
	}
	if got := fullDemoTestVideoFrames(t, ctx, ffprobe, output); got != frames {
		t.Fatalf("muxed output has %d frames, want %d", got, frames)
	}
	packets := fullDemoTestAudioPackets(t, ctx, ffprobe, output)
	if _, end := fullDemoTestPacketClock(t, packets, "delivered"); end != samples {
		t.Fatalf("delivered audio reaches sample %d, want %d", end, samples)
	}
	if last := packets[len(packets)-1]; last.Duration >= 1024 {
		t.Fatalf("delivered audio did not keep the short final packet: %+v", last)
	}
	fullDemoTestNoTemporaryAudioFiles(t, dir)
}

func TestFullDemoAudioOnlyRecoveryPreservesVideoAndClock(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ffprobe := fullDemoTestFFprobe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if !hasMediaFoundationAAC(ctx, ffmpeg) {
		t.Skip("Windows Media Foundation AAC is required for this recovery canary")
	}
	dir := t.TempDir()
	const frames = fullDemoAudioTestFrames
	input, duration := fullDemoAudioTestProgram(t, ctx, ffmpeg, dir, frames, fullDemoAudioTestTransient, "320x180")
	samples := int64(math.Round(duration * recapplan.SampleRate))
	target := recapplan.DefaultOptions().Audio.Loudness
	first, err := measureLoudness(ctx, ffmpeg, input, target, "", duration, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "final.mp4")
	evidence, err := recoverFullDemoAAC(ctx, ffmpeg, input, committedFullDemoProgramVideo(input), output, filepath.Join(dir, "logs"), target, duration, ProgramLoudnessEvidence{Policy: target.PolicyVersion, Input: first, MasterTargets: []recapplan.LoudnessOptions{}, DecodedAAC: []LoudnessMeasurement{}, Status: "unverified"}, nil)
	if err != nil {
		t.Fatalf("recovery: %v; evidence: %+v", err, evidence)
	}
	assertRecoveredAAC(t, evidence, target)
	if evidence.FinalMuxedAAC == nil {
		t.Fatal("recovery did not certify the final muxed AAC")
	}
	if got := fullDemoTestVideoHash(t, ctx, ffmpeg, output); got != fullDemoTestVideoHash(t, ctx, ffmpeg, input) {
		t.Fatalf("recovery changed the copied video: %s", got)
	}
	if got := fullDemoTestVideoFrames(t, ctx, ffprobe, output); got != frames {
		t.Fatalf("recovery output has %d frames, want %d", got, frames)
	}
	packets := fullDemoTestAudioPackets(t, ctx, ffprobe, output)
	if _, end := fullDemoTestPacketClock(t, packets, "recovered"); end != samples {
		t.Fatalf("recovered audio reaches sample %d, want %d", end, samples)
	}
	if last := packets[len(packets)-1]; last.Duration >= 1024 {
		t.Fatalf("recovered audio did not exercise a short final packet: %+v", last)
	}
	fullDemoTestNoTemporaryAudioFiles(t, dir)
}

func TestFullDemoAudioOnlyAcceptanceContract(t *testing.T) {
	target := recapplan.DefaultOptions().Audio.Loudness
	measured := func(integrated, peak float64) LoudnessMeasurement {
		return LoudnessMeasurement{Status: "measured", IntegratedLUFS: &integrated, TruePeakDBTP: &peak}
	}
	for _, tc := range []struct {
		name           string
		decoded        LoudnessMeasurement
		silentApproved bool
		accepted       bool
		terminal       bool
	}{
		{"within target", measured(target.TargetILUFS-0.5, target.TargetTPDBTP), false, true, false},
		{"loud by tolerance edge", measured(target.TargetILUFS+0.5, target.TargetTPDBTP-0.1), false, true, false},
		{"loud beyond tolerance", measured(target.TargetILUFS+0.51, target.TargetTPDBTP), false, false, false},
		{"peak over ceiling", measured(target.TargetILUFS, target.TargetTPDBTP+0.01), false, false, false},
		{"silent approved", LoudnessMeasurement{Status: "silent"}, true, true, false},
		{"silent rejected", LoudnessMeasurement{Status: "silent"}, false, false, true},
		{"unmeasurable", LoudnessMeasurement{}, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accepted, err := fullDemoDecodedAACAccepted(tc.decoded, target, tc.silentApproved)
			if (err != nil) != tc.terminal {
				t.Fatalf("terminal error = %v, want %v", err, tc.terminal)
			}
			if accepted != tc.accepted {
				t.Fatalf("accepted = %v, want %v", accepted, tc.accepted)
			}
		})
	}
}

func TestFullDemoAudioOnlyPublicationPreservesPreviousOutput(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := t.TempDir()
	const frames = 120
	input, duration := fullDemoAudioTestProgram(t, ctx, ffmpeg, dir, frames, fullDemoAudioTestSteady, "320x180")
	target := recapplan.DefaultOptions().Audio.Loudness
	template := ProgramLoudnessEvidence{Policy: target.PolicyVersion, MasterTargets: []recapplan.LoudnessOptions{}, DecodedAAC: []LoudnessMeasurement{}, Status: "unverified"}
	previous := []byte("previous good output")
	logs := filepath.Join(dir, "logs")
	output := filepath.Join(dir, "final.mp4")
	writePrevious := func(t *testing.T) {
		t.Helper()
		if err := os.WriteFile(output, previous, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("corrupt candidate", func(t *testing.T) {
		writePrevious(t)
		corrupt := filepath.Join(dir, "corrupt.m4a")
		if err := os.WriteFile(corrupt, []byte("not media"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := deliverFullDemoAACCandidate(ctx, ffmpeg, input, corrupt, output, logs, target, false, duration, template, nil); err == nil {
			t.Fatal("corrupt candidate was published")
		}
		fullDemoTestAssertOutputUnchanged(t, output, previous)
		fullDemoTestNoTemporaryAudioFiles(t, dir)
	})

	t.Run("missing program for mux", func(t *testing.T) {
		writePrevious(t)
		if _, err := deliverFullDemoAACCandidate(ctx, ffmpeg, filepath.Join(dir, "missing.nut"), input, output, logs, target, false, duration, template, nil); err == nil {
			t.Fatal("mux without a program was published")
		}
		fullDemoTestAssertOutputUnchanged(t, output, previous)
		fullDemoTestNoTemporaryAudioFiles(t, dir)
	})

	t.Run("unapproved silent final mix", func(t *testing.T) {
		writePrevious(t)
		silent := filepath.Join(dir, "silent.m4a")
		command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", fullDemoAudioTestSilent + ":d=" + decimal(duration), "-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-ac", "2", "-t", decimal(duration), silent}
		if _, err := runFFmpegOutput(ctx, command, "silent candidate"); err != nil {
			t.Fatal(err)
		}
		if _, err := deliverFullDemoAACCandidate(ctx, ffmpeg, input, silent, output, logs, target, false, duration, template, nil); err == nil {
			t.Fatal("silent final mix was published without approval")
		}
		fullDemoTestAssertOutputUnchanged(t, output, previous)
		fullDemoTestNoTemporaryAudioFiles(t, dir)
	})

	t.Run("candidate without an audio stream", func(t *testing.T) {
		writePrevious(t)
		videoOnly := filepath.Join(dir, "video-only.mp4")
		command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=64x64:r=30:d=" + decimal(duration), "-c:v", "libx264", "-preset", "ultrafast", "-an", "-t", decimal(duration), videoOnly}
		if _, err := runFFmpegOutput(ctx, command, "video-only candidate"); err != nil {
			t.Fatal(err)
		}
		if _, err := deliverFullDemoAACCandidate(ctx, ffmpeg, input, videoOnly, output, logs, target, false, duration, template, nil); err == nil {
			t.Fatal("candidate without audio was published")
		}
		fullDemoTestAssertOutputUnchanged(t, output, previous)
		fullDemoTestNoTemporaryAudioFiles(t, dir)
	})

	// FFmpeg creates the attempt before it writes the header, so a candidate
	// MP4 cannot carry fails the mux with an already created, empty attempt
	// on disk: the one failure shape whose leak the other cases cannot see.
	t.Run("unmuxable candidate", func(t *testing.T) {
		writePrevious(t)
		unmuxable := filepath.Join(dir, "unmuxable.wav")
		command := []string{ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "sine=f=440:r=48000:d=" + decimal(duration), "-c:a", "adpcm_ima_wav", "-ac", "2", "-t", decimal(duration), unmuxable}
		if _, err := runFFmpegOutput(ctx, command, "unmuxable candidate"); err != nil {
			t.Fatal(err)
		}
		if _, err := deliverFullDemoAACCandidate(ctx, ffmpeg, input, unmuxable, output, logs, target, false, duration, template, nil); err == nil {
			t.Fatal("unmuxable candidate was published")
		}
		fullDemoTestAssertOutputUnchanged(t, output, previous)
		fullDemoTestNoTemporaryAudioFiles(t, dir)
	})

	t.Run("cancelled context", func(t *testing.T) {
		writePrevious(t)
		cancelled, cancelNow := context.WithCancel(ctx)
		cancelNow()
		if _, err := deliverFullDemoAACCandidate(cancelled, ffmpeg, input, input, output, logs, target, false, duration, template, nil); err == nil {
			t.Fatal("cancelled mux was published")
		}
		fullDemoTestAssertOutputUnchanged(t, output, previous)
		fullDemoTestNoTemporaryAudioFiles(t, dir)
	})
}

// The window between the final accepted measurement and the atomic replace has
// no media work in it, so this boundary is exercised deterministically at the
// publication helper instead of relying on a timing race. A context cancelled
// there must leave the previous output and its own attempt untouched.
func TestFullDemoAudioOnlyPublishRechecksCancellation(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "final.mp4")
	previous := []byte("previous good output")
	if err := os.WriteFile(output, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	attempt, cleanup, err := filecommit.Attempt(output)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := os.WriteFile(attempt, []byte("new verified media"), 0o600); err != nil {
		t.Fatal(err)
	}
	cancelled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if err := publishFullDemoAAC(cancelled, attempt, output); err == nil {
		t.Fatal("cancellation immediately before publication replaced the output")
	}
	fullDemoTestAssertOutputUnchanged(t, output, previous)
	if _, err := os.Stat(attempt); err != nil {
		t.Fatalf("cancelled publication removed its attempt before cleanup: %v", err)
	}
	if err := publishFullDemoAAC(context.Background(), attempt, output); err != nil {
		t.Fatalf("uncancelled publication: %v", err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new verified media" {
		t.Fatalf("published output = %q", got)
	}
}

func TestFullDemoAudioOnlyMasterKeepsOutputOnRejection(t *testing.T) {
	ffmpeg := fullDemoTestFFmpeg(t)
	dir := t.TempDir()
	const frames = 120
	previous := []byte("previous good output")
	output := filepath.Join(dir, "final.mp4")
	if err := os.WriteFile(output, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	input, duration := fullDemoAudioTestProgram(t, ctx, ffmpeg, dir, frames, fullDemoAudioTestSilent, "320x180")
	target := recapplan.DefaultOptions().Audio.Loudness
	if _, err := masterFullDemoProgram(ctx, ffmpeg, input, output, filepath.Join(dir, "logs"), target, false, duration, nil); err == nil {
		t.Fatal("unapproved silent program was published")
	}
	fullDemoTestAssertOutputUnchanged(t, output, previous)
	fullDemoTestNoTemporaryAudioFiles(t, dir)

	cancelled, cancelNow := context.WithCancel(ctx)
	cancelNow()
	if _, err := masterFullDemoProgram(cancelled, ffmpeg, input, output, filepath.Join(dir, "logs"), target, true, duration, nil); err == nil {
		t.Fatal("cancelled master was published")
	}
	fullDemoTestAssertOutputUnchanged(t, output, previous)
	fullDemoTestNoTemporaryAudioFiles(t, dir)
}
