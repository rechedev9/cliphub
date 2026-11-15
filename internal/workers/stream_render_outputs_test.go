package workers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/storage"
	"github.com/rechedev9/cliphub/internal/streamclips"
	"github.com/rechedev9/cliphub/internal/tasks"
)

// concatRecordingRunner writes every output and keeps each concat list it reads.
type concatRecordingRunner struct {
	mu    sync.Mutex
	lists []string
}

func (r *concatRecordingRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	if argValue(args, "-f") == "concat" {
		list, err := os.ReadFile(argValue(args, "-i"))
		if err != nil {
			return nil, err
		}
		r.mu.Lock()
		r.lists = append(r.lists, string(list))
		r.mu.Unlock()
	}
	out := args[len(args)-1]
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return nil, err
	}
	return nil, os.WriteFile(out, []byte("rendered-video"), 0o600)
}

func TestStreamRenderWorkerRendersOneVideoPerGroup(t *testing.T) {
	id := uuid.New()
	variant := streamclips.DefaultVariant().Name
	plan := streamclips.DefaultEditPlan()
	plan.Clips = []streamclips.ClipRange{
		{ID: "clip-1", StartSeconds: 0, EndSeconds: 2, Title: "TRIPLE"},
		{ID: "clip-2", StartSeconds: 3, EndSeconds: 5},
		{ID: "clip-3", StartSeconds: 6, EndSeconds: 9, Title: "ACE"},
	}
	plan.Groups = []streamclips.ClipGroup{{ID: "group-1", ClipIDs: []string{"clip-3", "clip-1"}}}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal edit plan: %v", err)
	}
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("storage.NewLocal: %v", err)
	}
	if err := store.Put(streamclips.SourceKey(id), strings.NewReader("source-video")); err != nil {
		t.Fatalf("write stream source: %v", err)
	}
	repo := &completionStreamRepo{job: streamclips.Job{
		ID: id, Status: streamclips.StatusReady, SourcePath: streamclips.SourceKey(id), EditPlan: planJSON,
	}}
	w := NewStreamRenderWorker(repo, store, StreamRenderWorkerConfig{WorkDir: t.TempDir(), FFmpegPath: "ffmpeg"})
	runner := &concatRecordingRunner{}
	w.runner = runner
	task, err := tasks.NewRenderStreamClipTask(id, variant)
	if err != nil {
		t.Fatalf("NewRenderStreamClipTask: %v", err)
	}
	if err := w.HandleRenderStreamClip(context.Background(), task); err != nil {
		t.Fatalf("HandleRenderStreamClip: %v", err)
	}

	key, err := streamclips.RenderStateKey(id, variant)
	if err != nil {
		t.Fatalf("RenderStateKey: %v", err)
	}
	rc, err := store.Open(key)
	if err != nil {
		t.Fatalf("open render state: %v", err)
	}
	defer rc.Close()
	var state streamclips.RenderState
	if err := json.NewDecoder(rc).Decode(&state); err != nil {
		t.Fatalf("decode render state: %v", err)
	}
	ids := make([]string, 0, len(state.Videos))
	for _, video := range state.Videos {
		ids = append(ids, video.ClipID)
	}
	if !slices.Equal(ids, []string{"group-1", "clip-2"}) {
		t.Fatalf("videos = %v, want the group then the lone moment", ids)
	}
	if got := state.Videos[0]; got.Title != "ACE + TRIPLE" || got.DurationSeconds != 5 {
		t.Fatalf("group video = %+v", got)
	}
	if len(runner.lists) != 1 {
		t.Fatalf("concat runs = %d, want 1", len(runner.lists))
	}
	list := runner.lists[0]
	if ace, triple := strings.Index(list, "clip-3.mp4"), strings.Index(list, "clip-1.mp4"); ace < 0 || triple < ace {
		t.Fatalf("concat list does not hold the group in its order:\n%s", list)
	}
}

func TestStreamJoinArgsReencodeOnlyTheAudio(t *testing.T) {
	args := strings.Join(streamJoinArgs("list.txt", []string{"a.mp4", "b.mp4"}, true, "out.mp4"), " ")
	for _, want := range []string{"-i list.txt -i a.mp4 -i b.mp4", "[1:a][2:a]concat=n=2:v=0:a=1[a]", "-map 0:v -map [a] -c:v copy -c:a aac"} {
		if !strings.Contains(args, want) {
			t.Fatalf("join args missing %q: %s", want, args)
		}
	}
	silent := strings.Join(streamJoinArgs("list.txt", []string{"a.mp4", "b.mp4"}, false, "out.mp4"), " ")
	if strings.Contains(silent, "a.mp4") || !strings.Contains(silent, "-map 0:v -c:v copy") {
		t.Fatalf("silent join args = %s", silent)
	}
}
