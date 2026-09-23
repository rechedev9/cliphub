package recording

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// MuxSegmentClips combines each segment take's video.mp4 and audio.wav into a
// consumable MP4 under <output>/segments/<segment-id>.mp4.
func MuxSegmentClips(ctx context.Context, plan RecordingPlan, artifacts []RecordingArtifact, ffmpegPath, ffprobePath string) []RecordingArtifact {
	if ffmpegPath == "" {
		return nil
	}
	pairs := segmentMediaPairs(artifacts)
	if len(pairs) == 0 {
		return nil
	}

	outDir := filepath.Join(plan.OutputDir, "segments")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return []RecordingArtifact{{
			Role:       "segment",
			Path:       outDir,
			ProbeError: fmt.Sprintf("create segment output dir: %v", err),
		}}
	}

	segmentOrder := make(map[string]int, len(plan.Segments))
	for i, s := range plan.Segments {
		segmentOrder[s.ID] = i
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return segmentOrder[pairs[i].segmentID] < segmentOrder[pairs[j].segmentID]
	})

	return muxSegmentPairs(pairs, min(4, runtime.NumCPU()), func(pair segmentMediaPair) RecordingArtifact {
		path := filepath.Join(outDir, pair.segmentID+".mp4")
		artifact := RecordingArtifact{
			SegmentID: pair.segmentID,
			TakeID:    pair.takeID,
			Type:      "video",
			Role:      "segment",
			Path:      path,
		}
		// A clip the IncrementalMuxer already published mid-run was muxed from
		// the same take inputs; re-muxing would rewrite a file an observer may be
		// uploading right now, so keep it and only collect its metadata.
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			artifact.SizeBytes = info.Size()
			if ffprobePath != "" {
				probeArtifact(ctx, ffprobePath, &artifact)
			}
			return artifact
		}
		if err := muxPair(ctx, ffmpegPath, pair.video.Path, pair.audio.Path, path); err != nil {
			artifact.ProbeError = fmt.Sprintf("ffmpeg mux: %v", err)
			return artifact
		}
		if info, err := os.Stat(path); err == nil {
			artifact.SizeBytes = info.Size()
		}
		if ffprobePath != "" {
			probeArtifact(ctx, ffprobePath, &artifact)
		}
		return artifact
	})
}

// Finalization runs after capture has stopped. Muxing copies video packets and
// encodes only audio, so a small pool can finish outstanding clips and probe
// already-published ones together. Preserve input order and serialize pairs
// sharing a segment destination: multiple takes must never write the same
// .part file or replace an incremental clip concurrently.
func muxSegmentPairs(pairs []segmentMediaPair, jobs int, process func(segmentMediaPair) RecordingArtifact) []RecordingArtifact {
	out := make([]RecordingArtifact, len(pairs))
	groups := make([][]int, 0, len(pairs))
	bySegment := make(map[string]int, len(pairs))
	for i, pair := range pairs {
		// Tokens are ASCII; fold case conservatively for Windows/macOS paths.
		key := strings.ToLower(pair.segmentID)
		group, ok := bySegment[key]
		if !ok {
			group = len(groups)
			bySegment[key] = group
			groups = append(groups, nil)
		}
		groups[group] = append(groups[group], i)
	}
	queue := make(chan []int)
	var workers sync.WaitGroup
	for range min(max(1, jobs), len(groups)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for indices := range queue {
				for _, i := range indices {
					out[i] = process(pairs[i])
				}
			}
		}()
	}
	for _, indices := range groups {
		queue <- indices
	}
	close(queue)
	workers.Wait()
	return out
}

type segmentMediaPair struct {
	segmentID string
	takeID    string
	video     RecordingArtifact
	audio     RecordingArtifact
}

func segmentMediaPairs(artifacts []RecordingArtifact) []segmentMediaPair {
	// Group by pointer into the input slice: RecordingArtifact is large, so a
	// value map would copy both video and audio structs on every read-modify-write.
	type partial struct {
		video *RecordingArtifact
		audio *RecordingArtifact
	}
	grouped := map[string]*partial{}
	for i := range artifacts {
		a := &artifacts[i]
		if a.SegmentID == "" || a.TakeID == "" {
			continue
		}
		key := a.SegmentID + "\x00" + a.TakeID
		p := grouped[key]
		if p == nil {
			p = &partial{}
			grouped[key] = p
		}
		switch a.Type {
		case "video":
			if p.video == nil || filepath.Base(a.Path) == "video.mp4" {
				p.video = a
			}
		case "audio":
			if p.audio == nil || filepath.Base(a.Path) == "audio.wav" {
				p.audio = a
			}
		}
	}

	pairs := make([]segmentMediaPair, 0, len(grouped))
	for key, p := range grouped {
		if p.video == nil || p.audio == nil {
			continue
		}
		segmentID, takeID := splitPairKey(key)
		pairs = append(pairs, segmentMediaPair{
			segmentID: segmentID,
			takeID:    takeID,
			video:     *p.video,
			audio:     *p.audio,
		})
	}
	// The only caller re-sorts by plan order, so do not sort here.
	return pairs
}

func splitPairKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == 0 {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
