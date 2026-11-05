package tuiclient

import (
	"context"
	"net/http"
	"testing"
)

func TestVideoDurationsKeyedByPublishedFileStem(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[
			{"segment_id":"seg-001","video":"C:\\jobs\\x\\publish\\short-001-seg-001-s1mple-mirage-4k.mp4","duration_seconds":21.5},
			{"segment_id":"seg-002","video":"publish/short-002-seg-002-s1mple-mirage-2k.mp4","duration_seconds":14},
			{"segment_id":"seg-003","video":"publish/short-003.mp4","duration_seconds":0}]}`))
	})
	got, err := c.VideoDurations(context.Background(), "job", "viral-60-clean")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["short-001-seg-001-s1mple-mirage-4k"] != 21.5 || got["short-002-seg-002-s1mple-mirage-2k"] != 14 {
		t.Errorf("durations = %v", got)
	}
	if _, bySegment := got["seg-001"]; bySegment {
		t.Error("durations must not be keyed by segment id")
	}
}

func TestJobIDsAreEscapedInPaths(t *testing.T) {
	var seen string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{}`))
	})
	if _, err := c.GetMoments(context.Background(), "a/../b"); err != nil {
		t.Fatal(err)
	}
	if want := "/api/jobs/a%2F..%2Fb/moments"; seen != want {
		t.Errorf("path = %q, want %q", seen, want)
	}
}

func TestFilterMomentsSendsTheFilter(t *testing.T) {
	var seen string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	})
	if _, err := c.FilterMoments(context.Background(), MomentsQuery{JobID: "j", Weapon: "awp", Kinds: []string{"clutch", "ace"}, Side: "CT"}); err != nil {
		t.Fatal(err)
	}
	if want := "kind=clutch&kind=ace&side=CT&weapon=awp"; seen != want {
		t.Errorf("query = %q, want %q", seen, want)
	}
}
