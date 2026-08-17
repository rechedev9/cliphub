package mediaassets

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAssetValidate(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	valid := Asset{
		ID:       id,
		SHA256:   strings.Repeat("ab", 32),
		FileName: "clip.mp4",
		Origin:   OriginUpload,
		MediaKey: MediaKey(id),
	}
	cases := []struct {
		name    string
		mutate  func(*Asset)
		wantErr string
	}{
		{name: "ok", mutate: func(*Asset) {}},
		{name: "nil id", mutate: func(a *Asset) { a.ID = uuid.Nil }, wantErr: "id is required"},
		{name: "bad hash", mutate: func(a *Asset) { a.SHA256 = "zz" }, wantErr: "sha256"},
		{name: "path name", mutate: func(a *Asset) { a.FileName = "../x.mp4" }, wantErr: "file name"},
		{name: "parens name", mutate: func(a *Asset) { a.FileName = "clip (1).mp4" }, wantErr: "file name"},
		{name: "bad origin", mutate: func(a *Asset) { a.Origin = "ftp" }, wantErr: "origin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := valid
			tc.mutate(&a)
			err := a.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{in: "clip.mp4", want: "clip.mp4"},
		{in: "clip (1).mp4", want: "clip 1.mp4"},
		{in: "../secret.mp4", want: "secret.mp4"},
		{in: "???", want: "clip.mp4"},
		{in: "", want: "clip.mp4"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := SanitizeFileName(tc.in); got != tc.want {
				t.Fatalf("SanitizeFileName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
