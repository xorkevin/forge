package writefs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type (
	// FS is a file system that may be read from and written to
	FS interface {
		OpenFile(name string, flag int, perm fs.FileMode) (io.WriteCloser, error)
	}

	// OS implements FS with the os file system
	OS struct {
		Base string
	}
)

func NewOS(base string) *OS {
	return &OS{
		Base: base,
	}
}

func (o *OS) OpenFile(name string, flag int, perm fs.FileMode) (io.WriteCloser, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	path := filepath.Join(o.Base, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("Failed to mkdir: %w", err)
	}
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("Invalid file: %w", err)
	}
	return f, nil
}
