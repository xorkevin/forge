package writefs

import (
	"bytes"
	"io"
	"io/fs"
)

type (
	FSMock struct {
		Files map[string]string
	}

	FSFileMock struct {
		name string
		b    *bytes.Buffer
		f    *FSMock
	}
)

func NewFSMock() *FSMock {
	return &FSMock{
		Files: map[string]string{},
	}
}

func (f *FSMock) OpenFile(name string, flag int, perm fs.FileMode) (io.WriteCloser, error) {
	return NewFSFileMock(name, f), nil
}

func NewFSFileMock(name string, f *FSMock) *FSFileMock {
	return &FSFileMock{
		name: name,
		b:    &bytes.Buffer{},
		f:    f,
	}
}

func (w *FSFileMock) Write(p []byte) (n int, err error) {
	return w.b.Write(p)
}

func (w *FSFileMock) Close() error {
	w.f.Files[w.name] = w.b.String()
	return nil
}
