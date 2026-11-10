package teamvoice

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// download fetches a pinned file to dst, refusing a size or digest other than the pin's.
type download struct {
	URL    string
	SHA256 string
	Bytes  int64
	Dst    string
	// Progress hears the bytes written so far.
	Progress func(done int64)
}

func fetchPinned(ctx context.Context, client *http.Client, d download) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.URL, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", filepath.Base(d.Dst), res.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(d.Dst), "."+filepath.Base(d.Dst)+"-*")
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(tmp.Name())
		}
	}()
	digest := sha256.New()
	counter := &progressWriter{report: d.Progress}
	n, copyErr := io.Copy(io.MultiWriter(tmp, digest, counter), io.LimitReader(res.Body, d.Bytes+1))
	closeErr := tmp.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return err
	}
	if n != d.Bytes {
		return fmt.Errorf("download %s: got %d bytes, want %d", filepath.Base(d.Dst), n, d.Bytes)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != d.SHA256 {
		return fmt.Errorf("download %s: sha256 %s does not match the pinned %s", filepath.Base(d.Dst), got, d.SHA256)
	}
	if err := os.Rename(tmp.Name(), d.Dst); err != nil {
		return err
	}
	keep = true
	return nil
}

type progressWriter struct {
	done   int64
	report func(int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.done += int64(len(p))
	if w.report != nil {
		w.report(w.done)
	}
	return len(p), nil
}

// maxUnzipBytes caps what an archive may expand to.
const maxUnzipBytes = 512 << 20

// unzipTo extracts the regular files of an archive under dir; a path that leaves dir fails it.
func unzipTo(archive, dir string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	var total int64
	for _, f := range r.File {
		name := filepath.FromSlash(f.Name)
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() || filepath.IsAbs(name) || strings.Contains(f.Name, "..") || strings.ContainsAny(f.Name, ":\\") {
			return fmt.Errorf("unzip: refusing %q", f.Name)
		}
		dst := filepath.Join(dir, name)
		if rel, err := filepath.Rel(dir, dst); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("unzip: %q leaves the folder", f.Name)
		}
		total += int64(f.UncompressedSize64)
		if total > maxUnzipBytes {
			return errors.New("unzip: the archive is larger than expected")
		}
		if err := extractOne(f, dst); err != nil {
			return err
		}
	}
	return nil
}

func extractOne(f *zip.File, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o750)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, io.LimitReader(src, int64(f.UncompressedSize64)+1))
	return errors.Join(copyErr, out.Close())
}

// findFile walks dir for the first file with this name, case-insensitively.
func findFile(dir, name string) string {
	found := ""
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), name) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
