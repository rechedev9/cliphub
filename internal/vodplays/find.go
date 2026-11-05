package vodplays

import "context"

type FindOptions struct {
	FFmpeg string
	Input  string
	// DurationSeconds is the video length, used only to report progress.
	DurationSeconds float64
	// OnProgress receives the share of the video scanned, from 0 to 1.
	OnProgress func(float64)
}

// FindKills scans the whole video and returns the recording player's kills.
func FindKills(ctx context.Context, opts FindOptions) ([]Kill, error) {
	var tracker Tracker
	err := Scan(ctx, ScanOptions{FFmpeg: opts.FFmpeg, Input: opts.Input}, func(s Sample) {
		tracker.Add(s)
		if opts.OnProgress != nil && opts.DurationSeconds > 0 {
			opts.OnProgress(min(s.Seconds/opts.DurationSeconds, 0.99))
		}
	})
	if err != nil {
		return nil, err
	}
	return tracker.Kills(), nil
}
