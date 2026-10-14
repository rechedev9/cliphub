package rooted

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Path struct {
	full string
}

func (p Path) String() string {
	return p.full
}

func Local(path string) (Path, error) {
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return Path{}, fmt.Errorf("rooted: invalid path")
	}
	return Path{full: filepath.Clean(path)}, nil
}

type Dir struct {
	root string
}

func NewDir(root string) (Dir, error) {
	if root == "" || strings.IndexByte(root, 0) >= 0 {
		return Dir{}, fmt.Errorf("rooted: invalid directory")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Dir{}, err
	}
	return Dir{root: filepath.Clean(abs)}, nil
}

func (d Dir) Root() string {
	return d.root
}

func (d Dir) Within(key string) (Path, error) {
	if strings.HasPrefix(key, "/") || strings.HasPrefix(key, `\`) {
		return Path{}, fmt.Errorf("rooted: invalid key %q", key)
	}
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || filepath.IsAbs(clean) {
		return Path{}, fmt.Errorf("rooted: invalid key %q", key)
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return Path{}, fmt.Errorf("rooted: invalid key %q", key)
	}
	abs := filepath.Join(d.root, clean)
	rel, err := filepath.Rel(d.root, abs)
	if err != nil {
		return Path{}, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return Path{}, fmt.Errorf("rooted: key escapes root")
	}
	return Path{full: abs}, nil
}

func (p Path) Open() (*os.File, error) {
	return os.Open(p.full) // #nosec G304
}

func (p Path) Create() (*os.File, error) {
	return os.Create(p.full) // #nosec G304
}

func (p Path) OpenFile(flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(p.full, flag, perm) // #nosec G304
}

func (p Path) ReadFile() ([]byte, error) {
	return os.ReadFile(p.full) // #nosec G304
}

func (p Path) WriteFile(data []byte, perm os.FileMode) error {
	return os.WriteFile(p.full, data, perm) // #nosec G304
}

func (p Path) Stat() (os.FileInfo, error) {
	return os.Stat(p.full) // #nosec G304
}

func Stat(path string) (os.FileInfo, error) {
	p, err := Local(path)
	if err != nil {
		return nil, err
	}
	return p.Stat()
}

func Open(path string) (*os.File, error) {
	p, err := Local(path)
	if err != nil {
		return nil, err
	}
	return p.Open()
}

func Create(path string) (*os.File, error) {
	p, err := Local(path)
	if err != nil {
		return nil, err
	}
	return p.Create()
}

func OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	p, err := Local(path)
	if err != nil {
		return nil, err
	}
	return p.OpenFile(flag, perm)
}

func ReadFile(path string) ([]byte, error) {
	p, err := Local(path)
	if err != nil {
		return nil, err
	}
	return p.ReadFile()
}

func WriteFile(path string, data []byte, perm os.FileMode) error {
	p, err := Local(path)
	if err != nil {
		return err
	}
	return p.WriteFile(data, perm)
}
