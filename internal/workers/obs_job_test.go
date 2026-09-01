package workers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/rechedev9/cliphub/internal/job"
	"github.com/rechedev9/cliphub/internal/obs"
	"github.com/rechedev9/cliphub/internal/tasks"
)

func TestJobFailureIsQueryableByJobIDAndClass(t *testing.T) {
	rec := obs.Default()
	if rec == nil {
		t.Fatal("obs.Default is nil")
	}
	cases := []struct {
		name string
		task string
		err  error
		want string
	}{
		{
			name: "missing_plate",
			task: tasks.TypeRenderStreamClip,
			err:  fmt.Errorf(`composite keydrop banner code "HUASO": keydrop banner style "jcorko" plate is missing`),
			want: obs.ClassMissingPlate,
		},
		{
			name: "capture_flake",
			task: tasks.TypeRecordDemo,
			err:  errors.New("recorder failed: observer target 76561198000000000 drifted from 76561198000000001 during seg-001"),
			want: obs.ClassCaptureFlake,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New()
			repo := newFakeJobRepo(job.Job{ID: id, Status: job.StatusRecording})
			if err := recordTaskFailure(context.Background(), repo, id, tc.task, tc.err); err != nil {
				t.Fatalf("recordTaskFailure: %v", err)
			}
			got := repo.jobs[id]
			if got.Status != job.StatusFailed {
				t.Fatalf("status = %s, want failed", got.Status)
			}
			if got.FailureReason != tc.err.Error() {
				t.Fatalf("FailureReason = %q, want the original human text", got.FailureReason)
			}
			if got.FailureCode != tc.want {
				t.Fatalf("FailureCode = %q, want %q", got.FailureCode, tc.want)
			}
			found, err := rec.SelectErrors(id.String(), tc.want)
			if err != nil {
				t.Fatalf("SelectErrors: %v", err)
			}
			if len(found) != 1 {
				t.Fatalf("SelectErrors(%s, %s) = %#v, want one journal line", id, tc.want, found)
			}
			if found[0].JobID != id.String() || found[0].Class != tc.want || found[0].Task != tc.task {
				t.Fatalf("journal event = %+v", found[0])
			}
			if found[0].Message != tc.err.Error() {
				t.Fatalf("journal message = %q", found[0].Message)
			}
			if obs.Select(found, id.String(), "no-such-class") != nil {
				t.Fatal("Select matched a class that is not on the event")
			}
		})
	}
}
